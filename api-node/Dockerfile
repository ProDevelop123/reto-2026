# Imagen de la API de estadisticas (Node.js + Express).
#
# Construccion multi-etapa. La imagen final es distroless, igual que la de la
# API en Go: ambos servicios comparten la misma postura de seguridad en tiempo
# de ejecucion, sin shell ni gestor de paquetes disponibles dentro.

# ---------------------------------------------------------------------------
# Etapa 1: dependencias de produccion
# ---------------------------------------------------------------------------
# Se usa una base Debian (slim) y no Alpine porque la imagen final es
# distroless, que tambien es Debian. Resolver las dependencias sobre musl y
# ejecutarlas sobre glibc funcionaria con paquetes de JavaScript puro, pero
# rompeexactamente el dia que alguien anada una dependencia con codigo nativo.
FROM node:22-slim AS deps

WORKDIR /app

# Los manifiestos se copian antes que el codigo: mientras las dependencias no
# cambien, Docker reutiliza esta capa y se salta la instalacion completa.
COPY package.json package-lock.json ./

# `npm ci` instala exactamente lo fijado en el lockfile, de forma reproducible,
# a diferencia de `npm install`, que puede resolver versiones nuevas.
RUN npm ci --omit=dev && npm cache clean --force

# ---------------------------------------------------------------------------
# Etapa 2: verificacion
# ---------------------------------------------------------------------------
# La bateria se ejecuta como etapa propia (docker build --target test) para que
# pueda correr en CI dentro del mismo contenedor que se publica, y no solo en
# la maquina de quien desarrolla.
FROM node:22-slim AS test

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY src ./src
COPY tests ./tests
COPY jest.config.js ./

RUN npm test

# ---------------------------------------------------------------------------
# Etapa 3: imagen de ejecucion
# ---------------------------------------------------------------------------
# distroless: no contiene shell, ni gestor de paquetes, ni utilidades del
# sistema. Si alguien consiguiera ejecucion remota dentro del contenedor, no
# encontraria herramientas con las que moverse.
#
# La etiqueta :nonroot ejecuta como UID 65532 sin privilegios.
#
# Su ENTRYPOINT ya es el propio ejecutable de Node, de modo que CMD solo indica
# el script que debe ejecutarse.
FROM gcr.io/distroless/nodejs22-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./
COPY src ./src

ENV NODE_ENV=production \
    PORT=3001

EXPOSE 3001

# La sonda invoca al propio Node con el script de healthcheck, que consulta
# GET /health y termina con codigo 0 o 1. Se declara en forma exec porque no
# hay shell que interprete la forma de cadena.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/nodejs/bin/node", "src/healthcheck.js"]

# No hace falta un init como dumb-init: el cierre ordenado del servidor se
# apoya en un manejador explicito de SIGTERM registrado en server.js, y las
# senales con manejador SI se entregan al proceso 1. Ademas el servicio no
# lanza procesos hijos, asi que no hay zombis que recolectar.
CMD ["src/server.js"]
