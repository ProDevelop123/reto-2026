# Coding Challenge · Factorización QR

Sistema de dos APIs comunicadas por HTTP, con frontend y contenerización
completa.

- **API en Go (Fiber v3)** — recibe una matriz rectangular, calcula su
  factorización QR mediante reflexiones de Householder y orquesta la llamada al
  segundo servicio.
- **API en Node.js (Express)** — calcula estadísticas sobre las matrices
  resultantes: máximo, mínimo, promedio, suma total y detección de matriz
  diagonal.
- **Frontend (React 19)** — consume el sistema y **verifica su corrección
  matemática en el navegador**.

```
                    ┌──────────────────────────────────────────┐
  Navegador ───────▶│  API en Go · Fiber v3          :8080     │
  (React)           │                                          │
                    │  1. Autentica (JWT RS256)                │
                    │  2. A = Q·R  (Householder)               │
                    │  3. ──────────────┐                      │
                    └───────────────────┼──────────────────────┘
                                        │ HTTP · red interna
                                        ▼
                    ┌──────────────────────────────────────────┐
                    │  API en Node · Express         :3001     │
                    │  máx · mín · promedio · suma · diagonal  │
                    │  (sin puerto publicado al host)          │
                    └──────────────────────────────────────────┘

  Respuesta única:  { matrix, q, r, statistics }
```

---

## Arranque

```bash
sh scripts/generate-keys.sh keys     # par RSA, solo la primera vez
docker compose up -d --build
node tests/e2e/run.mjs               # 38 comprobaciones end-to-end
```

Frontend:

```bash
cd web && npm install && cp .env.example .env && npm run dev
```

→ http://localhost:5173 · usuario `admin` · contraseña `Reto2026.Demo`

![Workspace](web/docs/workspace.png)

---

## Sobre la ambigüedad del enunciado

El PDF dice **"rotación de la matriz"** en la lámina de arquitectura y
**"factorización QR"** en la de funcionalidad requerida. Se interpretó que manda
la segunda, por dos razones:

1. Es la que describe la funcionalidad exigible. La otra parece texto reciclado
   de un enunciado anterior sobre rotación de matrices.
2. Los términos no son ajenos. Una de las tres vías clásicas hacia la QR son las
   **rotaciones de Givens**, que anulan los elementos subdiagonales aplicando
   rotaciones planas hasta obtener exactamente `A = Q·R`.

Se implementó **Householder** y no Givens porque es el estándar para matrices
densas —lo que usan LAPACK y las bibliotecas de referencia—: anula una columna
entera por reflexión en lugar de un elemento por rotación. Givens es preferible
cuando la matriz es dispersa o ya casi triangular, que no es el caso.

Sobre *"un frontend que consuma ambas APIs"*: el frontend **sí** consume ambas,
en una sola petición. La API en Go compone su resultado con el del servicio de
estadísticas. Exponer la API de Node al navegador exigiría CORS, una segunda
ruta de autenticación y romper el aislamiento de red, a cambio de nada: el dato
ya viaja en la misma respuesta. La interfaz marca de qué servicio viene cada
bloque para que la arquitectura se vea sin leer código.

---

## Requisitos del enunciado

| Requisito | Dónde |
|---|---|
| API en Go con Fiber | [api-go/](api-go) · Fiber v3.5 |
| API en Node con Express | [api-node/](api-node) · Express 5 |
| Matriz rectangular → factorización QR | [householder.go](api-go/internal/matrix/householder.go) |
| Estadísticas sobre las matrices devueltas | [statistics.js](api-node/src/domain/statistics.js) |
| Máx · mín · promedio · suma · diagonal | Las cinco, con desglose por matriz |
| Comunicación HTTP entre las APIs | [statistics_client.go](api-go/internal/infrastructure/client/statistics_client.go) |
| Docker | 3 etapas por servicio · imágenes distroless |
| Documentación del código | Comentarios que explican el *porqué*, no el *qué* |
| **Opcional** · JWT | RS256 asimétrico, rotación y detección de reutilización |
| **Opcional** · Frontend | [web/](web) |
| **Opcional** · Tests unitarios y de integración | 29 en Go · 41 en Node · 38 end-to-end |

---

## Las decisiones que sostienen la solución

### 1. Householder, y por qué el signo importa

Tres caminos llevan a la QR: **Gram-Schmidt clásico** (intuitivo, inestable),
**rotaciones de Givens** (estables, óptimas para matrices dispersas) y
**reflexiones de Householder** (estables e incondicionales, el estándar para
densas).

La diferencia no es teórica. Midiendo `‖QᵀQ − I‖` sobre la matriz de Läuchli:

| ε | Householder | Gram-Schmidt clásico |
|---|---|---|
| 1e-6 | 4.4e-16 | 5.1e-05 |
| 1e-8 | 1.1e-15 | **5.0e-01** |
| 1e-10 | 4.4e-16 | **5.0e-01** |

Householder se mantiene en el epsilon de la máquina; Gram-Schmidt pierde la
ortogonalidad por completo. **Se puede comprobar en vivo**: el ejemplo *Mal
condicionada* del frontend es esa matriz.

La clave está en el signo: `α = −copysign(‖x‖, x₀)`. Con el contrario, un vector
casi alineado con e₁ produciría una resta entre cantidades casi iguales
—cancelación catastrófica— y el resultado tendría muy pocos dígitos
significativos. Es el error clásico al implementar el algoritmo.

### 2. Eficiencia medida, no afirmada

- **Las reflexiones nunca se materializan.** Aplicar `H = I − 2vvᵀ` a una matriz
  es `M − 2v(vᵀM)`: producto matriz-vector más actualización de rango uno.
  Nunca se construye `H`. Pasa de O(m²) a O(m·n) por reflexión.
- **Asignaciones constantes.** El benchmark da **10 allocations sea cual sea el
  tamaño** (50×50, 200×50, 200×200): cada matriz se reserva en un bloque
  contiguo y el espacio de trabajo se reutiliza entre reflexiones. El
  recolector de basura no trabaja dentro del algoritmo.
- **Norma con escalado dinámico.** Elevar al cuadrado desbordaría con ~1e200 y
  subdesbordaría con ~1e-200. Hay test para ambos extremos.
- **Suma compensada de Neumaier** en las estadísticas. Los valores de Q rondan
  1e-1 y los de R son varios órdenes mayores; una suma ingenua acumula error que
  se propaga al promedio. Test: `[1e16, 1, -1e16, 0]` da `0` con `+=` y `1` con
  la compensada.
- **Sin librerías matemáticas** en ninguna de las dos APIs. Traer `mathjs` para
  calcular un máximo añadiría peso sin aportar capacidad.

### 3. Arquitectura asimétrica, a propósito

```
api-go/   hexagonal          api-node/  capas
├─ matrix/     álgebra pura  ├─ domain/        lógica pura
├─ core/                     ├─ application/   caso de uso
│  ├─ domain/                └─ infrastructure/
│  ├─ port/    ← interfaces
│  └─ usecase/
└─ infrastructure/
```

Go usa puertos y adaptadores porque **depende de un servicio externo**.
Declararlo como interfaz (`StatisticsProvider`) invierte la dependencia, y el
beneficio es verificable: `qr_usecase_test.go` ejercita el caso de uso completo
—factorización incluida— **sin levantar la API de Node, sin abrir un socket y
sin depender de la red**.

Node no llama a nadie ni persiste nada. Montar la misma ceremonia allí sería
inconsistencia disfrazada de consistencia.

> Se aplicó el nivel de arquitectura que cada servicio justifica.

### 4. Seguridad: la infraestructura impone lo que el código promete

**RS256 asimétrico, no HS256.** Go firma con la clave privada; Node solo tiene
la pública. Es incapaz de emitir tokens, así que comprometerlo no permite
suplantar a nadie.

Y no es una convención del código: el `docker-compose.yml` monta
**exclusivamente** `keys/public.pem` en el contenedor de Node. Verificado:

```
/keys/public.pem  → LEGIBLE
/keys/private.pem → ENOENT
```

Otras medidas, todas comprobadas en la batería end-to-end:

- **Algoritmo fijado en la verificación** — cierra el ataque de confusión de
  algoritmo (`alg: none`, o HS256 usando la clave pública como secreto).
- **El tipo de token se comprueba** — ambos se firman con la misma clave, así
  que la verificación criptográfica no los distingue; el claim `tokenType`
  impide usar un refresh de vida larga como token de acceso.
- **Rotación con detección de reutilización** — cada refresco invalida el
  anterior. Si llega uno ya canjeado, hay dos copias en circulación y una es de
  un atacante: como no se puede saber cuál, se revoca la **familia entera**.
- **Refresh en cookie HttpOnly**, nunca en el cuerpo: un XSS no puede robarlo.
- **Defensa CSRF por cabecera** — en producción la cookie necesita
  `SameSite=None` (dominios distintos), lo que elimina la protección que `Lax`
  da gratis. Un formulario ajeno no puede añadir cabeceras propias, y hacerlo
  fuerza una consulta preliminar que CORS rechazará.
- **Comparación de credenciales en tiempo constante** y mensaje único para
  usuario inexistente y contraseña incorrecta: distinguirlos permitiría enumerar
  usuarios.
- **La causa interna nunca sale al cliente** — se registra en el log; la
  respuesta lleva solo el mensaje redactado para exponerse.
- **Ni el cuerpo ni `Authorization` se registran nunca.**
- **Contenedores endurecidos** — distroless, `read_only`, `cap_drop: ALL`,
  `no-new-privileges`, UID 65532. Verificado: escribir en `/app` da `EROFS`.
- **El arranque falla** si la configuración es insegura: contraseña por defecto
  en producción, `SameSite=None` sin `Secure`, o comodín en `CORS_ORIGINS`.

### 5. Topología de red

La API de Node **no publica ningún puerto**. Solo la alcanza la API en Go, por
DNS interno del orquestador. La línea `ports:` está en el compose, comentada y
explicada, por si conviene inspeccionarla de forma aislada.

> La API de Node es un backend puro, inalcanzable desde fuera. La API en Go es
> el único punto de entrada y valida el JWT antes de que la carga llegue a Node.

**En Cloud Run esa garantía no se obtiene gratis**: un servicio recibe URL
pública por defecto. Hay que configurar `--ingress=internal` y
`--no-allow-unauthenticated` con una cuenta de servicio.

### 6. Fallo explícito, no degradación silenciosa

Si la API de Node cae, la respuesta es **502 en 4.8 s deterministas** (3
intentos: 1.5 + 0.1 + 1.5 + 0.2 + 1.5).

Se consideró degradar —devolver Q y R con estadísticas vacías— y se descartó: el
contrato publicado promete estadísticas, y un 200 a medias obligaría a todos los
clientes a comprobar si llegaron.

---

## Tres bugs que solo aparecieron al containerizar

**1. La etapa de test pasaba en vacío.** El `.dockerignore` excluía
`**/*_test.go`, así que `go test` reportaba `[no test files]` en todos los
paquetes. Un pipeline que no podía fallar.

**2. La imagen base iba desatrasada.** `go mod tidy` elevó `go.mod` a Go 1.25
(lo exige Fiber v3) y el toolchain local se auto-actualizó en silencio. El
contenedor, con `GOTOOLCHAIN=local`, falló. Es el valor de compilar en
contenedor: reproducibilidad frente a *"en mi máquina funciona"*.

**3. El fallo del upstream tardaba 11.9 s.** En local, `localhost:3001` rechaza
la conexión al instante. En Docker, el contenedor parado **desaparece del DNS
interno** y cada intento se consumía esperando resolución. El cliente acotaba
cada intento pero no la operación completa.

Dos correcciones: un **presupuesto global** (distinguiendo el vencimiento del
contexto padre del timeout por intento) y un **timeout de conexión separado**
que distingue *"no consigo conectar"* de *"conecté pero tarda"*. Resultado: de
11.9 s variables a **4.8 s deterministas**.

> Probar en contenedores destapó un modo de fallo que las pruebas en localhost
> ocultaban, porque una conexión rechazada es instantánea y un nombre que no
> resuelve no lo es.

---

## Verificación

```bash
cd api-go   && go test ./... -cover        # 29 tests · matrix 97.0% · usecase 82.5%
cd api-node && npm test                    # 41 tests · 87.6% statements
node tests/e2e/run.mjs                     # 38 comprobaciones sobre el stack real

docker build --target test ./api-go        # go test -race dentro del contenedor
docker build --target test ./api-node      # jest dentro del contenedor
```

El detector de carreras necesita cgo, no disponible en Windows: por eso el
Dockerfile tiene una etapa `test` con `gcc` que lo ejecuta en Linux.

**El frontend verifica el backend.** El panel de verificación recalcula `Q·R` y
`QᵀQ` en el navegador y muestra los residuos. La corrección del algoritmo no es
una afirmación del servidor.

---

## Estructura

```
reto-2026/
├─ api-go/           Go 1.25 · Fiber v3 · hexagonal
│  ├─ internal/matrix/       álgebra pura, cero dependencias
│  ├─ internal/core/         domain · port · usecase
│  └─ internal/infrastructure/
├─ api-node/         Node 22 · Express 5 · capas
├─ web/              React 19 · Vite · shadcn/ui
├─ keys/             par RSA (no versionado)
├─ scripts/          generación de claves
├─ tests/e2e/        prueba de integración sin dependencias
└─ docker-compose.yml
```

Cada servicio tiene su propio README con el detalle:
[api-go](api-go/README.md) · [api-node](api-node/README.md) ·
[web](web/README.md)

---

## Contrato

`POST /api/v1/qr` — requiere `Authorization: Bearer <token>`

```json
{ "matrix": [[12,-51,4],[6,167,-68],[-4,24,-41]], "mode": "reduced" }
```

```jsonc
{
  "success": true,
  "data": {
    "matrix": { "name": "A", "rows": 3, "columns": 3, "data": [[…]] },
    "q":      { "name": "Q", "rows": 3, "columns": 3, "data": [[…]] },
    "r":      { "name": "R", "rows": 3, "columns": 3, "data": [[…]] },
    "statistics": {
      "global": {
        "max": 70, "min": -175, "average": -5.2178, "sum": -93.92,
        "isAnyDiagonal": false, "diagonalMatrices": []
      },
      "perMatrix": [ … ]
    }
  },
  "metadata": { "mode": "reduced", "reflectors": 3, "tolerance": 1e-9, "durationMs": 5.25 }
}
```

Ambas APIs comparten el mismo sobre pese a estar en lenguajes distintos: **el
contrato pertenece al sistema, no al framework**. El frontend puede escribir un
único interceptor que entienda las respuestas de los dos servicios.

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `BAD_REQUEST` | El cuerpo no es JSON válido |
| 401 | `UNAUTHORIZED` | Token ausente, mal formado, expirado, de tipo incorrecto, o credenciales inválidas |
| 404 | `NOT_FOUND` | Ruta inexistente fuera de `/api/v1` |
| 422 | `VALIDATION_ERROR` | Matriz no rectangular, vacía o con valores no finitos |
| 502 | `UPSTREAM_ERROR` | La API de estadísticas falló |
| 504 | `UPSTREAM_TIMEOUT` | La API de estadísticas no respondió a tiempo |

---

## Nota sobre `metadata.reflectors`

Es el número de reflexiones aplicadas, normalmente `mín(m,n)`. **No es un
estimador del rango.** Sobre una matriz deficiente de rango pero sin ceros
exactos, la subcolumna residual queda en ~1e-16 en lugar de en cero, así que se
aplica una reflexión más sobre ruido de redondeo —el mismo comportamiento de
LAPACK—. Determinar el rango numérico de forma fiable exige **QR con pivoteo por
columnas**, que es un algoritmo distinto. El contador solo baja cuando una
columna es exactamente nula.
