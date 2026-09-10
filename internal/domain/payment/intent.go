// Package payment modela el ciclo de vida de un intento de pago.
package payment

import (
	"errors"
	"fmt"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/domain/shared"
)

// Errores de dominio del agregado PaymentIntent.
var (
	ErrInvalidTransition    = errors.New("invalid state transition")
	ErrRefundExceedsCapture = errors.New("refund exceeds captured amount")
	ErrCaptureExceedsAuth   = errors.New("capture exceeds authorized amount")
	ErrNotFound             = errors.New("payment intent not found")
	ErrForbidden            = errors.New("payment intent belongs to another merchant")
)

// Status es el estado de un intento de pago.
type Status string

const (
	StatusCreated    Status = "created"
	StatusAuthorized Status = "authorized"
	StatusCaptured   Status = "captured"
	StatusRefunded   Status = "refunded"
	StatusVoided     Status = "voided"
	StatusFailed     Status = "failed"
)

// transitions declara la máquina de estados como dato, no como condicionales
// dispersos: cualquier transición no listada es inválida por definición.
var transitions = map[Status][]Status{
	StatusCreated:    {StatusAuthorized, StatusFailed},
	StatusAuthorized: {StatusCaptured, StatusVoided, StatusFailed},
	StatusCaptured:   {StatusRefunded},
	StatusRefunded:   {StatusRefunded}, // reembolsos parciales sucesivos
	StatusVoided:     {},
	StatusFailed:     {},
}

// CanTransition indica si el paso de un estado a otro está permitido.
func CanTransition(from, to Status) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Method identifica cómo se cobra: con tarjeta (token) o sin tarjeta.
type Method struct {
	Type  string `json:"type"`            // card | bank_transfer | cash | qr
	Token string `json:"token,omitempty"` // referencia opaca; nunca un PAN
}

// Intent es el agregado raíz: un intento de cobro de un comercio.
type Intent struct {
	ID          string            `json:"id"`
	MerchantID  string            `json:"merchant_id"`
	Amount      shared.Money      `json:"amount"`
	Captured    shared.Money      `json:"captured"`
	Refunded    shared.Money      `json:"refunded"`
	Status      Status            `json:"status"`
	Method      Method            `json:"method"`
	ProviderRef string            `json:"provider_ref,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// New crea un intento en estado inicial.
func New(id, merchantID string, amount shared.Money, method Method, metadata map[string]string, now time.Time) *Intent {
	zero := shared.Money{Amount: 0, Currency: amount.Currency}
	return &Intent{
		ID: id, MerchantID: merchantID, Amount: amount,
		Captured: zero, Refunded: zero, Status: StatusCreated,
		Method: method, Metadata: metadata, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
}

func (i *Intent) transition(to Status, now time.Time) error {
	if !CanTransition(i.Status, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, i.Status, to)
	}
	i.Status = to
	i.Version++
	i.UpdatedAt = now
	return nil
}

// Authorize marca el intento como autorizado por el procesador.
func (i *Intent) Authorize(providerRef string, now time.Time) error {
	if err := i.transition(StatusAuthorized, now); err != nil {
		return err
	}
	i.ProviderRef = providerRef
	return nil
}

// Capture captura un importe, total o parcial, sin exceder lo autorizado.
func (i *Intent) Capture(amount shared.Money, now time.Time) error {
	if i.Status != StatusAuthorized {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, i.Status, StatusCaptured)
	}
	next, err := i.Captured.Add(amount)
	if err != nil {
		return err
	}
	if more, err := next.GreaterThan(i.Amount); err != nil {
		return err
	} else if more {
		return fmt.Errorf("%w: %s > %s", ErrCaptureExceedsAuth, next, i.Amount)
	}
	if err := i.transition(StatusCaptured, now); err != nil {
		return err
	}
	i.Captured = next
	return nil
}

// Refund reembolsa un importe sin exceder lo efectivamente capturado.
func (i *Intent) Refund(amount shared.Money, now time.Time) error {
	if i.Status != StatusCaptured && i.Status != StatusRefunded {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, i.Status, StatusRefunded)
	}
	next, err := i.Refunded.Add(amount)
	if err != nil {
		return err
	}
	if more, err := next.GreaterThan(i.Captured); err != nil {
		return err
	} else if more {
		return fmt.Errorf("%w: %s > %s", ErrRefundExceedsCapture, next, i.Captured)
	}
	if err := i.transition(StatusRefunded, now); err != nil {
		return err
	}
	i.Refunded = next
	return nil
}

// Void anula una autorización que aún no fue capturada.
func (i *Intent) Void(now time.Time) error { return i.transition(StatusVoided, now) }

// BelongsTo verifica el aislamiento multitenant.
func (i *Intent) BelongsTo(merchantID string) bool { return i.MerchantID == merchantID }
