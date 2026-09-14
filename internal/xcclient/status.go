package xcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Status is the upstream connection's health (§8.1 / §12): served by
// GET /api/admin/scraper/upstream and the studio banner.
type Status struct {
	// Connected is whether the stream socket is open right now.
	Connected bool `json:"connected"`
	// Since is when Connected last changed: "connected since" when up,
	// "disconnected since" when down (the banner's timestamp).
	Since time.Time `json:"since"`
	// Reconnects counts successful dials after the first.
	Reconnects int `json:"reconnects"`
	// LastFrameAt is when the worker last handled an upstream frame.
	LastFrameAt time.Time `json:"last_frame_at"`
	// SeqGaps counts skipped seq numbers across all (instance, class) streams.
	SeqGaps uint64 `json:"seq_gaps"`
	// Shed counts frames the read pump dropped under backpressure (D-11).
	Shed uint64 `json:"shed"`
	// Stale is set once a disconnect outlasted StaleAfter and the mirror
	// was cleared (D-12); cleared by the next hello.
	Stale bool `json:"stale"`
	// Attempts counts failed dials since the stream was last up (0 while
	// connected): the banner's "reconnecting (n attempts)".
	Attempts int `json:"attempts"`
	// LastError is why the last dial failed or the last connection ended
	// (token redacted); "" once a connection is up again.
	LastError string `json:"last_error"`
	// AuthRejected is set when the daemon refused the feed token: the
	// socket stays up but the daemon admitted it as anonymous (hello lists
	// no instances, every host:* join answers forbidden — wire.md A.1), so
	// Connected alone looks healthy while nothing flows. Sticky across
	// reconnects; cleared by the first frame that proves a join succeeded.
	AuthRejected bool `json:"auth_rejected"`
}

// Status returns a snapshot of the connection state and counters.
func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{
		Connected:    c.connected,
		Since:        c.since,
		Reconnects:   c.reconnects,
		LastFrameAt:  c.lastFrameAt,
		SeqGaps:      c.seqGaps.Load(),
		Shed:         c.shed.Load(),
		Stale:        c.stale,
		Attempts:     c.attempts,
		LastError:    c.lastErr,
		AuthRejected: c.authRejected,
	}
}

// noteConnectError records why a dial failed or a connection ended (the
// reconnect loop's error, already redacted) and counts the attempt.
func (c *Client) noteConnectError(err error) {
	c.mu.Lock()
	c.attempts++
	if err != nil {
		c.lastErr = err.Error()
	}
	c.mu.Unlock()
}

// noteAuthRejected latches AuthRejected on a forbidden error frame and
// reports whether this is the edge (false→true), so the caller logs once
// per rejection episode rather than once per join or per reconnect.
func (c *Client) noteAuthRejected(room string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr = "feed token rejected by the daemon (forbidden on " + room + ")"
	if c.authRejected {
		return false
	}
	c.authRejected = true
	return true
}

// noteAuthAccepted clears AuthRejected once a frame proves the daemon
// serves this connection (a join replay or live data on a host:* room);
// reports whether a rejection episode just ended.
func (c *Client) noteAuthAccepted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.authRejected {
		return false
	}
	c.authRejected = false
	c.lastErr = ""
	return true
}

func (c *Client) touch() {
	c.mu.Lock()
	c.lastFrameAt = time.Now()
	c.mu.Unlock()
}

func (c *Client) setStale(v bool) {
	c.mu.Lock()
	c.stale = v
	c.mu.Unlock()
}

// noteSeqGap counts a gap and logs at most one line per minute (§12).
func (c *Client) noteSeqGap(instance, class string, gap uint64) {
	c.seqGaps.Add(gap)
	c.mu.Lock()
	quiet := time.Since(c.gapLogAt) < time.Minute
	if !quiet {
		c.gapLogAt = time.Now()
	}
	c.mu.Unlock()
	if !quiet {
		c.logf("xcclient: %s/%s: seq gap of %d (total %d)", instance, class, gap, c.seqGaps.Load())
	}
}

// instanceRow is the slice of GET /api/instances the client needs
// (daemon.InstanceRow embeds runner.Info; started_at is what §8.1 reads).
type instanceRow struct {
	Name      string    `json:"name"`
	StartedAt time.Time `json:"started_at"`
}

// fetchInstances reads GET /api/instances with the feed token.
func (c *Client) fetchInstances(ctx context.Context) (map[string]time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, c.fetchTimeout)
	defer cancel()
	u := *c.base
	u.Path = c.base.Path + "/api/instances"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /api/instances: status %d %s", resp.StatusCode, trimPayload(body))
	}
	var rows []instanceRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("GET /api/instances: %w", err)
	}
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		if r.Name != "" {
			out[r.Name] = r.StartedAt
		}
	}
	return out, nil
}

// refreshPlaceholders replaces placeholder started_at values with the
// daemon's (GET /api/instances). Unless force is set, the read is attempted
// at most once per fetchRetry; a failure keeps the placeholders and logs.
func (c *Client) refreshPlaceholders(ctx context.Context, force bool) {
	names := c.mirror.Placeholders()
	if len(names) == 0 {
		return
	}
	c.mu.Lock()
	due := force || time.Since(c.fetchAt) >= c.fetchRetry
	if due {
		c.fetchAt = time.Now()
	}
	c.mu.Unlock()
	if !due {
		return
	}
	rows, err := c.fetchInstances(ctx)
	if err != nil {
		c.logf("xcclient: started_at for %v unavailable (%v); envelope ts stands in", names, err)
		return
	}
	for _, name := range names {
		if at, ok := rows[name]; ok && !at.IsZero() {
			c.mirror.SetInstance(name, at, false)
		}
	}
}
