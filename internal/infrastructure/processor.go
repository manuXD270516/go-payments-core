package infrastructure

import (
	"context"
	"strings"

	"github.com/manuXD270516/go-payments-core/internal/application"
)

// SimulatedProcessor implementa el puerto Processor sin salir a la red.
//
// Permite ejercitar el sistema completo en desarrollo y pruebas. Un procesador
// real se agrega como otro adaptador, sin tocar dominio ni casos de uso.
type SimulatedProcessor struct{ IDs IDGenerator }

// NewSimulatedProcessor construye el procesador simulado.
func NewSimulatedProcessor() *SimulatedProcessor { return &SimulatedProcessor{} }

// Authorize aprueba salvo que el token indique explícitamente un rechazo,
// lo que permite probar el camino de error de forma determinista.
func (p *SimulatedProcessor) Authorize(_ context.Context, req application.AuthorizationRequest) (application.AuthorizationResult, error) {
	if strings.HasPrefix(req.Token, "tok_declined") {
		return application.AuthorizationResult{Approved: false, Code: "insufficient_funds"}, nil
	}
	return application.AuthorizationResult{
		Approved:    true,
		ProviderRef: p.IDs.NewID("auth"),
		Code:        "approved",
	}, nil
}
