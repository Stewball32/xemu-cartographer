package sinks

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSink appends one envelope per line (NDJSON) to a regular file.
// The handle is opened lazily on the first Write so a configured-but-
// never-fed class doesn't litter the filesystem with empty files.
// Writes append a single '\n' after the envelope bytes; envelopes
// must not themselves contain a literal newline (scraper.MakeEnvelope
// uses encoding/json which never emits one).
//
// Concurrency: the runner loop is the only writer in normal operation,
// but applyPolicies may call Close on a different goroutine when a
// policy reload changes the spec — mu serialises both.
type FileSink struct {
	path string

	mu sync.Mutex
	f  *os.File
}

// NewFileSink returns a Sink that appends to path. The file and any
// missing parent directories are created on first Write.
func NewFileSink(path string) *FileSink {
	return &FileSink{path: path}
}

// Path returns the destination path the sink was constructed with.
// Used by sinkManager to detect spec changes without a separate
// shadow map.
func (s *FileSink) Path() string { return s.path }

func (s *FileSink) Write(env []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return fmt.Errorf("sink: mkdir %s: %w", filepath.Dir(s.path), err)
		}
		f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("sink: open %s: %w", s.path, err)
		}
		s.f = f
	}
	if _, err := s.f.Write(env); err != nil {
		return fmt.Errorf("sink: write %s: %w", s.path, err)
	}
	if _, err := s.f.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("sink: write newline %s: %w", s.path, err)
	}
	return nil
}

// Close flushes (via os.File.Close) and releases the handle. Subsequent
// Writes reopen the file in O_APPEND mode, so Close mid-runner is safe.
// Idempotent — repeated calls return nil.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}
