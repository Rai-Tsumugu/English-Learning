package sse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewSetsHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	w, err := New(rec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil Writer")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Connection"); got != "keep-alive" {
		t.Errorf("Connection = %q", got)
	}
}

func TestSendFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	w, err := New(rec)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := w.Send("plan", map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	body := rec.Body.String()
	want := "event: plan\ndata: {\"hello\":\"world\"}\n\n"
	if body != want {
		t.Errorf("body=%q want=%q", body, want)
	}
}

func TestConvenienceMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	w, _ := New(rec)
	if err := w.Plan(map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := w.Question(map[string]int{"b": 2}); err != nil {
		t.Fatal(err)
	}
	if err := w.Done(map[string]int{"c": 3}); err != nil {
		t.Fatal(err)
	}
	if err := w.Error(errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	for _, sub := range []string{
		"event: plan\ndata: {\"a\":1}\n\n",
		"event: question\ndata: {\"b\":2}\n\n",
		"event: done\ndata: {\"c\":3}\n\n",
		"event: error\ndata: {\"error\":\"boom\"}\n\n",
	} {
		if !strings.Contains(body, sub) {
			t.Errorf("missing %q in body=%q", sub, body)
		}
	}
}

func TestErrorNil(t *testing.T) {
	rec := httptest.NewRecorder()
	w, _ := New(rec)
	if err := w.Error(nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), "event: error\ndata: {\"error\":\"\"}\n\n") {
		t.Errorf("unexpected body=%q", rec.Body.String())
	}
}

func TestHeartbeatStopsOnContextCancel(t *testing.T) {
	rec := httptest.NewRecorder()
	w, _ := New(rec)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Heartbeat(ctx, 5*time.Millisecond)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not stop after cancel")
	}
	if !strings.Contains(rec.Body.String(), ":keepalive\n\n") {
		t.Errorf("expected keepalive frame, body=%q", rec.Body.String())
	}
}

func TestHeartbeatZeroInterval(t *testing.T) {
	rec := httptest.NewRecorder()
	w, _ := New(rec)
	done := make(chan struct{})
	go func() {
		w.Heartbeat(context.Background(), 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Heartbeat with zero interval should return immediately")
	}
}

// nfWriter is a ResponseWriter without Flusher to verify the error path.
type nfWriter struct{ h http.Header }

func (n *nfWriter) Header() http.Header        { return n.h }
func (n *nfWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nfWriter) WriteHeader(int)             {}

func TestNewRequiresFlusher(t *testing.T) {
	nw := &nfWriter{h: http.Header{}}
	if _, err := New(nw); !errors.Is(err, ErrStreamingUnsupported) {
		t.Errorf("err = %v, want ErrStreamingUnsupported", err)
	}
}
