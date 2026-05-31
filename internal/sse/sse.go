// Package sse provides a Server-Sent Events writer with normalized event types
// (plan, question, done, error), heartbeat support, and context-aware lifecycle
// management for the English-Learning Phase1 streaming endpoints.
package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event represents a normalized SSE event name.
type Event string

const (
	// EventPlan is emitted when the agent produces a learning plan.
	EventPlan Event = "plan"
	// EventQuestion is emitted when the agent produces a question.
	EventQuestion Event = "question"
	// EventDone is emitted when a stream completes successfully.
	EventDone Event = "done"
	// EventError is emitted when the stream terminates with an error.
	EventError Event = "error"
)

// ErrStreamingUnsupported is returned when the underlying ResponseWriter
// does not implement http.Flusher.
var ErrStreamingUnsupported = errors.New("sse: streaming unsupported (http.Flusher not implemented)")

// Writer is an SSE writer that serializes payloads as JSON and flushes
// each event immediately. It is safe for concurrent use.
type Writer struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
}

// New constructs a Writer, sets the SSE response headers, and verifies the
// underlying ResponseWriter supports flushing.
func New(w http.ResponseWriter) (*Writer, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, ErrStreamingUnsupported
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	return &Writer{w: w, flusher: flusher}, nil
}

// Send serializes data as JSON and writes an SSE event frame, then flushes.
func (sw *Writer) Send(eventName string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse: marshal payload: %w", err)
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if _, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", eventName, payload); err != nil {
		return err
	}
	sw.flusher.Flush()
	return nil
}

// Plan sends a `plan` event.
func (sw *Writer) Plan(data any) error { return sw.Send(string(EventPlan), data) }

// Question sends a `question` event.
func (sw *Writer) Question(data any) error { return sw.Send(string(EventQuestion), data) }

// Done sends a `done` event.
func (sw *Writer) Done(data any) error { return sw.Send(string(EventDone), data) }

// Error sends an `error` event. If err is nil an empty message is sent.
func (sw *Writer) Error(err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return sw.Send(string(EventError), map[string]string{"error": msg})
}

// Heartbeat writes SSE comment frames (":keepalive\n\n") every interval until
// ctx is canceled. It is intended to be run in a goroutine and returns when
// the context is done.
func (sw *Writer) Heartbeat(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sw.mu.Lock()
			if _, err := fmt.Fprint(sw.w, ":keepalive\n\n"); err != nil {
				sw.mu.Unlock()
				return
			}
			sw.flusher.Flush()
			sw.mu.Unlock()
		}
	}
}
