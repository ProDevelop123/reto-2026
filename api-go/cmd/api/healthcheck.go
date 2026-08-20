package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// healthcheckCommand es el nombre del subcomando de sonda.
const healthcheckCommand = "healthcheck"

// runHealthcheck consulta el endpoint de salud del propio proceso y termina
// con codigo 0 si responde correctamente, o 1 en cualquier otro caso.
//
// Existe porque la imagen final es distroless: no contiene shell, ni curl, ni
// wget con los que sondear el servicio. Anadir alguna de esas herramientas
// solo para el healthcheck echaria por tierra la razon de usar distroless, que
// es precisamente no ofrecer utilidades a quien logre entrar en el contenedor.
//
// La alternativa idiomatica es la que se implementa aqui: el mismo binario que
// sirve la API sabe tambien comprobarla. No anade superficie de ataque —el
// ejecutable ya esta en la imagen— y funciona igual en Docker, en Compose y en
// Kubernetes.
//
// Uso:  /app/api healthcheck
func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// La sonda debe fallar rapido: si el servicio no responde en dos segundos,
	// esperar mas solo retrasa la deteccion de que algo va mal.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Se consulta 127.0.0.1 y no el nombre del servicio: la sonda comprueba
	// ESTE proceso, no la resolucion de nombres de la red.
	url := fmt.Sprintf("http://127.0.0.1:%s/health", port)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: peticion invalida: %v\n", err)
		return 1
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: sin respuesta: %v\n", err)
		return 1
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: estado %d\n", response.StatusCode)
		return 1
	}

	return 0
}
