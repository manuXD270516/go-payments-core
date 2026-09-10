// Package usecase orquesta las operaciones del núcleo de pagos.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/manuXD270516/go-payments-core/internal/application"
	"github.com/manuXD270516/go-payments-core/internal/domain/ledger"
	"github.com/manuXD270516/go-payments-core/internal/domain/payment"
	"github.com/manuXD270516/go-payments-core/internal/domain/shared"
)

// ErrDeclined indica que el procesador rechazó la autorización.
var ErrDeclined = errors.New("authorization declined")

// Payments agrupa los casos de uso del ciclo de vida de un pago.
type Payments struct {
	Intents   application.IntentRepository
	Ledger    application.LedgerRepository
	Processor application.Processor
	Events    application.EventPublisher
	IDs       application.IDGenerator
	Clock     application.Clock
}

// CreateIntentInput son los datos para crear un intento de pago.
type CreateIntentInput struct {
	MerchantID string
	Amount     int64
	Currency   string
	MethodType string
	Token      string
	Metadata   map[string]string
}

// CreateIntent registra un nuevo intento en estado inicial.
func (p *Payments) CreateIntent(ctx context.Context, in CreateIntentInput) (*payment.Intent, error) {
	amount, err := shared.NewMoney(in.Amount, in.Currency)
	if err != nil {
		return nil, fmt.Errorf("importe invalido: %w", err)
	}
	intent := payment.New(
		p.IDs.NewID("pi"), in.MerchantID, amount,
		payment.Method{Type: in.MethodType, Token: in.Token},
		in.Metadata, p.Clock.Now(),
	)
	if err := p.Intents.Save(ctx, intent); err != nil {
		return nil, fmt.Errorf("guardando intent: %w", err)
	}
	p.emit(ctx, "payment_intent.created", intent)
	return intent, nil
}

// Authorize solicita la autorización al procesador y actualiza el estado.
func (p *Payments) Authorize(ctx context.Context, merchantID, intentID string) (*payment.Intent, error) {
	intent, err := p.load(ctx, merchantID, intentID)
	if err != nil {
		return nil, err
	}
	version := intent.Version

	res, err := p.Processor.Authorize(ctx, application.AuthorizationRequest{
		IntentID: intent.ID, MerchantID: intent.MerchantID,
		Amount: intent.Amount.Amount, Currency: intent.Amount.Currency,
		MethodType: intent.Method.Type, Token: intent.Method.Token,
	})
	if err != nil {
		return nil, fmt.Errorf("procesador: %w", err)
	}
	if !res.Approved {
		return nil, fmt.Errorf("%w: %s", ErrDeclined, res.Code)
	}
	if err := intent.Authorize(res.ProviderRef, p.Clock.Now()); err != nil {
		return nil, err
	}
	if err := p.Intents.Update(ctx, intent, version); err != nil {
		return nil, fmt.Errorf("actualizando intent: %w", err)
	}
	p.emit(ctx, "payment_intent.authorized", intent)
	return intent, nil
}

// Capture captura un importe y registra los asientos contables.
// Si el ledger no balancea, la operación se aborta sin persistir el cambio.
func (p *Payments) Capture(ctx context.Context, merchantID, intentID string, amount int64) (*payment.Intent, error) {
	intent, err := p.load(ctx, merchantID, intentID)
	if err != nil {
		return nil, err
	}
	version := intent.Version
	monto, err := p.money(amount, intent)
	if err != nil {
		return nil, err
	}
	if err := intent.Capture(monto, p.Clock.Now()); err != nil {
		return nil, err
	}
	tx := ledger.NewCapture(p.IDs.NewID("tx"), intent.ID, intent.MerchantID, monto, p.Clock.Now())
	if err := tx.Balance(); err != nil {
		return nil, fmt.Errorf("asientos invalidos: %w", err)
	}
	if err := p.Intents.Update(ctx, intent, version); err != nil {
		return nil, fmt.Errorf("actualizando intent: %w", err)
	}
	if err := p.Ledger.Record(ctx, tx); err != nil {
		return nil, fmt.Errorf("registrando asientos: %w", err)
	}
	p.emit(ctx, "payment_intent.captured", intent)
	return intent, nil
}

// Refund reembolsa un importe capturado y registra los asientos inversos.
func (p *Payments) Refund(ctx context.Context, merchantID, intentID string, amount int64) (*payment.Intent, error) {
	intent, err := p.load(ctx, merchantID, intentID)
	if err != nil {
		return nil, err
	}
	version := intent.Version
	monto, err := p.money(amount, intent)
	if err != nil {
		return nil, err
	}
	if err := intent.Refund(monto, p.Clock.Now()); err != nil {
		return nil, err
	}
	tx := ledger.NewRefund(p.IDs.NewID("tx"), intent.ID, intent.MerchantID, monto, p.Clock.Now())
	if err := tx.Balance(); err != nil {
		return nil, fmt.Errorf("asientos invalidos: %w", err)
	}
	if err := p.Intents.Update(ctx, intent, version); err != nil {
		return nil, fmt.Errorf("actualizando intent: %w", err)
	}
	if err := p.Ledger.Record(ctx, tx); err != nil {
		return nil, fmt.Errorf("registrando asientos: %w", err)
	}
	p.emit(ctx, "payment_intent.refunded", intent)
	return intent, nil
}

// Void anula una autorización no capturada.
func (p *Payments) Void(ctx context.Context, merchantID, intentID string) (*payment.Intent, error) {
	intent, err := p.load(ctx, merchantID, intentID)
	if err != nil {
		return nil, err
	}
	version := intent.Version
	if err := intent.Void(p.Clock.Now()); err != nil {
		return nil, err
	}
	if err := p.Intents.Update(ctx, intent, version); err != nil {
		return nil, fmt.Errorf("actualizando intent: %w", err)
	}
	p.emit(ctx, "payment_intent.voided", intent)
	return intent, nil
}

// Get devuelve un intento respetando el aislamiento por comercio.
func (p *Payments) Get(ctx context.Context, merchantID, intentID string) (*payment.Intent, error) {
	return p.load(ctx, merchantID, intentID)
}

// LedgerEntries devuelve los asientos contables de un intento.
func (p *Payments) LedgerEntries(ctx context.Context, merchantID, intentID string) ([]ledger.Entry, error) {
	if _, err := p.load(ctx, merchantID, intentID); err != nil {
		return nil, err
	}
	return p.Ledger.EntriesByIntent(ctx, merchantID, intentID)
}

func (p *Payments) load(ctx context.Context, merchantID, intentID string) (*payment.Intent, error) {
	intent, err := p.Intents.FindByID(ctx, merchantID, intentID)
	if err != nil {
		return nil, err
	}
	if !intent.BelongsTo(merchantID) {
		return nil, payment.ErrForbidden
	}
	return intent, nil
}

func (p *Payments) money(amount int64, i *payment.Intent) (shared.Money, error) {
	if amount == 0 {
		return i.Amount, nil // por defecto, el total del intento
	}
	return shared.NewMoney(amount, i.Amount.Currency)
}

func (p *Payments) emit(ctx context.Context, tipo string, i *payment.Intent) {
	if p.Events == nil {
		return
	}
	_ = p.Events.Publish(ctx, application.Event{
		ID: p.IDs.NewID("evt"), Type: tipo, MerchantID: i.MerchantID, IntentID: i.ID,
		Payload: map[string]any{"status": string(i.Status), "amount": i.Amount.Amount, "currency": i.Amount.Currency},
	})
}
