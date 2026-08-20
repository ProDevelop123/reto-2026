# Documentación de Infraestructura

Contenerización, red, seguridad de los contenedores, operación y despliegue.

**Índice**

1. [Topología](#1-topología)
2. [Las imágenes](#2-las-imágenes)
3. [Sondas de salud sin shell](#3-sondas-de-salud-sin-shell)
4. [docker-compose](#4-docker-compose)
5. [Gestión de claves](#5-gestión-de-claves)
6. [Endurecimiento de los contenedores](#6-endurecimiento-de-los-contenedores)
7. [Operación diaria](#7-operación-diaria)
8. [Resolución de problemas](#8-resolución-de-problemas)
9. [Lecciones al containerizar](#9-lecciones-al-containerizar)
10. [Guía de despliegue](#10-guía-de-despliegue)

---

## 1. Topología

```
                      INTERNET / HOST
                            │
                            │ puerto 8080 (único publicado)
                            ▼
   ┌────────────────────────────────────────────────────┐
   │  red "reto"  (bridge definida por el usuario)      │
   │                                                     │
   │   ┌─────────────────────┐                          │
   │   │  api-go             │                          │
   │   │  Fiber v3 · :8080   │   http://api-node:3001   │
   │   │  distroless 20 MB   │─────────────┐            │
   │   └─────────────────────┘             │            │
   │                                        ▼           │
   │                         ┌─────────────────────┐    │
   │                         │  api-node           │    │
   │                         │  Express · :3001    │    │
   │                         │  distroless 221 MB  │    │
   │                         │  SIN PUERTO PÚBLICO │    │
   │                         └─────────────────────┘    │
   └────────────────────────────────────────────────────┘
```

### La decisión de red

**La API de Node no publica ningún puerto al host.** Solo la alcanza la API en
Go, a través de la red interna.

Publicarla la expondría sin necesidad y permitiría saltarse por completo al
orquestador —incluida su validación de token—. Es el principio de menor
privilegio aplicado a la topología.

En el `docker-compose.yml` la línea está escrita, **comentada y explicada**, por
si conviene inspeccionarla de forma aislada durante una demostración:

```yaml
# ports:
#   - "3001:3001"
```

Se puede comprobar:

```bash
curl http://localhost:3001/health
# → connection refused
```

```bash
docker compose ps
# api-node   Up (healthy)   3001/tcp        ← expuesto, NO publicado
# api-go     Up (healthy)   0.0.0.0:8080->8080/tcp
```

`3001/tcp` sin `0.0.0.0->` delante significa: el contenedor escucha, pero nada
lo mapea al host.

### Descubrimiento de servicio

Go encuentra a Node con una variable de entorno:

```yaml
STATISTICS_API_URL: http://api-node:3001
```

`api-node` lo resuelve el **DNS embebido de Docker**, que registra cada servicio
como alias en su red. La IP del contenedor cambia en cada arranque sin que a
nadie le importe.

> **Detalle que mucha gente ignora:** esto solo funciona en **redes definidas
> por el usuario**. En la red `bridge` por defecto de Docker **no hay resolución
> por nombre**. Por eso el compose declara su red explícitamente.

**El nombre de la variable importa.** Se llama `STATISTICS_API_URL`, no
`NODE_API_URL`: nombra la **capacidad**, no la tecnología. Si mañana esa
capacidad la sirviera un servicio en otro lenguaje, `NODE_API_URL` sería una
variable que miente.

### Por qué no hay service discovery más sofisticado

Consul, etcd o un service mesh resuelven problemas que aquí no existen:
múltiples instancias por servicio, registro dinámico, balanceo con enrutado
según salud, mTLS entre servicios.

Con dos servicios y un orquestador que ya provee DNS, añadir un registro externo
sería más piezas que mantener y más modos de fallo, a cambio de nada.

> Se delega el descubrimiento en el orquestador. En Compose es el DNS embebido;
> en Cloud Run es la URL del servicio; en Kubernetes sería el Service. En los
> tres casos la aplicación solo necesita una variable de entorno, y por eso el
> adaptador acepta una URL base en vez de codificar un host.

---

## 2. Las imágenes

Ambos servicios usan **construcción multi-etapa** con tres etapas.

### 2.1 Por qué multi-etapa

Sin ella, la imagen publicada contendría el compilador, el código fuente, los
módulos descargados y las dependencias de desarrollo. Con ella, la imagen final
copia **solo lo necesario para ejecutar**.

```
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  1. builder   │   │  2. test      │   │  3. runtime   │
│               │   │               │   │               │
│  compilador   │   │  compilador   │   │  SOLO el      │
│  + fuentes    │   │  + tests      │   │  binario o    │
│  + módulos    │   │  + gcc        │   │  el código +  │
│               │   │               │   │  node_modules │
│  → binario ───┼──────────────────────▶│               │
└───────────────┘   └───────────────┘   └───────────────┘
   se descarta        se descarta          SE PUBLICA
```

### 2.2 La imagen de Go

```dockerfile
FROM golang:1.25-alpine AS builder
...
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/api ./cmd/api
```

| Opción | Qué hace | Por qué |
|---|---|---|
| `CGO_ENABLED=0` | Binario estático, sin dependencias de libc | Permite ejecutarlo sobre una imagen **sin sistema operativo** |
| `-trimpath` | Elimina rutas absolutas de compilación | Sin él, los errores filtrarían la estructura de directorios de quien compiló |
| `-s -w` | Descarta símbolos y depuración | Reduce el binario ~30 %. Contrapartida asumida: las trazas de pánico pierden detalle |
| `-X main.version` | Inyecta la versión | La imagen declara qué contiene |

### 2.3 La imagen de Node

```dockerfile
FROM node:22-slim AS deps
RUN npm ci --omit=dev
```

`npm ci` instala **exactamente** lo fijado en el lockfile, de forma
reproducible. `npm install` podría resolver versiones nuevas.

**Se usa Debian (`slim`) y no Alpine** aunque la etapa final sea distroless —que
también es Debian—. Resolver dependencias sobre musl y ejecutarlas sobre glibc
funciona con paquetes de JavaScript puro, pero **rompe el día que alguien añada
una dependencia con código nativo**.

### 2.4 Por qué distroless

La imagen final de ambos servicios es **distroless**: no contiene shell, ni
gestor de paquetes, ni utilidades del sistema. Ni siquiera un binario que liste
directorios.

Si alguien consiguiera ejecución remota dentro del contenedor, **no encontraría
herramientas con las que moverse**.

Y en el caso de Node, la decisión está respaldada por medición, no por
preferencia:

| Base | Tamaño |
|---|---|
| `node:22-alpine` | 232 MB |
| `node:22-alpine` sin npm | ~215 MB |
| **`distroless/nodejs22`** | **204 MB** |

Distroless resulta **más pequeña *y* más segura**. El suelo son los **123 MB del
propio binario de Node**, irreducibles.

### 2.5 Tamaños finales

| Imagen | Tamaño | De los cuales, código propio |
|---|---|---|
| `api-go:dev` | **20,2 MB** | ~12 MB de binario |
| `api-node:dev` | **221 MB** | ~15 MB (14,5 de `node_modules` + 0,5 de código) |

La diferencia no es mérito del código: es que un binario de Go estático no
necesita runtime, mientras que Node lleva su intérprete de 123 MB.

### 2.6 Sin privilegios

Ambas usan la etiqueta `:nonroot`, que ejecuta como **UID 65532**. Comprobado:

```bash
docker compose exec api-node /nodejs/bin/node -e "console.log(process.getuid())"
# → 65532
```

Ejecutar como root dentro del contenedor no aporta nada y amplía el impacto de
una intrusión.

---

## 3. Sondas de salud sin shell

Distroless plantea un problema práctico: **no hay shell, ni `curl`, ni `wget`**
con los que sondear el servicio.

Añadir `curl` solo para el healthcheck anularía la razón de usar distroless.

La solución idiomática: **que el propio ejecutable sepa sondearse**.

### En Go — un subcomando del binario

```go
func main() {
    if len(os.Args) > 1 && os.Args[1] == healthcheckCommand {
        os.Exit(runHealthcheck())
    }
    // ... arranque normal
}
```

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/api", "healthcheck"]
```

### En Node — un script propio

```dockerfile
HEALTHCHECK CMD ["/nodejs/bin/node", "src/healthcheck.js"]
```

Ambas sondas consultan `GET /health` contra `127.0.0.1` y terminan con código 0
o 1. Se declaran en **forma exec** (array), porque no hay shell que interprete
la forma de cadena.

**Se consulta 127.0.0.1 y no el nombre del servicio**: la sonda comprueba *este*
proceso, no la resolución de nombres de la red.

**Fallan rápido** (2 s de timeout): si el servicio no responde en ese plazo,
esperar más solo retrasa la detección.

---

## 4. docker-compose

### 4.1 Orden de arranque

```yaml
depends_on:
  api-node:
    condition: service_healthy
```

No basta con que el contenedor exista: debe estar **respondiendo**. Sin esta
condición, la API en Go aceptaría peticiones antes de que su dependencia
estuviera lista y las primeras devolverían 502.

Se ve en el arranque:

```
Container reto-2026-api-node-1  Started
Container reto-2026-api-node-1  Healthy      ← espera aquí
Container reto-2026-api-go-1    Starting
```

### 4.2 Producción también en local

```yaml
APP_ENV: production
```

La API en Go corre en modo producción **también en local**, para que la
validación de configuración del arranque se ejercite de verdad en lugar de
quedar solo para el despliegue.

Consecuencia visible: la contraseña por defecto del código (`admin123`) queda
**rechazada**, y por eso el compose define `Reto2026.Demo`.

```bash
curl -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
# → 401  (la guarda de producción está activa)
```

### 4.3 La cookie en local

```yaml
REFRESH_COOKIE_SAMESITE: Lax
REFRESH_COOKIE_SECURE: "false"
```

En local se sirve por HTTP plano sobre `localhost`, así que `Secure` haría que
el navegador descartara la cookie. En producción son `None` y `true` — ver la
sección 10.

### 4.4 Variables del compose

Todas tienen valor por defecto, así que `docker compose up --build` funciona sin
crear ningún fichero. Para sobreescribirlas:

```bash
cp .env.example .env
```

| Variable | Por defecto | Para qué |
|---|---|---|
| `API_PORT` | `8080` | Puerto del host |
| `VERSION` | `0.1.0` | Se inyecta en el binario |
| `AUTH_USERNAME` / `AUTH_PASSWORD` | `admin` / `Reto2026.Demo` | Login simulado |
| `CORS_ORIGINS` | `http://localhost:5173` | Origen del frontend |
| `GO_LOG_LEVEL` / `NODE_LOG_LEVEL` | `info` | Verbosidad |
| `STATISTICS_TIMEOUT` | `3s` | Timeout por intento |
| `STATISTICS_TOTAL_TIMEOUT` | `8s` | Techo global |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | `15m` / `168h` | Vida de los tokens |

---

## 5. Gestión de claves

### 5.1 Generación automática

**No hay ningún paso manual.** El `docker compose` incluye un servicio de
inicialización que genera el par si no existe:

```bash
docker compose up -d --build     # genera las claves y levanta todo
```

Se hizo así por dos razones. La primera, que arrancar el sistema sea **un solo
comando**: no hace falta tener `openssl` ni un shell POSIX en la máquina, solo
Docker, que ya es requisito. La segunda, y más importante, que **ninguna clave
tenga que viajar en el repositorio ni en la entrega**: cada entorno genera la
suya en el primer arranque.

Es **idempotente**: si las claves ya están, no las toca. Regenerarlas en cada
arranque invalidaría las sesiones abiertas.

```
keys/private.pem          →  firma de tokens (SOLO la API en Go)
keys/public.pem           →  verificación
keys/public/public.pem    →  copia que se monta en la API de estadísticas
```

Para el flujo sin Docker sigue existiendo el script equivalente:

```bash
sh scripts/generate-keys.sh keys
```

Produce un par RSA de 2048 bits en formato PKCS#8, que es el que espera
`golang-jwt` sin conversiones.

### 5.2 Nunca se versionan ni se hornean en la imagen

El `.gitignore` bloquea `keys/` y `*.pem`. Verificado contra el repositorio ya
publicado.

Y **no se copian dentro de las imágenes**: una clave privada horneada en una
capa queda ahí para siempre y viaja a cualquier registro donde se publique. Se
montan como volumen de solo lectura.

### 5.3 El montaje asimétrico

Aquí está la garantía de seguridad más fuerte del sistema:

```yaml
api-node:
  volumes:
    - ./keys/public:/keys:ro     # un DIRECTORIO que solo contiene la pública

api-go:
  volumes:
    - ./keys:/keys:ro            # ambas
```

**Se monta un directorio y no el fichero suelto**, y no es un detalle
estético. Si se monta un fichero que todavía no existe, Docker crea en su lugar
un **directorio** con ese nombre y el arranque falla — exactamente lo que
ocurriría en un clon recién hecho, donde las claves aún no se han generado. Con
un directorio no ocurre, y además su contenido se refleja en vivo, de modo que
las claves que produce `init-keys` aparecen dentro sin recrear el contenedor.

**El contenedor de Node no tiene forma de leer la clave privada: no está en su
sistema de ficheros.** La separación entre firmar y verificar deja de ser una
convención del código y pasa a estar **impuesta por la infraestructura**.

Comprobado:

```bash
docker compose exec api-node /nodejs/bin/node -e "
  const fs = require('fs');
  for (const p of ['/keys/public.pem','/keys/private.pem']) {
    try { fs.readFileSync(p); console.log(p, '→ LEGIBLE'); }
    catch (e) { console.log(p, '→', e.code); }
  }"
```

```
/keys/public.pem  → LEGIBLE
/keys/private.pem → ENOENT
```

### 5.4 En la nube: claves por variable de entorno

En plataformas serverless montar ficheros es incómodo. Ambos servicios aceptan
la clave **inline**:

```bash
JWT_PUBLIC_KEY="$(cat keys/public.pem)"
```

Se admite también codificada en **base64**, porque muchos gestores de secretos
no conservan los saltos de línea del PEM. La variante inline tiene prioridad
sobre la ruta.

---

## 6. Endurecimiento de los contenedores

| Medida | Qué previene | Cómo comprobarlo |
|---|---|---|
| **distroless** | Herramientas para moverse tras una intrusión | No hay `sh` en la imagen |
| **`:nonroot`** (UID 65532) | Escalada de privilegios | `process.getuid()` → `65532` |
| **`read_only: true`** | Escribir binarios o webshells en disco | Escribir en `/app` → `EROFS` |
| **`tmpfs: /tmp`** | — | Permite lo poco que sí necesita escribirse |
| **`cap_drop: ALL`** | Capacidades del kernel innecesarias | — |
| **`no-new-privileges`** | Escalada vía binarios setuid | — |
| **Sin puerto publicado** (Node) | Acceso directo saltándose el orquestador | `curl :3001` → rechazado |
| **Claves de solo lectura** | Modificación de material criptográfico | Montaje `:ro` |
| **Solo la clave pública** en Node | Emisión de tokens desde un servicio comprometido | `/keys/private.pem` → `ENOENT` |

### Comprobación del sistema de ficheros

```bash
docker compose exec api-node /nodejs/bin/node -e "
  const fs = require('fs');
  try { fs.writeFileSync('/app/intruso.js','x'); console.log('/app → ESCRIBIBLE'); }
  catch (e) { console.log('/app →', e.code); }
  try { fs.writeFileSync('/tmp/ok','x'); console.log('/tmp → escribible (tmpfs)'); }
  catch (e) { console.log('/tmp →', e.code); }"
```

```
/app → EROFS
/tmp → escribible (tmpfs)
```

`read_only: true` es viable porque **el proceso no escribe nada en disco**: los
logs van a `stdout`, que es lo que esperan los recolectores de las plataformas
de contenedores. Escribir a fichero dentro de un contenedor efímero sería
escribir a un disco que desaparece con él.

---

## 7. Operación diaria

### Arrancar

```bash
sh scripts/generate-keys.sh keys      # solo la primera vez
docker compose up -d --build
```

### Estado

```bash
docker compose ps
```

```
SERVICE    STATUS                  PORTS
api-go     Up 5 seconds (healthy)  0.0.0.0:8080->8080/tcp
api-node   Up 11 seconds (healthy) 3001/tcp
```

### Logs

```bash
docker compose logs -f                 # ambos
docker compose logs -f api-go          # uno
docker compose logs --tail 50 api-node # últimas líneas
```

Salen en **JSON estructurado**, que es lo que esperan los recolectores. Para
leerlos con comodidad:

```bash
docker compose logs api-go | grep '^api-go' | sed 's/^api-go-1  | //' | jq
```

### Ejecutar los tests

```bash
# Dentro de los contenedores, incluido el detector de carreras
docker build --target test ./api-go
docker build --target test ./api-node

# Contra el sistema en ejecución
node tests/e2e/run.mjs
```

### Reconstruir

```bash
docker compose up -d --build            # tras cambiar código
docker compose up -d --build api-go     # solo un servicio
docker compose down -v && docker compose up -d --build   # desde cero
```

### Detener

```bash
docker compose stop        # conserva los contenedores
docker compose down        # los elimina
docker compose down -v     # elimina también los volúmenes
```

> **Cuidado:** `docker compose start api-go` arranca **también sus
> dependencias**. Para probar el comportamiento con Node caído hay que detener
> Node **después** de que Go esté arriba.

### Entrar en un contenedor

Distroless **no tiene shell**, así que `docker compose exec api-go sh` falla.
Alternativas:

```bash
# Go: el binario sabe sondearse
docker compose exec api-go /app/api healthcheck

# Node: ejecutar JavaScript directamente
docker compose exec api-node /nodejs/bin/node -e "console.log(process.version)"
```

> En Git Bash sobre Windows hay que anteponer `MSYS_NO_PATHCONV=1`, o convertirá
> `/app/api` en una ruta de Windows.

---

## 8. Resolución de problemas

| Síntoma | Causa probable | Solución |
|---|---|---|
| `api-go` no arranca, sale al instante | Configuración inválida | `docker compose logs api-go` — la validación dice exactamente qué falla |
| `AUTH_PASSWORD conserva el valor por defecto` | `admin123` con `APP_ENV=production` | Definir otra en `.env` |
| `no se pudo leer la clave privada` | Faltan las claves | `sh scripts/generate-keys.sh keys` |
| `la clave publica no corresponde a la privada` | Par desparejado | Borrar `keys/` y regenerar |
| Todas las peticiones dan 502 | `api-node` caído o no sano | `docker compose ps` y revisar sus logs |
| 502 tarda ~4,8 s | Es el comportamiento correcto | 3 intentos con espera creciente |
| El frontend da error de CORS | Vite en otro puerto, o `CORS_ORIGINS` mal | Verificar el 5173 y la variable |
| El login funciona pero la sesión cae a los 15 min | La cookie no viaja | Revisar `withCredentials` y que `CORS_ORIGINS` no sea `*` |
| 401 `missing_csrf_header` | Falta `X-Refresh-Request` | El frontend la envía; con `curl` hay que añadirla |
| `docker compose exec … sh` falla | La imagen es distroless | Usar las alternativas de la sección 7 |
| El healthcheck nunca pasa a healthy | El servicio no arranca | Revisar los logs; el `start_period` es de 5 s |

### Diagnóstico rápido

```bash
# ¿Están sanos?
docker compose ps

# ¿Qué dijo cada uno al arrancar?
docker compose logs --tail 20

# ¿Responde Go desde el host?
curl -s localhost:8080/health | jq

# ¿Alcanza Go a Node por la red interna?
MSYS_NO_PATHCONV=1 docker compose exec api-go /app/api healthcheck && echo OK

# ¿El sistema completo funciona?
node tests/e2e/run.mjs
```

---

## 9. Lecciones al containerizar

Tres fallos reales que **solo aparecieron al llevar el sistema a contenedores**.

### 9.1 La etapa de test pasaba en vacío

El `.dockerignore` excluía `**/*_test.go`, así que los tests nunca entraban al
contexto de construcción:

```
ok      api-go/internal/matrix   [no test files]     ← ¡tiene 23 tests!
```

**Una etapa de verificación que no podía fallar.** Confianza falsa, que es peor
que no tener tests.

Corregido: los ficheros de test entran al contexto pero **no llegan a la imagen
final**, que solo copia el binario ya compilado.

### 9.2 La imagen base iba desatrasada

`go mod tidy` elevó `go.mod` a Go 1.25 —lo exige Fiber v3— y el toolchain local
se auto-actualizó **en silencio**. El contenedor, con `GOTOOLCHAIN=local`, falló:

```
go: go.mod requires go >= 1.25.0 (running go 1.24.13)
```

> Es exactamente el valor de compilar en contenedor: **reproducibilidad** frente
> a "en mi máquina funciona".

### 9.3 El fallo del upstream tardaba 11,9 segundos

El más instructivo.

**En local**, `localhost:3001` con nada escuchando rechaza la conexión
**instantáneamente**.

**En Docker**, un contenedor detenido **desaparece del DNS interno**. Cada
intento se consumía esperando una resolución de nombre que nunca llegaba.

El cliente acotaba cada intento, pero **no la operación completa**:

```
3 intentos × timeout  =  espera sin techo real
```

Dos correcciones:

1. **Presupuesto global** para toda la operación, distinguiendo el vencimiento
   del contexto padre (no reintentar) del timeout por intento (sí reintentar).
2. **Timeout de conexión separado** (1,5 s), que distingue *"no consigo
   conectar"* de *"conecté pero tarda"*.

Resultado medido:

| | Antes | Después |
|---|---|---|
| Latencia del 502 | 11,9 s variable | **4,8 s determinista** |
| Código | 502 | 504 si es timeout de conexión, 502 si es rechazo |

> Probar en contenedores destapó un modo de fallo que las pruebas en localhost
> ocultaban, porque una conexión rechazada es instantánea y un nombre que no
> resuelve no lo es.

---

## 10. Guía de despliegue

> Esta sección describe el procedimiento. El despliegue lo ejecuta el autor del
> proyecto.

Objetivo: **frontend en Vercel**, **ambas APIs en Google Cloud Run**.

### 10.1 Los dos puntos que causan fricción

Antes de nada, conviene tenerlos claros porque son los que rompen un despliegue
que funcionaba en local.

#### Cloud Run es público por defecto

Un servicio de Cloud Run recibe una URL `*.run.app` y **acepta tráfico de
internet**. Si se despliega `api-node` tal cual, el aislamiento de red del que
depende toda la argumentación se cae: basta abrir su URL.

Para que la garantía se sostenga hacen falta dos banderas:

```bash
--ingress=internal              # solo tráfico de la VPC o del propio proyecto
--no-allow-unauthenticated      # exige identidad de GCP además del JWT
```

Son **dos capas independientes**: red e identidad.

#### La cookie necesita SameSite=None

Vercel y GCP son dominios distintos, así que la petición de refresco es de
**origen cruzado**. El navegador solo enviará la cookie si vale `None`. Y `None`
**exige** `Secure=true`, que a su vez exige HTTPS.

```
REFRESH_COOKIE_SAMESITE=None
REFRESH_COOKIE_SECURE=true
```

Ambos son ya los valores por defecto del código cuando `APP_ENV=production`. El
compose los sobreescribe a `Lax`/`false` **solo** porque en local se sirve por
HTTP plano.

> Y por eso existe la defensa CSRF por cabecera: `SameSite=None` elimina la
> protección que `Lax` daba gratis. Ver la documentación del backend.

### 10.2 Preparación

```bash
gcloud auth login
gcloud config set project TU-PROYECTO
gcloud services enable run.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com

gcloud artifacts repositories create reto-2026 \
  --repository-format=docker --location=us-central1
```

### 10.3 Las claves como secretos

**Nunca** en variables de entorno planas ni en la imagen.

```bash
gcloud secrets create jwt-private-key --data-file=keys/private.pem
gcloud secrets create jwt-public-key  --data-file=keys/public.pem
```

### 10.4 Desplegar la API de Node (primero)

Va primero porque Go necesita su URL.

```bash
REGION=us-central1
REPO=$REGION-docker.pkg.dev/TU-PROYECTO/reto-2026

docker build -t $REPO/api-node:1.0.0 ./api-node
docker push $REPO/api-node:1.0.0

gcloud run deploy api-node \
  --image=$REPO/api-node:1.0.0 \
  --region=$REGION \
  --port=3001 \
  --ingress=internal \
  --no-allow-unauthenticated \
  --set-env-vars="NODE_ENV=production,JWT_ENABLED=true,JWT_ISSUER=reto-2026-api-go,JWT_AUDIENCE=reto-2026" \
  --set-secrets="JWT_PUBLIC_KEY=jwt-public-key:latest"
```

Nótese: solo recibe la **clave pública**. La asimetría se mantiene en la nube.

Anotar la URL que devuelve, por ejemplo
`https://api-node-xxxxx-uc.a.run.app`.

### 10.5 Desplegar la API de Go

```bash
docker build -t $REPO/api-go:1.0.0 --build-arg VERSION=1.0.0 ./api-go
docker push $REPO/api-go:1.0.0

gcloud run deploy api-go \
  --image=$REPO/api-go:1.0.0 \
  --region=$REGION \
  --port=8080 \
  --allow-unauthenticated \
  --vpc-egress=all-traffic \
  --set-env-vars="\
APP_ENV=production,\
STATISTICS_API_URL=https://api-node-xxxxx-uc.a.run.app,\
JWT_ISSUER=reto-2026-api-go,\
JWT_AUDIENCE=reto-2026,\
AUTH_USERNAME=admin,\
CORS_ORIGINS=https://TU-APP.vercel.app,\
REFRESH_COOKIE_SAMESITE=None,\
REFRESH_COOKIE_SECURE=true" \
  --set-secrets="JWT_PRIVATE_KEY=jwt-private-key:latest,JWT_PUBLIC_KEY=jwt-public-key:latest,AUTH_PASSWORD=auth-password:latest"
```

Puntos a vigilar:

- `CORS_ORIGINS` debe ser el dominio **exacto** de Vercel, con `https://` y sin
  barra final.
- `AUTH_PASSWORD` no puede ser `admin123`: el arranque fallaría, que es el
  comportamiento deseado.
- Para que Go alcance a Node con `--ingress=internal` hace falta salida por VPC
  con un conector de acceso VPC sin servidor.

#### Autenticación entre servicios

Con `--no-allow-unauthenticated` en Node, Go debe presentar un **token de
identidad de GCP** además del JWT de la aplicación. Se obtiene del servidor de
metadatos:

```
GET http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=<URL_DE_NODE>
Metadata-Flavor: Google
```

Y se envía como `Authorization: Bearer <token>`.

> **Esto requiere un cambio en el código**: el adaptador
> `statistics_client.go` usa hoy la cabecera `Authorization` para propagar el
> JWT del usuario. Habría que mover uno de los dos a otra cabecera, o usar el
> token de identidad de GCP en `Authorization` y el del usuario en una cabecera
> propia. **No está implementado**: en el compose la red interna ya provee el
> aislamiento y no hacía falta.
>
> Alternativa más sencilla para la demostración: dejar Node con
> `--ingress=internal` **sin** `--no-allow-unauthenticated`. Se conserva el
> aislamiento de red —que es la garantía principal— sin tocar código.

### 10.6 Desplegar el frontend

```bash
cd web
vercel --prod
```

Variable de entorno en Vercel:

```
VITE_API_URL = https://api-go-xxxxx-uc.a.run.app
```

Después hay que **actualizar `CORS_ORIGINS` en Go** con el dominio definitivo:

```bash
gcloud run services update api-go --region=$REGION \
  --update-env-vars="CORS_ORIGINS=https://el-dominio-real.vercel.app"
```

### 10.7 Verificación posterior

```bash
API=https://api-go-xxxxx-uc.a.run.app

# 1. Salud
curl -s $API/health | jq

# 2. Node NO debe ser alcanzable desde internet
curl -sI https://api-node-xxxxx-uc.a.run.app/health
# → 403 con --ingress=internal

# 3. La batería completa contra producción
API_URL=$API AUTH_PASSWORD='la-real' node tests/e2e/run.mjs
```

La última comprobación del test —*"la API de estadísticas NO es alcanzable desde
el host"*— fallará si el `ingress` no está bien configurado. Es justo lo que se
quiere detectar.

### 10.8 Lista de comprobación

- [ ] Claves en Secret Manager, **nunca** en variables planas ni en la imagen
- [ ] `api-node` con `--ingress=internal`
- [ ] `api-node` recibe **solo** la clave pública
- [ ] `AUTH_PASSWORD` distinta de `admin123`
- [ ] `CORS_ORIGINS` con el dominio exacto de Vercel, sin comodín
- [ ] `REFRESH_COOKIE_SAMESITE=None` y `REFRESH_COOKIE_SECURE=true`
- [ ] `VITE_API_URL` apuntando a la URL de Go
- [ ] Login, factorización y renovación probados en el dominio real
- [ ] Confirmado que la URL de Node devuelve 403 desde fuera
- [ ] Imágenes etiquetadas con versión, no con `latest`

### 10.9 Alternativas más simples

Si Cloud Run resulta laborioso para el alcance del reto:

| Plataforma | Ventaja | Inconveniente |
|---|---|---|
| **Render** | Redes privadas nativas, despliegue desde el repositorio | Menos control fino |
| **Fly.io** | Red privada `.internal` entre apps, muy directo | Requiere CLI propia |
| **Railway** | Lo más rápido de poner en marcha | Menos opciones de red |

En las tres, los dos puntos críticos son los mismos: **no exponer la API de
Node** y **configurar la cookie como cross-site**.
