// Package idempotency garantiza que un reintento de red no duplique efectos.
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
)

// Errores del registro de idempotencia.
var (
	// ErrKeyReuse indica misma clave con un cuerpo distinto.
	ErrKeyReuse = errors.New("idempotency key reused with different payload")
	// ErrInFlight indica que una operación con esa clave está en curso.
	ErrInFlight = errors.New("operation in flight")
)

// Record guarda la respuesta asociada a una clave de idempotencia.
type Record struct {
	Key         string
	MerchantID  string
	RequestHash string
	Status      int
	Response    []byte
	Completed   bool
}

// Store conserva los registros de idempotencia.
type Store struct {
	mu      sync.Mutex
	records map[string]*Record
}

// NewStore crea un registro vacío.
func NewStore() *Store { return &Store{records: make(map[string]*Record)} }

// Hash calcula la huella del cuerpo de la petición.
func Hash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Begin reserva la clave. Devuelve el registro previo si la operación ya se
// completó, para responder exactamente lo mismo sin re-ejecutar efectos.
func (s *Store) Begin(_ context.Context, merchantID, key, hash string) (*Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := merchantID + "|" + key
	if r, ok := s.records[k]; ok {
		if r.RequestHash != hash {
			return nil, false, ErrKeyReuse
		}
		if !r.Completed {
			return nil, false, ErrInFlight
		}
		return r, true, nil
	}
	s.records[k] = &Record{Key: key, MerchantID: merchantID, RequestHash: hash}
	return nil, false, nil
}

// Complete almacena la respuesta definitiva de la operación.
func (s *Store) Complete(_ context.Context, merchantID, key string, status int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.records[merchantID+"|"+key]; ok {
		r.Status, r.Response, r.Completed = status, body, true
	}
}

// Discard libera la clave cuando la operación falló y puede reintentarse.
func (s *Store) Discard(_ context.Context, merchantID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, merchantID+"|"+key)
}
