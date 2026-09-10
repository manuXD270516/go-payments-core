// Package shared contiene value objects y errores compartidos por el dominio.
package shared

import (
	"errors"
	"fmt"
)

// Errores de dominio para operaciones monetarias.
var (
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrNegativeAmount   = errors.New("negative amount")
	ErrInvalidCurrency  = errors.New("invalid currency")
)

// Money representa un importe en la unidad mínima de la moneda (centavos).
//
// Se usa un entero y no un float de forma deliberada: el punto flotante no
// puede representar exactamente valores como 0.10 y el error se acumula, algo
// inaceptable en contabilidad.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// NewMoney construye un importe validando moneda ISO 4217 y signo.
func NewMoney(amount int64, currency string) (Money, error) {
	if len(currency) != 3 {
		return Money{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, currency)
	}
	if amount < 0 {
		return Money{}, fmt.Errorf("%w: %d", ErrNegativeAmount, amount)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

// Add suma dos importes de la misma moneda.
func (m Money) Add(o Money) (Money, error) {
	if m.Currency != o.Currency {
		return Money{}, fmt.Errorf("%w: %s + %s", ErrCurrencyMismatch, m.Currency, o.Currency)
	}
	return Money{Amount: m.Amount + o.Amount, Currency: m.Currency}, nil
}

// Sub resta dos importes de la misma moneda; no admite resultado negativo.
func (m Money) Sub(o Money) (Money, error) {
	if m.Currency != o.Currency {
		return Money{}, fmt.Errorf("%w: %s - %s", ErrCurrencyMismatch, m.Currency, o.Currency)
	}
	if m.Amount < o.Amount {
		return Money{}, fmt.Errorf("%w: %d - %d", ErrNegativeAmount, m.Amount, o.Amount)
	}
	return Money{Amount: m.Amount - o.Amount, Currency: m.Currency}, nil
}

// GreaterThan indica si el importe es mayor que otro de la misma moneda.
func (m Money) GreaterThan(o Money) (bool, error) {
	if m.Currency != o.Currency {
		return false, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency, o.Currency)
	}
	return m.Amount > o.Amount, nil
}

// IsZero indica si el importe es cero.
func (m Money) IsZero() bool { return m.Amount == 0 }

// String representa el importe de forma legible, sin perder precisión.
func (m Money) String() string {
	return fmt.Sprintf("%d.%02d %s", m.Amount/100, m.Amount%100, m.Currency)
}
