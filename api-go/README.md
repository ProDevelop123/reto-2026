# API de Factorización QR (Go + Fiber v3)

Primera API del reto y **orquestadora del sistema**. Recibe una matriz
rectangular, calcula su factorización QR mediante reflexiones de Householder,
envía Q y R a la API de estadísticas en Node y devuelve todo en una sola
respuesta.

> **Documentación detallada:** [`docs/backend.md`](../docs/backend.md)
> — el algoritmo paso a paso, la arquitectura hexagonal, autenticación,
> referencia de endpoints y variables de entorno.

---

## Arranque

Lo mas comodo es levantar todo el sistema desde la raiz del repositorio, que
genera las claves automaticamente:

```bash
docker compose up -d --build
```

Para ejecutar solo este servicio, fuera de Docker:

```bash
sh ../scripts/generate-keys.sh ../keys   # solo si no hay claves todavia
cp .env.example .env
go run ./cmd/api
```

Requiere la API de Node escuchando en `STATISTICS_API_URL`.

## Tests

```bash
go test ./... -cover                                  # 29 tests (51 con subtests)
go test ./internal/matrix/ -bench=. -benchtime=20x    # benchmark
docker build --target test .                          # con -race, en Linux
```

| Paquete | Cobertura |
|---|---|
| `internal/matrix` | 97,0 % |
| `internal/core/usecase` | 82,5 % |

## Estructura

```
cmd/api/              arranque y composición de dependencias
internal/
├─ matrix/            álgebra pura — cero dependencias externas
├─ core/              domain · port · usecase (no conoce HTTP)
├─ infrastructure/    Fiber · JWT · cliente HTTP · persistencia
└─ config/
pkg/apperror/         error de aplicación con código HTTP
```

## Endpoints

| Método | Ruta | Autenticación |
|---|---|---|
| `POST` | `/api/v1/qr` | `Authorization: Bearer` |
| `POST` | `/api/v1/auth/login` | — |
| `POST` | `/api/v1/auth/refresh` | Cookie + `X-Refresh-Request` |
| `POST` | `/api/v1/auth/logout` | Cookie + `X-Refresh-Request` |
| `GET` | `/health` | — |

## En una línea

Householder por estabilidad numérica · arquitectura hexagonal porque hay una
dependencia externa que invertir · RS256 asimétrico porque este servicio es el
único que debe poder firmar · 10 asignaciones de memoria sea cual sea el tamaño
de la matriz.
