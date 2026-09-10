// Package ledger implementa el registro contable de doble partida.
package ledger

import (
	"errors"
	"fmt"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/domain/shared"
)

// Errores de dominio del ledger.
var (
	ErrUnbalanced   = errors.New("ledger entries do not balance")
	ErrEmptyEntries = errors.New("no ledger entries")
)

// Direction indica el lado del asiento.
type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

// Cuentas contables usadas por el núcleo.
const (
	AccountMerchantReceivable = "merchant_receivable"
	AccountGatewayClearing    = "gateway_clearing"
	AccountMerchantPayable    = "merchant_payable"
	AccountRefundsPayable     = "refunds_payable"
)

// Entry es un asiento contable individual.
type Entry struct {
	ID            string       `json:"id"`
	TransactionID string       `json:"transaction_id"`
	IntentID      string       `json:"intent_id"`
	MerchantID    string       `json:"merchant_id"`
	Account       string       `json:"account"`
	Direction     Direction    `json:"direction"`
	Amount        shared.Money `json:"amount"`
	CreatedAt     time.Time    `json:"created_at"`
}

// Transaction agrupa los asientos de un mismo movimiento de dinero.
// Su invariante central es que débitos y créditos se compensan.
type Transaction struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"` // capture | refund
	Entries []Entry `json:"entries"`
}

// Balance verifica el invariante de doble partida: la suma de débitos debe
// igualar a la de créditos, por moneda. Si no balancea, la operación completa
// debe abortarse.
func (t Transaction) Balance() error {
	if len(t.Entries) == 0 {
		return ErrEmptyEntries
	}
	saldos := make(map[string]int64)
	for _, e := range t.Entries {
		switch e.Direction {
		case Debit:
			saldos[e.Amount.Currency] += e.Amount.Amount
		case Credit:
			saldos[e.Amount.Currency] -= e.Amount.Amount
		default:
			return fmt.Errorf("%w: direccion desconocida %q", ErrUnbalanced, e.Direction)
		}
	}
	for currency, saldo := range saldos {
		if saldo != 0 {
			return fmt.Errorf("%w: %s descuadra en %d", ErrUnbalanced, currency, saldo)
		}
	}
	return nil
}

// NewCapture arma los asientos de una captura: el comercio pasa a tener un
// cobro por recibir y la pasarela registra el importe en compensación.
func NewCapture(txID, intentID, merchantID string, amount shared.Money, now time.Time) Transaction {
	return Transaction{
		ID: txID, Type: "capture",
		Entries: []Entry{
			{ID: txID + "_d", TransactionID: txID, IntentID: intentID, MerchantID: merchantID,
				Account: AccountGatewayClearing, Direction: Debit, Amount: amount, CreatedAt: now},
			{ID: txID + "_c", TransactionID: txID, IntentID: intentID, MerchantID: merchantID,
				Account: AccountMerchantPayable, Direction: Credit, Amount: amount, CreatedAt: now},
		},
	}
}

// NewRefund arma los asientos de un reembolso, en sentido inverso a la captura.
func NewRefund(txID, intentID, merchantID string, amount shared.Money, now time.Time) Transaction {
	return Transaction{
		ID: txID, Type: "refund",
		Entries: []Entry{
			{ID: txID + "_d", TransactionID: txID, IntentID: intentID, MerchantID: merchantID,
				Account: AccountMerchantPayable, Direction: Debit, Amount: amount, CreatedAt: now},
			{ID: txID + "_c", TransactionID: txID, IntentID: intentID, MerchantID: merchantID,
				Account: AccountRefundsPayable, Direction: Credit, Amount: amount, CreatedAt: now},
		},
	}
}
