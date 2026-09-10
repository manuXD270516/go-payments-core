// Package memory ofrece adaptadores en memoria, útiles para desarrollo y pruebas.
package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/manuXD270516/go-payments-core/internal/domain/ledger"
	"github.com/manuXD270516/go-payments-core/internal/domain/payment"
)

// ErrConflict indica que el intento fue modificado por otra operación.
var ErrConflict = errors.New("version conflict")

// IntentStore es un repositorio de intentos protegido para uso concurrente.
type IntentStore struct {
	mu    sync.RWMutex
	items map[string]payment.Intent
}

// NewIntentStore crea un repositorio vacío.
func NewIntentStore() *IntentStore {
	return &IntentStore{items: make(map[string]payment.Intent)}
}

func key(merchantID, id string) string { return merchantID + "|" + id }

// Save guarda un intento nuevo.
func (s *IntentStore) Save(_ context.Context, i *payment.Intent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key(i.MerchantID, i.ID)] = *i
	return nil
}

// Update aplica el cambio solo si la versión almacenada es la esperada.
// Esto serializa operaciones concurrentes sobre el mismo intento: dos capturas
// simultáneas no pueden ambas tener éxito.
func (s *IntentStore) Update(_ context.Context, i *payment.Intent, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	actual, ok := s.items[key(i.MerchantID, i.ID)]
	if !ok {
		return payment.ErrNotFound
	}
	if actual.Version != expectedVersion {
		return ErrConflict
	}
	s.items[key(i.MerchantID, i.ID)] = *i
	return nil
}

// FindByID recupera un intento del comercio indicado.
func (s *IntentStore) FindByID(_ context.Context, merchantID, id string) (*payment.Intent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.items[key(merchantID, id)]
	if !ok {
		return nil, payment.ErrNotFound
	}
	copia := i
	return &copia, nil
}

// LedgerStore es un repositorio de asientos contables en memoria.
type LedgerStore struct {
	mu      sync.RWMutex
	entries []ledger.Entry
}

// NewLedgerStore crea un ledger vacío.
func NewLedgerStore() *LedgerStore { return &LedgerStore{} }

// Record persiste los asientos de una transacción, validando el balance.
func (s *LedgerStore) Record(_ context.Context, tx ledger.Transaction) error {
	if err := tx.Balance(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, tx.Entries...)
	return nil
}

// EntriesByIntent devuelve los asientos de un intento del comercio indicado.
func (s *LedgerStore) EntriesByIntent(_ context.Context, merchantID, intentID string) ([]ledger.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ledger.Entry
	for _, e := range s.entries {
		if e.MerchantID == merchantID && e.IntentID == intentID {
			out = append(out, e)
		}
	}
	return out, nil
}
