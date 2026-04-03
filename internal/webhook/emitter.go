package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventEmitter is the interface use cases depend on to emit domain events.
type EventEmitter interface {
	Emit(ctx context.Context, event string, payload any)
}

type envelope struct {
	Event      string `json:"event"`
	OccurredAt string `json:"occurred_at"`
	Domain     string `json:"domain"`
	Payload    any    `json:"payload"`
}

// Emitter sends fire-and-forget webhook events to n8n.
// Emit is a no-op if baseURL is empty.
// At most maxConcurrent goroutines run simultaneously; excess events are dropped.
// Call Wait() during shutdown to drain in-flight goroutines.
type Emitter struct {
	baseURL string
	domain  string
	client  *http.Client
	logger  *zap.Logger
	sem     chan struct{} // bounds concurrent in-flight goroutines
	wg      sync.WaitGroup
}

const maxConcurrentWebhooks = 20

// New returns an Emitter pointed at baseURL. Pass "" to disable emission.
func New(baseURL, domain string, logger *zap.Logger) *Emitter {
	return &Emitter{
		baseURL: baseURL,
		domain:  domain,
		client:  &http.Client{Timeout: 5 * time.Second},
		logger:  logger,
		sem:     make(chan struct{}, maxConcurrentWebhooks),
	}
}

// Wait blocks until all in-flight webhook goroutines have completed.
// Call this during graceful shutdown after the HTTP server has stopped accepting requests.
func (e *Emitter) Wait() {
	e.wg.Wait()
}

// Emit fires a domain event to n8n as a fire-and-forget POST request.
// The goroutine uses its own 5-second timeout context independent of the caller's context,
// preserving fire-and-forget semantics while still supporting cancellation.
// Emit is a no-op if the Emitter was created with an empty baseURL.
// If maxConcurrentWebhooks goroutines are already running, the event is dropped and logged.
func (e *Emitter) Emit(_ context.Context, event string, payload any) {
	if e.baseURL == "" {
		return
	}
	env := envelope{
		Event:      event,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		Domain:     e.domain,
		Payload:    payload,
	}
	body, err := json.Marshal(env)
	if err != nil {
		e.logger.Warn("webhook: marshal failed", zap.String("event", event), zap.Error(err))
		return
	}
	e.logger.Debug("webhook: emitting", zap.String("event", event), zap.Any("payload", env))

	// Acquire semaphore slot; drop event if all slots are busy.
	select {
	case e.sem <- struct{}{}:
	default:
		e.logger.Warn("webhook: semaphore full, dropping event", zap.String("event", event))
		return
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() { <-e.sem }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/events", bytes.NewReader(body))
		if err != nil {
			e.logger.Warn("webhook: build request failed", zap.String("event", event), zap.Error(err))
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := e.client.Do(req)
		if err != nil {
			e.logger.Warn("webhook: emit failed", zap.String("event", event), zap.Error(err))
			return
		}
		defer resp.Body.Close() //nolint:errcheck
		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
			e.logger.Warn("webhook: n8n returned error",
				zap.String("event", event),
				zap.Int("status", resp.StatusCode),
				zap.String("n8n_response", string(respBody)),
			)
		}
	}()
}
