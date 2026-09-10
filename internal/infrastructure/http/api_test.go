package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/manuXD270516/go-payments-core/internal/application/usecase"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/idempotency"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/persistence/memory"
)

func nuevoServidor(t *testing.T) *httptest.Server {
	t.Helper()
	payments := &usecase.Payments{
		Intents:   memory.NewIntentStore(),
		Ledger:    memory.NewLedgerStore(),
		Processor: infrastructure.NewSimulatedProcessor(),
		IDs:       infrastructure.IDGenerator{},
		Clock:     infrastructure.SystemClock{},
	}
	api := &API{
		Payments: payments,
		Idem:     idempotency.NewStore(),
		Log:      slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	srv := httptest.NewServer(api.Routes())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, path, merchant, idemKey, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if merchant != "" {
		req.Header.Set("X-Merchant-Id", merchant)
	}
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	var out map[string]any
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	res.Body.Close()
	_ = json.Unmarshal(buf.Bytes(), &out)
	return res, out
}

func TestCreacionEsIdempotente(t *testing.T) {
	srv := nuevoServidor(t)
	cuerpo := `{"amount":15000,"currency":"BOB","method":"card","token":"tok_ok"}`

	res1, body1 := post(t, srv, "/v1/payment-intents", "merch_1", "key-1", cuerpo)
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("primer intento: status %d", res1.StatusCode)
	}
	res2, body2 := post(t, srv, "/v1/payment-intents", "merch_1", "key-1", cuerpo)
	if res2.StatusCode != http.StatusCreated {
		t.Fatalf("reintento: status %d", res2.StatusCode)
	}
	if body1["id"] != body2["id"] {
		t.Fatalf("el reintento creo otro intent: %v vs %v", body1["id"], body2["id"])
	}
	if res2.Header.Get("Idempotency-Replayed") != "true" {
		t.Error("el reintento debe marcarse como repetido")
	}
}

func TestMismaClaveCuerpoDistinto(t *testing.T) {
	srv := nuevoServidor(t)
	post(t, srv, "/v1/payment-intents", "merch_1", "key-2", `{"amount":100,"currency":"BOB","method":"card"}`)
	res, body := post(t, srv, "/v1/payment-intents", "merch_1", "key-2", `{"amount":999,"currency":"BOB","method":"card"}`)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, se esperaba 422", res.StatusCode)
	}
	if body["code"] != "idempotency_key_reuse" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestFaltaCabeceraComercio(t *testing.T) {
	srv := nuevoServidor(t)
	res, body := post(t, srv, "/v1/payment-intents", "", "key-3", `{"amount":100,"currency":"BOB","method":"card"}`)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, se esperaba 401", res.StatusCode)
	}
	if body["code"] != "merchant_required" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestCapturaSinAutorizarDevuelve409(t *testing.T) {
	srv := nuevoServidor(t)
	_, creado := post(t, srv, "/v1/payment-intents", "merch_1", "key-4", `{"amount":5000,"currency":"BOB","method":"card","token":"tok_ok"}`)
	id, _ := creado["id"].(string)

	res, body := post(t, srv, "/v1/payment-intents/"+id+"/capture", "merch_1", "key-5", `{"amount":5000}`)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, se esperaba 409", res.StatusCode)
	}
	if body["code"] != "invalid_state_transition" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestReembolsoExcedeCaptura(t *testing.T) {
	srv := nuevoServidor(t)
	_, creado := post(t, srv, "/v1/payment-intents", "merch_1", "k1", `{"amount":5000,"currency":"BOB","method":"card","token":"tok_ok"}`)
	id := creado["id"].(string)
	post(t, srv, "/v1/payment-intents/"+id+"/authorize", "merch_1", "k2", "")
	post(t, srv, "/v1/payment-intents/"+id+"/capture", "merch_1", "k3", `{"amount":3000}`)

	res, body := post(t, srv, "/v1/payment-intents/"+id+"/refund", "merch_1", "k4", `{"amount":4000}`)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, se esperaba 422", res.StatusCode)
	}
	if body["code"] != "refund_exceeds_captured" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestAutorizacionRechazadaDevuelve402(t *testing.T) {
	srv := nuevoServidor(t)
	_, creado := post(t, srv, "/v1/payment-intents", "merch_1", "d1", `{"amount":5000,"currency":"BOB","method":"card","token":"tok_declined_1"}`)
	id := creado["id"].(string)
	res, body := post(t, srv, "/v1/payment-intents/"+id+"/authorize", "merch_1", "d2", "")
	if res.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, se esperaba 402", res.StatusCode)
	}
	if body["code"] != "authorization_declined" {
		t.Errorf("code = %v", body["code"])
	}
}

func TestOtroComercioNoAccede(t *testing.T) {
	srv := nuevoServidor(t)
	_, creado := post(t, srv, "/v1/payment-intents", "merch_1", "x1", `{"amount":5000,"currency":"BOB","method":"card","token":"tok_ok"}`)
	id := creado["id"].(string)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/payment-intents/"+id, nil)
	req.Header.Set("X-Merchant-Id", "merch_2")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, se esperaba 404", res.StatusCode)
	}
}

func TestLedgerBalanceaViaAPI(t *testing.T) {
	srv := nuevoServidor(t)
	_, creado := post(t, srv, "/v1/payment-intents", "merch_1", "l1", `{"amount":8000,"currency":"BOB","method":"card","token":"tok_ok"}`)
	id := creado["id"].(string)
	post(t, srv, "/v1/payment-intents/"+id+"/authorize", "merch_1", "l2", "")
	post(t, srv, "/v1/payment-intents/"+id+"/capture", "merch_1", "l3", `{"amount":8000}`)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/payment-intents/"+id+"/ledger", nil)
	req.Header.Set("X-Merchant-Id", "merch_1")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()

	var payload struct {
		Entries []struct {
			Direction string `json:"direction"`
			Amount    struct {
				Amount int64 `json:"amount"`
			} `json:"amount"`
		} `json:"entries"`
	}
	_ = json.NewDecoder(res.Body).Decode(&payload)
	if len(payload.Entries) != 2 {
		t.Fatalf("se esperaban 2 asientos, hay %d", len(payload.Entries))
	}
	var saldo int64
	for _, e := range payload.Entries {
		if e.Direction == "debit" {
			saldo += e.Amount.Amount
		} else {
			saldo -= e.Amount.Amount
		}
	}
	if saldo != 0 {
		t.Fatalf("el ledger expuesto no balancea: %d", saldo)
	}
}

func TestDocumentacionDisponible(t *testing.T) {
	srv := nuevoServidor(t)
	for _, ruta := range []string{"/healthz", "/readyz", "/openapi.yaml", "/docs"} {
		res, err := srv.Client().Get(srv.URL + ruta)
		if err != nil {
			t.Fatalf("get %s: %v", ruta, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s devolvio %d", ruta, res.StatusCode)
		}
	}
}
