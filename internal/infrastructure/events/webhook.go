// Package events publica eventos de dominio hacia consumidores externos.
package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/application"
)

// WebhookPublisher entrega eventos por HTTP con reintentos.
type WebhookPublisher struct {
	Endpoint    string
	Client      *http.Client
	MaxAttempts int
	Log         *slog.Logger
	DeadLetter  func(application.Event)
}

// NewWebhookPublisher construye un publicador con timeout obligatorio.
func NewWebhookPublisher(endpoint string, maxAttempts int, log *slog.Logger) *WebhookPublisher {
	return &WebhookPublisher{
		Endpoint:    endpoint,
		Client:      &http.Client{Timeout: 5 * time.Second},
		MaxAttempts: maxAttempts,
		Log:         log,
	}
}

// Publish entrega el evento reintentando con backoff exponencial y jitter.
//
// El jitter evita que muchos reintentos coincidan y golpeen al destino en el
// mismo instante. Agotados los intentos, el evento va a la cola muerta.
func (p *WebhookPublisher) Publish(ctx context.Context, e application.Event) error {
	if p.Endpoint == "" {
		return nil // sin destino configurado, publicar es una operación nula
	}
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("serializando evento: %w", err)
	}
	var ultimo error
	for intento := 1; intento <= p.MaxAttempts; intento++ {
		if err := p.enviar(ctx, body); err == nil {
			return nil
		} else {
			ultimo = err
		}
		if intento < p.MaxAttempts {
			select {
			case <-time.After(backoff(intento)):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	p.Log.Warn("evento a cola muerta", "event_id", e.ID, "type", e.Type)
	if p.DeadLetter != nil {
		p.DeadLetter(e)
	}
	return ultimo
}

func (p *WebhookPublisher) enviar(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("respuesta %d del destino", res.StatusCode)
	}
	return nil
}

func backoff(intento int) time.Duration {
	base := time.Duration(math.Pow(2, float64(intento-1))) * 200 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(base/2 + 1)))
	return base + jitter
}
