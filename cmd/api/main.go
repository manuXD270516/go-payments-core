// Comando api arranca el servicio HTTP del núcleo de pagos.
package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manuXD270516/go-payments-core/internal/application/usecase"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/events"
	apihttp "github.com/manuXD270516/go-payments-core/internal/infrastructure/http"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/idempotency"
	"github.com/manuXD270516/go-payments-core/internal/infrastructure/persistence/memory"
	"github.com/manuXD270516/go-payments-core/internal/platform/config"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: nivel(cfg.LogLevel)}))

	// Composición de dependencias: los adaptadores se eligen aquí y en ningún
	// otro lugar; el resto del sistema solo conoce puertos.
	intents := memory.NewIntentStore()
	ledger := memory.NewLedgerStore()
	processor := infrastructure.NewSimulatedProcessor()
	publisher := events.NewWebhookPublisher(os.Getenv("WEBHOOK_ENDPOINT"), 5, log)

	payments := &usecase.Payments{
		Intents: intents, Ledger: ledger, Processor: processor,
		Events: publisher, IDs: infrastructure.IDGenerator{}, Clock: infrastructure.SystemClock{},
	}
	api := &apihttp.API{Payments: payments, Idem: idempotency.NewStore(), Log: log}

	srv := &stdhttp.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("servicio escuchando", "port", cfg.HTTPPort, "docs", "/docs")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			log.Error("fallo del servidor", "error", err)
			os.Exit(1)
		}
	}()

	// Apagado ordenado: se dejan terminar las peticiones en curso.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("apagando servicio")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("apagado forzado", "error", err)
	}
}

func nivel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
