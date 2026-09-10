package payment

import (
	"errors"
	"testing"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/domain/shared"
)

func nuevoIntent(t *testing.T) *Intent {
	t.Helper()
	amount, err := shared.NewMoney(10000, "BOB")
	if err != nil {
		t.Fatalf("money: %v", err)
	}
	return New("pi_1", "merch_1", amount, Method{Type: "card", Token: "tok_x"}, nil, time.Now())
}

func TestCicloDeVidaFeliz(t *testing.T) {
	i := nuevoIntent(t)
	now := time.Now()
	if err := i.Authorize("prov_1", now); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if i.Status != StatusAuthorized {
		t.Fatalf("estado = %s", i.Status)
	}
	monto, _ := shared.NewMoney(10000, "BOB")
	if err := i.Capture(monto, now); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if i.Captured.Amount != 10000 || i.Status != StatusCaptured {
		t.Fatalf("captura incorrecta: %v %s", i.Captured, i.Status)
	}
	parcial, _ := shared.NewMoney(4000, "BOB")
	if err := i.Refund(parcial, now); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if i.Refunded.Amount != 4000 {
		t.Fatalf("reembolsado = %d", i.Refunded.Amount)
	}
}

func TestTransicionesInvalidas(t *testing.T) {
	now := time.Now()
	monto, _ := shared.NewMoney(1000, "BOB")

	t.Run("capturar sin autorizar", func(t *testing.T) {
		i := nuevoIntent(t)
		if err := i.Capture(monto, now); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("se esperaba ErrInvalidTransition, se obtuvo %v", err)
		}
	})
	t.Run("reembolsar sin capturar", func(t *testing.T) {
		i := nuevoIntent(t)
		_ = i.Authorize("prov_1", now)
		if err := i.Refund(monto, now); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("se esperaba ErrInvalidTransition, se obtuvo %v", err)
		}
	})
	t.Run("anular ya capturado", func(t *testing.T) {
		i := nuevoIntent(t)
		_ = i.Authorize("prov_1", now)
		total, _ := shared.NewMoney(10000, "BOB")
		_ = i.Capture(total, now)
		if err := i.Void(now); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("se esperaba ErrInvalidTransition, se obtuvo %v", err)
		}
	})
}

func TestLimitesDeImporte(t *testing.T) {
	now := time.Now()
	t.Run("captura mayor a lo autorizado", func(t *testing.T) {
		i := nuevoIntent(t)
		_ = i.Authorize("prov_1", now)
		exceso, _ := shared.NewMoney(10001, "BOB")
		if err := i.Capture(exceso, now); !errors.Is(err, ErrCaptureExceedsAuth) {
			t.Fatalf("se esperaba ErrCaptureExceedsAuth, se obtuvo %v", err)
		}
	})
	t.Run("reembolso mayor a lo capturado", func(t *testing.T) {
		i := nuevoIntent(t)
		_ = i.Authorize("prov_1", now)
		parcial, _ := shared.NewMoney(5000, "BOB")
		_ = i.Capture(parcial, now)
		exceso, _ := shared.NewMoney(5001, "BOB")
		if err := i.Refund(exceso, now); !errors.Is(err, ErrRefundExceedsCapture) {
			t.Fatalf("se esperaba ErrRefundExceedsCapture, se obtuvo %v", err)
		}
	})
	t.Run("suma de reembolsos parciales no excede", func(t *testing.T) {
		i := nuevoIntent(t)
		_ = i.Authorize("prov_1", now)
		total, _ := shared.NewMoney(10000, "BOB")
		_ = i.Capture(total, now)
		mitad, _ := shared.NewMoney(6000, "BOB")
		if err := i.Refund(mitad, now); err != nil {
			t.Fatalf("primer reembolso: %v", err)
		}
		if err := i.Refund(mitad, now); !errors.Is(err, ErrRefundExceedsCapture) {
			t.Fatalf("el segundo reembolso debe exceder, se obtuvo %v", err)
		}
	})
}

func TestAislamientoMultitenant(t *testing.T) {
	i := nuevoIntent(t)
	if !i.BelongsTo("merch_1") {
		t.Error("debe pertenecer a su comercio")
	}
	if i.BelongsTo("merch_2") {
		t.Error("no debe pertenecer a otro comercio")
	}
}
