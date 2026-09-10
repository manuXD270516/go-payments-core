package ledger

import (
	"errors"
	"testing"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/domain/shared"
)

func TestCapturaBalancea(t *testing.T) {
	monto, _ := shared.NewMoney(25000, "BOB")
	tx := NewCapture("tx_1", "pi_1", "merch_1", monto, time.Now())
	if err := tx.Balance(); err != nil {
		t.Fatalf("una captura debe balancear: %v", err)
	}
	if len(tx.Entries) != 2 {
		t.Fatalf("se esperaban 2 asientos, hay %d", len(tx.Entries))
	}
}

func TestReembolsoBalancea(t *testing.T) {
	monto, _ := shared.NewMoney(7500, "USD")
	tx := NewRefund("tx_2", "pi_1", "merch_1", monto, time.Now())
	if err := tx.Balance(); err != nil {
		t.Fatalf("un reembolso debe balancear: %v", err)
	}
}

func TestDetectaDescuadre(t *testing.T) {
	d, _ := shared.NewMoney(1000, "BOB")
	c, _ := shared.NewMoney(999, "BOB")
	tx := Transaction{ID: "tx_3", Type: "capture", Entries: []Entry{
		{Account: AccountGatewayClearing, Direction: Debit, Amount: d},
		{Account: AccountMerchantPayable, Direction: Credit, Amount: c},
	}}
	if err := tx.Balance(); !errors.Is(err, ErrUnbalanced) {
		t.Fatalf("se esperaba ErrUnbalanced, se obtuvo %v", err)
	}
}

func TestTransaccionSinAsientos(t *testing.T) {
	if err := (Transaction{ID: "tx_4"}).Balance(); !errors.Is(err, ErrEmptyEntries) {
		t.Fatalf("se esperaba ErrEmptyEntries, se obtuvo %v", err)
	}
}

// Invariante general: cualquier transacción generada por el dominio balancea.
func TestInvarianteGeneral(t *testing.T) {
	now := time.Now()
	for _, monto := range []int64{1, 99, 100, 123456, 999999999} {
		m, _ := shared.NewMoney(monto, "BOB")
		if err := NewCapture("tx", "pi", "m", m, now).Balance(); err != nil {
			t.Errorf("captura de %d no balancea: %v", monto, err)
		}
		if err := NewRefund("tx", "pi", "m", m, now).Balance(); err != nil {
			t.Errorf("reembolso de %d no balancea: %v", monto, err)
		}
	}
}
