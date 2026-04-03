package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/rafaelsoares/alfredo/internal/webhook"
)

func TestEmitter_sendsEnvelopeToServer(t *testing.T) {
	var mu sync.Mutex
	var received []byte
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = body
		gotPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := webhook.New(srv.URL, "petcare", zap.NewNop())
	e.Emit(context.Background(), "bath.recorded", map[string]string{"bath_id": "b1"})

	// give goroutine time to fire
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("no request received by server")
	}
	var env map[string]any
	if err := json.Unmarshal(received, &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env["event"] != "bath.recorded" {
		t.Errorf("event = %v, want bath.recorded", env["event"])
	}
	if env["domain"] != "petcare" {
		t.Errorf("domain = %v, want petcare", env["domain"])
	}
	if env["occurred_at"] == nil {
		t.Error("occurred_at missing")
	}
	if env["payload"] == nil {
		t.Error("payload missing")
	}
	if gotPath != "/events" {
		t.Errorf("path = %v, want /events", gotPath)
	}
}

func TestEmitter_noopWhenURLEmpty(t *testing.T) {
	e := webhook.New("", "petcare", zap.NewNop())
	// Must complete immediately (no goroutine spawned, no blocking)
	done := make(chan struct{})
	go func() {
		e.Emit(context.Background(), "bath.recorded", nil)
		close(done)
	}()
	select {
	case <-done:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Error("Emit with empty URL should return immediately")
	}
}

func TestEmitter_logsN8nResponseBodyOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Invalid node parameter"}`))
	}))
	defer srv.Close()

	core, logs := observer.New(zapcore.WarnLevel)
	log := zap.New(core)

	e := webhook.New(srv.URL, "petcare", log)
	e.Emit(context.Background(), "bath.recorded", map[string]string{"bath_id": "b1"})

	time.Sleep(50 * time.Millisecond)

	if logs.Len() == 0 {
		t.Fatal("expected at least one warn log")
	}
	entry := logs.All()[0]
	var found bool
	for _, f := range entry.Context {
		if f.Key == "n8n_response" && f.String == `{"message":"Invalid node parameter"}` {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("n8n_response field missing or wrong; got fields: %+v", entry.Context)
	}
}

func TestEmitter_debugLogsOutgoingPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	core, logs := observer.New(zapcore.DebugLevel)
	log := zap.New(core)

	e := webhook.New(srv.URL, "petcare", log)
	e.Emit(context.Background(), "bath.recorded", map[string]string{"bath_id": "b1"})

	time.Sleep(50 * time.Millisecond)

	var found bool
	for _, entry := range logs.All() {
		if entry.Message == "webhook: emitting" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'webhook: emitting' debug log not found")
	}
}
