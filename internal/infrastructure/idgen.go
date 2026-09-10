// Package infrastructure ofrece utilidades transversales de los adaptadores.
package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// IDGenerator produce identificadores opacos con prefijo por tipo de recurso.
type IDGenerator struct{}

// NewID devuelve un identificador aleatorio no adivinable.
func (IDGenerator) NewID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().String()))[:16]
	}
	return prefix + "_" + hex.EncodeToString(b)
}

// SystemClock devuelve el tiempo real; en pruebas se sustituye por un reloj fijo.
type SystemClock struct{}

// Now devuelve la hora actual en UTC.
func (SystemClock) Now() time.Time { return time.Now().UTC() }
