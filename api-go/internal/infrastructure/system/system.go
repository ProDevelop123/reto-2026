// Package system contiene los adaptadores hacia los servicios del sistema
// operativo y del entorno de ejecucion: el reloj y la fuente de aleatoriedad.
//
// Son dependencias que suelen usarse directamente (time.Now, rand) y que por
// eso mismo vuelven los tests lentos e intermitentes. Declararlas como puertos
// y resolverlas aqui permite sustituirlas en los tests por implementaciones
// deterministas.
package system

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// Clock devuelve la hora real del sistema. Implementa port.Clock.
type Clock struct{}

// NewClock construye el reloj del sistema.
func NewClock() Clock { return Clock{} }

// Now devuelve el instante actual en UTC.
//
// Siempre en UTC: los tokens se emiten aqui y se verifican en la API de Node,
// que puede correr en otra region. Fijar la zona horaria en el origen evita
// desfases de validez entre servicios.
func (Clock) Now() time.Time { return time.Now().UTC() }

// IDGenerator produce identificadores aleatorios. Implementa port.IDGenerator.
type IDGenerator struct {
	bytes int
}

// NewIDGenerator construye el generador con 128 bits de entropia.
//
// Los identificadores nombran sesiones y familias de tokens, de modo que
// adivinar uno permitiria interferir con la sesion de otro usuario. Por eso se
// usa crypto/rand y no math/rand: este ultimo es predecible a partir de unas
// pocas salidas observadas.
func NewIDGenerator() IDGenerator { return IDGenerator{bytes: 16} }

// NewID devuelve un identificador aleatorio codificado en base64 seguro para URL.
func (g IDGenerator) NewID() (string, error) {
	buffer := make([]byte, g.bytes)

	if _, err := rand.Read(buffer); err != nil {
		// Un fallo de la fuente de entropia del sistema es excepcional, pero
		// devolver un identificador debil seria peor que fallar: se propaga.
		return "", fmt.Errorf("no se pudo generar el identificador: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
