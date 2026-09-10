// Package application define los puertos y casos de uso del núcleo de pagos.
//
// Los puertos son interfaces declaradas del lado del consumidor: el dominio y
// los casos de uso no conocen Postgres, HTTP ni ningún proveedor concreto.
package application

import (
	"context"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/domain/ledger"
	"github.com/manuXD270516/go-payments-core/internal/domain/payment"
)

// IntentRepository persiste intentos de pago con control de concurrencia.
type IntentRepository interface {
	Save(ctx context.Context, i *payment.Intent) error
	// Update aplica un cambio verificando la versión esperada (bloqueo
	// optimista); si otra operación modificó el intent, devuelve ErrConflict.
	Update(ctx context.Context, i *payment.Intent, expectedVersion int) error
	FindByID(ctx context.Context, merchantID, id string) (*payment.Intent, error)
}

// LedgerRepository persiste transacciones contables balanceadas.
type LedgerRepository interface {
	Record(ctx context.Context, tx ledger.Transaction) error
	EntriesByIntent(ctx context.Context, merchantID, intentID string) ([]ledger.Entry, error)
}

// AuthorizationRequest es la solicitud enviada al procesador.
type AuthorizationRequest struct {
	IntentID   string
	MerchantID string
	Amount     int64
	Currency   string
	MethodType string
	Token      string
}

// AuthorizationResult es la respuesta normalizada del procesador.
type AuthorizationResult struct {
	Approved    bool
	ProviderRef string
	Code        string
}

// Processor abstrae al procesador de pagos externo.
type Processor interface {
	Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResult, error)
}

// Event es un evento de dominio publicado hacia los consumidores (webhooks).
type Event struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	MerchantID string         `json:"merchant_id"`
	IntentID   string         `json:"intent_id"`
	Payload    map[string]any `json:"payload"`
}

// EventPublisher publica eventos de dominio.
type EventPublisher interface {
	Publish(ctx context.Context, e Event) error
}

// IDGenerator genera identificadores opacos.
type IDGenerator interface{ NewID(prefix string) string }

// Clock permite controlar el tiempo en las pruebas.
type Clock interface{ Now() time.Time }
