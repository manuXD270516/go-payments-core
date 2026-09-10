// Package http expone el núcleo de pagos como API REST.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/manuXD270516/go-payments-core/internal/application/usecase"
	"github.com/manuXD270516/go-payments-core/internal/domain/ledger"
	"github.com/manuXD270516/go-payments-core/internal/domain/payment"
	"github.com/manuXD270516/go-payments-core/internal/domain/shared"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/idempotency"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/persistence/memory"
)

// Problem implementa el formato de error RFC 7807.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code"`
}

// writeProblem serializa un problema en application/problem+json.
func writeProblem(w http.ResponseWriter, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type:   "https://docs.paygo.dev/errors/" + code,
		Title:  title,
		Status: status,
		Detail: detail,
		Code:   code,
	})
}

// mapError traduce errores de dominio a respuestas HTTP con código estable.
// Los consumidores programan contra estos códigos, no contra mensajes.
func mapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, payment.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "payment_intent_not_found", "Intento de pago no encontrado", "")
	case errors.Is(err, payment.ErrForbidden):
		writeProblem(w, http.StatusForbidden, "forbidden", "El recurso pertenece a otro comercio", "")
	case errors.Is(err, payment.ErrInvalidTransition):
		writeProblem(w, http.StatusConflict, "invalid_state_transition", "Transicion de estado invalida", err.Error())
	case errors.Is(err, payment.ErrRefundExceedsCapture):
		writeProblem(w, http.StatusUnprocessableEntity, "refund_exceeds_captured", "El reembolso excede lo capturado", err.Error())
	case errors.Is(err, payment.ErrCaptureExceedsAuth):
		writeProblem(w, http.StatusUnprocessableEntity, "capture_exceeds_authorized", "La captura excede lo autorizado", err.Error())
	case errors.Is(err, usecase.ErrDeclined):
		writeProblem(w, http.StatusPaymentRequired, "authorization_declined", "Autorizacion rechazada", err.Error())
	case errors.Is(err, shared.ErrCurrencyMismatch):
		writeProblem(w, http.StatusUnprocessableEntity, "currency_mismatch", "Monedas incompatibles", err.Error())
	case errors.Is(err, shared.ErrNegativeAmount), errors.Is(err, shared.ErrInvalidCurrency):
		writeProblem(w, http.StatusUnprocessableEntity, "invalid_amount", "Importe o moneda invalidos", err.Error())
	case errors.Is(err, ledger.ErrUnbalanced):
		writeProblem(w, http.StatusInternalServerError, "ledger_unbalanced", "Los asientos no balancean", "")
	case errors.Is(err, memory.ErrConflict):
		writeProblem(w, http.StatusConflict, "concurrent_modification", "El recurso cambio durante la operacion", "")
	case errors.Is(err, idempotency.ErrKeyReuse):
		writeProblem(w, http.StatusUnprocessableEntity, "idempotency_key_reuse", "La clave de idempotencia se reuso con otro cuerpo", "")
	case errors.Is(err, idempotency.ErrInFlight):
		writeProblem(w, http.StatusConflict, "operation_in_flight", "Ya hay una operacion en curso con esa clave", "")
	default:
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Error interno", "")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) []byte {
	body, err := json.Marshal(v)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "encoding_error", "No se pudo serializar la respuesta", "")
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
	return body
}
