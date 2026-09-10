package http

import (
	"log/slog"
	"net/http"
	"time"
)

// requireMerchant exige la identificación del comercio en toda ruta de negocio,
// base del aislamiento multitenant.
func requireMerchant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if merchant(r) == "" {
			writeProblem(w, http.StatusUnauthorized, "merchant_required",
				"Falta la cabecera X-Merchant-Id", "")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverPanic evita que un fallo inesperado tumbe el proceso.
func recoverPanic(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recuperado", "path", r.URL.Path)
				writeProblem(w, http.StatusInternalServerError, "internal_error", "Error interno", "")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logRequests registra cada petición sin volcar cuerpos ni datos sensibles.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		next.ServeHTTP(w, r)
		log.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"merchant", merchant(r), "duration_ms", time.Since(inicio).Milliseconds())
	})
}

// Routes construye el enrutador con la API, la documentación y los health checks.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	negocio := http.NewServeMux()
	negocio.HandleFunc("POST /v1/payment-intents", a.createIntent)
	negocio.HandleFunc("GET /v1/payment-intents/{id}", a.getIntent)
	negocio.HandleFunc("POST /v1/payment-intents/{id}/authorize", a.authorize)
	negocio.HandleFunc("POST /v1/payment-intents/{id}/capture", a.capture)
	negocio.HandleFunc("POST /v1/payment-intents/{id}/refund", a.refund)
	negocio.HandleFunc("POST /v1/payment-intents/{id}/void", a.void)
	negocio.HandleFunc("GET /v1/payment-intents/{id}/ledger", a.getLedger)

	mux.Handle("/v1/", requireMerchant(negocio))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /openapi.yaml", serveOpenAPI)
	mux.HandleFunc("GET /docs", serveSwaggerUI)

	return recoverPanic(a.Log, logRequests(a.Log, mux))
}
