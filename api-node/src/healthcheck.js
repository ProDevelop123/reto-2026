/**
 * Sonda de salud del contenedor.
 *
 * Consulta GET /health contra el propio proceso y termina con codigo 0 si
 * responde correctamente, o 1 en cualquier otro caso.
 *
 * Existe por la misma razon que su equivalente en la API de Go: la imagen final
 * es distroless y no contiene shell, ni curl, ni wget con los que sondear.
 * Anadir alguna de esas herramientas solo para el healthcheck anularia el
 * motivo de usar distroless, que es no ofrecer utilidades a quien logre entrar
 * en el contenedor.
 *
 * Uso:  node src/healthcheck.js
 */

const port = process.env.PORT || '3001';

// La sonda debe fallar rapido: si el servicio no responde en dos segundos,
// esperar mas solo retrasa la deteccion de que algo va mal.
const TIMEOUT_MS = 2000;

// Se consulta 127.0.0.1 y no el nombre del servicio: la sonda comprueba ESTE
// proceso, no la resolucion de nombres de la red del contenedor.
const url = `http://127.0.0.1:${port}/health`;

try {
  const response = await fetch(url, { signal: AbortSignal.timeout(TIMEOUT_MS) });

  if (!response.ok) {
    process.stderr.write(`healthcheck: estado ${response.status}\n`);
    process.exit(1);
  }

  process.exit(0);
} catch (error) {
  process.stderr.write(`healthcheck: sin respuesta: ${error.message}\n`);
  process.exit(1);
}
