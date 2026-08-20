# Imagen de la API de factorizacion QR (Go + Fiber v3).
#
# Construccion multi-etapa: se compila con el toolchain completo de Go y la
# imagen final contiene UNICAMENTE el binario resultante. Ni el compilador, ni
# el codigo fuente, ni los modulos descargados llegan a publicarse.

# ---------------------------------------------------------------------------
# Etapa 1: compilacion
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Los manifiestos se copian antes que el codigo: mientras las dependencias no
# cambien, Docker reutiliza esta capa y se salta la descarga completa.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# La version se inyecta en el enlazado para que la imagen declare que contiene.
ARG VERSION=dev

# CGO_ENABLED=0 produce un binario estaticamente enlazado, sin dependencias de
# libc. Es lo que permite ejecutarlo sobre una imagen base sin sistema operativo.
#
# -trimpath elimina las rutas absolutas de compilacion del binario: sin el, los
# mensajes de error filtrarian la estructura de directorios de la maquina que
# compilo.
#
# -s -w descartan la tabla de simbolos y la informacion de depuracion, lo que
# reduce el binario en torno a un 30%. Se asume la contrapartida: las trazas de
# panico pierden detalle. Es aceptable porque los errores se registran con
# contexto estructurado y el middleware recover los captura.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/api \
    ./cmd/api

# ---------------------------------------------------------------------------
# Etapa 2: verificacion
# ---------------------------------------------------------------------------
# Los tests se ejecutan como una etapa propia para que puedan lanzarse dentro
# del contenedor (docker build --target test) con el detector de carreras
# activo, que en Windows no esta disponible por requerir cgo.
FROM golang:1.25-alpine AS test

WORKDIR /src

# El detector de carreras necesita un compilador de C.
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go test -race ./...

# ---------------------------------------------------------------------------
# Etapa 3: imagen de ejecucion
# ---------------------------------------------------------------------------
# distroless en lugar de alpine: no contiene shell, ni gestor de paquetes, ni
# busybox, ni siquiera un binario que liste directorios. Si alguien consiguiera
# ejecucion remota dentro del contenedor, no encontraria herramientas con las
# que moverse. Es defensa en profundidad real y no cuesta nada.
#
# La etiqueta :nonroot ejecuta como UID 65532 sin privilegios.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=builder /out/api /app/api

ENV APP_ENV=production \
    PORT=8080

EXPOSE 8080

USER nonroot:nonroot

# La sonda invoca al PROPIO binario con el subcomando "healthcheck", que
# consulta GET /health y termina con codigo 0 o 1. Es la forma de sondear una
# imagen distroless: no hay shell ni curl, y anadirlos solo para esto anularia
# la razon de usar distroless. Se declara en forma exec (sin shell de por medio).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/api", "healthcheck"]

ENTRYPOINT ["/app/api"]
