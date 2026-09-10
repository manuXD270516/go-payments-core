package shared

import (
	"errors"
	"testing"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		currency string
		wantErr  error
	}{
		{"importe valido", 1050, "BOB", nil},
		{"cero es valido", 0, "USD", nil},
		{"importe negativo", -1, "USD", ErrNegativeAmount},
		{"moneda invalida", 100, "US", ErrInvalidCurrency},
		{"moneda vacia", 100, "", ErrInvalidCurrency},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewMoney(tc.amount, tc.currency)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, se esperaba %v", err, tc.wantErr)
			}
			if err == nil && got.Amount != tc.amount {
				t.Errorf("amount = %d, se esperaba %d", got.Amount, tc.amount)
			}
		})
	}
}

func TestMoneyAddSub(t *testing.T) {
	a, _ := NewMoney(1000, "BOB")
	b, _ := NewMoney(250, "BOB")
	other, _ := NewMoney(250, "USD")

	if sum, err := a.Add(b); err != nil || sum.Amount != 1250 {
		t.Errorf("Add = %v, %v; se esperaba 1250", sum.Amount, err)
	}
	if diff, err := a.Sub(b); err != nil || diff.Amount != 750 {
		t.Errorf("Sub = %v, %v; se esperaba 750", diff.Amount, err)
	}
	if _, err := a.Add(other); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("se esperaba ErrCurrencyMismatch, se obtuvo %v", err)
	}
	if _, err := b.Sub(a); !errors.Is(err, ErrNegativeAmount) {
		t.Errorf("restar de mas debe fallar, se obtuvo %v", err)
	}
}

// El importe nunca debe perder precisión: esta prueba documenta por qué no se usa float.
func TestMoneyPrecision(t *testing.T) {
	total, _ := NewMoney(0, "USD")
	diez, _ := NewMoney(10, "USD") // 0.10
	for i := 0; i < 10; i++ {
		total, _ = total.Add(diez)
	}
	if total.Amount != 100 {
		t.Fatalf("diez veces 0.10 debe dar exactamente 1.00, se obtuvo %s", total)
	}
}
