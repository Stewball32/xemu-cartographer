package containers

// WebSocket relay for the kiosk's keyboard-only VNC port.
//
// Why a relay instead of direct: same reason as the kiosk HTTP proxy — the
// browser_vnc port binds to 127.0.0.1 only when CONTAINERS_BIND_LOOPBACK is
// in effect. The relay enforces PocketBase JWT auth (via ?token=) and pipes
// bytes both ways without inspecting the RFB protocol.

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerVNCRelay)
}

func registerVNCRelay() {
	if Router == nil {
		return
	}
	Router.GET("/api/admin/containers/{name}/vnc", handleVNCRelay)
}

func handleVNCRelay(e *core.RequestEvent) error {
	if !authorizeAdminQueryToken(e) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	name := e.Request.PathValue("name")
	info, ok := Manager.Get(name)
	if !ok {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "container not found"})
	}

	clientConn, err := websocket.Accept(e.Response, e.Request, &websocket.AcceptOptions{
		Subprotocols:       []string{"binary"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		return err
	}
	defer clientConn.CloseNow()

	ctx, cancel := context.WithCancel(e.Request.Context())
	defer cancel()

	upstreamURL := "ws://127.0.0.1:" + strconv.Itoa(info.Ports.BrowserVNC) + "/websockify"
	upstreamConn, _, err := websocket.Dial(ctx, upstreamURL, &websocket.DialOptions{
		Subprotocols: []string{"binary"},
	})
	if err != nil {
		clientConn.Close(websocket.StatusBadGateway, "upstream dial failed")
		return nil
	}
	defer upstreamConn.CloseNow()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer cancel()
		copyMessages(ctx, upstreamConn, clientConn)
	}()
	go func() {
		defer wg.Done()
		defer cancel()
		copyMessages(ctx, clientConn, upstreamConn)
	}()
	wg.Wait()
	return nil
}

func copyMessages(ctx context.Context, dst, src *websocket.Conn) {
	for {
		typ, data, err := src.Read(ctx)
		if err != nil {
			return
		}
		if err := dst.Write(ctx, typ, data); err != nil {
			return
		}
	}
}
