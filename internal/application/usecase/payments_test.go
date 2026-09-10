package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/application"
	"github.com/manuXD270516/go-payments-core/internal/domain/ledger"
	"github.com/manuXD270516/go-payments-core/internal/domain/payment"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/persistence/memory"
)

type relojFijo struct{ t time.Time }

func (r relojFijo) Now() time.Time { return r.t }

type idsSecuenciales struct {
	mu sync.Mutex
	n  int
}

func (g *idsSecuenciales) NewID(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return prefix + "_" + string(rune('a'+g.n%26)) + string(rune('0'+g.n%10))
}

type procesadorFalso struct{ aprobar bool }

func (p procesadorFalso) Authorize(context.Context, application.AuthorizationRequest) (application.AuthorizationResult, error) {
	if !p.aprobar {
		return application.AuthorizationResult{Approved: false, Code: "do_not_honor"}, nil
	}
	return application.AuthorizationResult{Approved: true, ProviderRef: "auth_x", Code: "approved"}, nil
}

func nuevoServicio(t *testing.T, aprobar bool) (*Payments, *memory.LedgerStore) {
	t.Helper()
	lg := memory.NewLedgerStore()
	return &Payments{
		Intents:   memory.NewIntentStore(),
		Ledger:    lg,
		Processor: procesadorFalso{aprobar: aprobar},
		IDs:       &idsSecuenciales{},
		Clock:     relojFijo{t: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)},
	}, lg
}

func crear(t *testing.T, s *Payments) *payment.Intent {
	t.Helper()
	i, err := s.CreateIntent(context.Background(), CreateIntentInput{
		MerchantID: "merch_1", Amount: 20000, Currency: "BOB", MethodType: "card", Token: "tok_ok",
	})
	if err != nil {
		t.Fatalf("crear intent: %v", err)
	}
	return i
}

func TestFlujoCompleto(t *testing.T) {
	s, lg := nuevoServicio(t, true)
	ctx := context.Background()
	i := crear(t, s)

	if _, err := s.Authorize(ctx, "merch_1", i.ID); err != nil {
		t.Fatalf("autorizar: %v", err)
	}
	capturado, err := s.Capture(ctx, "merch_1", i.ID, 20000)
	if err != nil {
		t.Fatalf("capturar: %v", err)
	}
	if capturado.Captured.Amount != 20000 {
		t.Fatalf("capturado = %d", capturado.Captured.Amount)
	}
	if _, err := s.Refund(ctx, "merch_1", i.ID, 5000); err != nil {
		t.Fatalf("reembolsar: %v", err)
	}

	entries, err := lg.EntriesByIntent(ctx, "merch_1", i.ID)
	if err != nil {
		t.Fatalf("asientos: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("se esperaban 4 asientos (captura + reembolso), hay %d", len(entries))
	}
	// Invariante contable: débitos y créditos se compensan.
	var saldo int64
	for _, e := range entries {
		if e.Direction == ledger.Debit {
			saldo += e.Amount.Amount
		} else {
			saldo -= e.Amount.Amount
		}
	}
	if saldo != 0 {
		t.Fatalf("el ledger no balancea: %d", saldo)
	}
}

func TestAutorizacionRechazada(t *testing.T) {
	s, _ := nuevoServicio(t, false)
	i := crear(t, s)
	if _, err := s.Authorize(context.Background(), "merch_1", i.ID); !errors.Is(err, ErrDeclined) {
		t.Fatalf("se esperaba ErrDeclined, se obtuvo %v", err)
	}
}

func TestAislamientoEntreComercios(t *testing.T) {
	s, _ := nuevoServicio(t, true)
	i := crear(t, s)
	if _, err := s.Get(context.Background(), "merch_2", i.ID); !errors.Is(err, payment.ErrNotFound) {
		t.Fatalf("otro comercio no debe ver el intent, se obtuvo %v", err)
	}
}

// Dos capturas simultáneas sobre el mismo intento deben producir una sola
// captura efectiva: el bloqueo optimista rechaza la perdedora.
func TestCapturasConcurrentes(t *testing.T) {
	s, lg := nuevoServicio(t, true)
	ctx := context.Background()
	i := crear(t, s)
	if _, err := s.Authorize(ctx, "merch_1", i.ID); err != nil {
		t.Fatalf("autorizar: %v", err)
	}

	const n = 50
	var wg sync.WaitGroup
	exitos := make(chan struct{}, n)
	for k := 0; k < n; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Capture(ctx, "merch_1", i.ID, 20000); err == nil {
				exitos <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(exitos)
	if got := len(exitos); got != 1 {
		t.Fatalf("se esperaba exactamente 1 captura efectiva, hubo %d", got)
	}
	entries, _ := lg.EntriesByIntent(ctx, "merch_1", i.ID)
	if len(entries) != 2 {
		t.Fatalf("se esperaban 2 asientos de una sola captura, hay %d", len(entries))
	}
}
