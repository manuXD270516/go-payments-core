package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/manuXD270516/go-payments-core/internal/application/usecase"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/idempotency"
)

// API agrupa las dependencias de los handlers.
type API struct {
	Payments *usecase.Payments
	Idem     *idempotency.Store
	Log      *slog.Logger
}

const headerMerchant = "X-Merchant-Id"
const headerIdempotency = "Idempotency-Key"

func merchant(r *http.Request) string { return r.Header.Get(headerMerchant) }

type createIntentBody struct {
	Amount   int64             `json:"amount"`
	Currency string            `json:"currency"`
	Method   string            `json:"method"`
	Token    string            `json:"token"`
	Metadata map[string]string `json:"metadata"`
}

type amountBody struct {
	Amount int64 `json:"amount"`
}

// withIdempotency envuelve una operación de escritura para que un reintento con
// la misma clave devuelva la respuesta original sin repetir el efecto.
func (a *API) withIdempotency(w http.ResponseWriter, r *http.Request, body []byte, fn func() (int, any, error)) {
	key := r.Header.Get(headerIdempotency)
	if key == "" {
		writeProblem(w, http.StatusBadRequest, "idempotency_key_required",
			"Falta la cabecera Idempotency-Key", "")
		return
	}
	m := merchant(r)
	prev, replay, err := a.Idem.Begin(r.Context(), m, key, idempotency.Hash(body))
	if err != nil {
		mapError(w, err)
		return
	}
	if replay {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Idempotency-Replayed", "true")
		w.WriteHeader(prev.Status)
		_, _ = w.Write(prev.Response)
		return
	}
	status, payload, err := fn()
	if err != nil {
		a.Idem.Discard(r.Context(), m, key)
		mapError(w, err)
		return
	}
	written := writeJSON(w, status, payload)
	a.Idem.Complete(r.Context(), m, key, status, written)
}

func (a *API) createIntent(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var in createIntentBody
	if err := json.Unmarshal(raw, &in); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_json", "Cuerpo invalido", "")
		return
	}
	a.withIdempotency(w, r, raw, func() (int, any, error) {
		intent, err := a.Payments.CreateIntent(r.Context(), usecase.CreateIntentInput{
			MerchantID: merchant(r), Amount: in.Amount, Currency: in.Currency,
			MethodType: in.Method, Token: in.Token, Metadata: in.Metadata,
		})
		return http.StatusCreated, intent, err
	})
}

func (a *API) authorize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	a.withIdempotency(w, r, append([]byte(id), raw...), func() (int, any, error) {
		intent, err := a.Payments.Authorize(r.Context(), merchant(r), id)
		return http.StatusOK, intent, err
	})
}

func (a *API) capture(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var in amountBody
	_ = json.Unmarshal(raw, &in)
	a.withIdempotency(w, r, append([]byte(id), raw...), func() (int, any, error) {
		intent, err := a.Payments.Capture(r.Context(), merchant(r), id, in.Amount)
		return http.StatusOK, intent, err
	})
}

func (a *API) refund(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var in amountBody
	_ = json.Unmarshal(raw, &in)
	a.withIdempotency(w, r, append([]byte(id), raw...), func() (int, any, error) {
		intent, err := a.Payments.Refund(r.Context(), merchant(r), id, in.Amount)
		return http.StatusOK, intent, err
	})
}

func (a *API) void(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	a.withIdempotency(w, r, append([]byte(id), raw...), func() (int, any, error) {
		intent, err := a.Payments.Void(r.Context(), merchant(r), id)
		return http.StatusOK, intent, err
	})
}

func (a *API) getIntent(w http.ResponseWriter, r *http.Request) {
	intent, err := a.Payments.Get(r.Context(), merchant(r), r.PathValue("id"))
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, intent)
}

func (a *API) getLedger(w http.ResponseWriter, r *http.Request) {
	entries, err := a.Payments.LedgerEntries(r.Context(), merchant(r), r.PathValue("id"))
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
