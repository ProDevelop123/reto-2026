# Documentación del Backend

Los dos servicios que forman el backend: la API en Go que factoriza matrices y
la API en Node que calcula estadísticas.

**Índice**

1. [Cómo funciona el sistema](#1-cómo-funciona-el-sistema)
2. [API en Go](#2-api-en-go-fiber-v3)
3. [El algoritmo de Householder, paso a paso](#3-el-algoritmo-de-householder-paso-a-paso)
4. [Autenticación](#4-autenticación)
5. [API en Node](#5-api-en-node-express-5)
6. [El contrato compartido](#6-el-contrato-compartido)
7. [Referencia de endpoints](#7-referencia-de-endpoints)
8. [Variables de entorno](#8-variables-de-entorno)
9. [Tests](#9-tests)

---

## 1. Cómo funciona el sistema

Cuando alguien envía una matriz, pasan cinco cosas:

```
1. El cliente hace POST /api/v1/qr a la API en Go, con su token.

2. Go valida el token (firma, emisor, audiencia, tipo y vigencia).

3. Go calcula la factorización QR:  A = Q · R

4. Go llama por HTTP a la API en Node y le envía Q y R.
   Node calcula máximo, mínimo, promedio, suma y si alguna es diagonal.

5. Go compone la respuesta con todo junto y la devuelve.
```

**Punto clave:** el cliente hace **una sola llamada**. La API en Go es el
orquestador; la de Node ni siquiera está expuesta al exterior.

### Por qué dos servicios y no uno

El enunciado del reto lo pide así. Dicho eso, la división es limpia:

| Servicio | Responsabilidad | ¿Llama a alguien? |
|---|---|---|
| **Go** | Álgebra lineal + orquestación + autenticación | Sí, a Node |
| **Node** | Estadísticas sobre matrices | No, a nadie |

La API de Node **no sabe qué es una factorización QR**. Recibe una lista de
matrices y devuelve métricas. Esa ignorancia es deliberada: la hace reutilizable
y mantiene toda la responsabilidad del álgebra del lado de Go.

---

## 2. API en Go (Fiber v3)

### 2.1 Estructura de carpetas

```
api-go/
├─ cmd/api/
│  ├─ main.go              arranque y conexión de dependencias
│  └─ healthcheck.go       sonda de salud (subcomando del binario)
│
├─ internal/
│  ├─ matrix/              ÁLGEBRA PURA — sin dependencias externas
│  │  ├─ matrix.go         tipo Matrix, multiplicar, trasponer, validar
│  │  └─ householder.go    la factorización QR
│  │
│  ├─ core/                EL NÚCLEO — no conoce HTTP ni Fiber
│  │  ├─ domain/           conceptos del negocio
│  │  ├─ port/             interfaces que el núcleo necesita
│  │  └─ usecase/          orquestación
│  │
│  ├─ infrastructure/      TODO LO REEMPLAZABLE
│  │  ├─ http/             Fiber: router, handlers, DTOs, middlewares
│  │  ├─ jwt/              firma y verificación RS256
│  │  ├─ client/           cliente HTTP hacia la API de Node
│  │  ├─ persistence/      almacén de sesiones en memoria
│  │  ├─ logger/           logs estructurados
│  │  └─ system/           reloj y generador de identificadores
│  │
│  └─ config/              lectura y validación de variables de entorno
│
└─ pkg/apperror/           error de aplicación con código HTTP
```

### 2.2 Arquitectura hexagonal, explicada sin jerga

La idea es simple: **el código importante no debe depender de detalles
cambiantes**.

Sin esta arquitectura, el caso de uso llamaría directamente al cliente HTTP:

```
usecase  ──depende de──▶  cliente HTTP  ──▶  API de Node
```

Para testear el caso de uso habría que levantar la API de Node. Con la
arquitectura hexagonal, se le da la vuelta:

```
usecase  ──depende de──▶  [ interfaz StatisticsProvider ]
                                    ▲
                                    │ la implementa
                          cliente HTTP  ──▶  API de Node
```

Ahora el caso de uso solo conoce una **interfaz**. En los tests se le pasa un
doble en memoria y todo funciona sin red:

```go
// internal/core/port/ports.go
type StatisticsProvider interface {
    Analyze(ctx context.Context, req domain.StatisticsRequest) (domain.StatisticsResult, error)
}
```

El beneficio es medible: `qr_usecase_test.go` ejercita el caso de uso completo
—factorización incluida— **sin levantar Node, sin abrir un socket y sin red**.
Corre en microsegundos y no puede fallar de forma intermitente.

**¿Y si mañana Node se sustituye por gRPC o una cola?** Se escribe un adaptador
nuevo que implemente la misma interfaz. El núcleo no se toca.

### 2.3 Dónde se conecta todo

Hay **un único sitio** donde se decide qué implementación concreta cumple cada
interfaz: `cmd/api/main.go`.

```go
clock      := system.NewClock()             // reloj real
ids        := system.NewIDGenerator()       // crypto/rand
tokens, _  := appjwt.New(cfg.JWT, clock)    // RS256
sessions   := memory.NewSessionRepository() // ← cambiar por Redis aquí
statistics := client.New(cfg.Statistics)    // ← cambiar por gRPC aquí

qrUseCase := usecase.NewQRUseCase(statistics)
```

Sustituir el almacén en memoria por Redis es **cambiar una línea**.

### 2.4 El cliente hacia la API de Node

`internal/infrastructure/client/statistics_client.go` es el único fichero que
sabe que las estadísticas las calcula un proceso remoto.

Tiene cuatro protecciones, y cada una responde a un fallo distinto:

| Protección | Valor | Qué previene |
|---|---|---|
| Timeout de conexión | 1,5 s | Que un destino inexistente consuma el timeout completo |
| Timeout por intento | 3 s | Que una respuesta lenta retenga goroutines indefinidamente |
| Presupuesto global | 8 s | Que los reintentos multipliquen la espera del cliente |
| Reintentos | 2, con espera creciente | Fallos transitorios de red o 5xx |

**Solo se reintenta lo que puede tener éxito al repetirse**: errores de red y
respuestas 5xx. Un 4xx significa que la petición es incorrecta y repetirla es
inútil.

Por qué hacen falta los dos timeouts:

```
Sin presupuesto global:  3s + 0,1s + 3s + 0,2s + 3s  =  9,3 s de espera
Con presupuesto global:  se corta a los 8 s
Con timeout de conexión: 1,5 + 0,1 + 1,5 + 0,2 + 1,5 = 4,8 s  ← real
```

Este ajuste salió de una medición: con Node caído el fallo tardaba **11,9
segundos**. Ver la sección de infraestructura para el detalle.

### 2.5 Manejo de errores

Ninguna capa construye respuestas de error. Todas devuelven un `AppError`, y
**un único middleware** lo convierte en respuesta HTTP.

```go
// desde cualquier capa
return apperror.Validation("La matriz no es rectangular.").
    WithDetails(map[string]any{"row": 1, "expected": 2, "received": 1})
```

Regla importante: **la causa interna nunca sale al cliente**. Se registra en el
log, pero la respuesta lleva solo el mensaje redactado para exponerse.

```
LOG:       cause: dial tcp 127.0.0.1:3001: connection refused
RESPUESTA: { "code": "UPSTREAM_ERROR", "message": "No se pudo contactar…" }
```

Sin esa separación, un error filtraría rutas de fichero, nombres de host
internos o versiones de dependencias.

---

## 3. El algoritmo de Householder, paso a paso

### 3.1 Qué es una factorización QR

Descomponer una matriz `A` en dos factores:

```
A  =  Q · R

Q  →  columnas ortonormales (perpendiculares entre sí y de longitud 1)
R  →  triangular superior (todo cero por debajo de la diagonal)
```

Se usa para resolver mínimos cuadrados, calcular autovalores y estabilizar
sistemas mal condicionados.

### 3.2 Tres caminos, y por qué se eligió uno

| Método | Idea | Problema |
|---|---|---|
| **Gram-Schmidt clásico** | Ortogonalizar columna a columna | **Numéricamente inestable** |
| **Rotaciones de Givens** | Anular un elemento por rotación | Hace de más en matrices densas |
| **Reflexiones de Householder** | Anular una columna entera por reflexión | Ninguno relevante aquí |

Se eligió **Householder**: es lo que usan LAPACK y las bibliotecas de
referencia para matrices densas.

> **Dato para la ambigüedad del enunciado:** el PDF dice "rotación" en una
> lámina y "factorización QR" en la siguiente. Givens llega a la QR aplicando
> **rotaciones planas**, así que ambos términos están conectados. Se implementó
> Householder por eficiencia en matrices densas.

### 3.3 La idea geométrica

Una **reflexión de Householder** es un espejo. Se elige el espejo de forma que,
al reflejar una columna, todos sus elementos por debajo de la diagonal caigan a
cero.

```
Antes                Después de reflejar
┌   ┐                ┌     ┐
│ 3 │                │ -5  │
│ 4 │   ──reflejo──▶ │  0  │
└   ┘                └     ┘
```

Aplicando una reflexión por columna, `R` va quedando triangular:

```
Paso 0        Paso 1        Paso 2        Paso 3
■ ■ ■         ■ ■ ■         ■ ■ ■         ■ ■ ■
■ ■ ■   ──▶   0 ■ ■   ──▶   0 ■ ■   ──▶   0 ■ ■
■ ■ ■         0 ■ ■         0 0 ■         0 0 ■
```

### 3.4 El cálculo, en cinco pasos

Para cada columna `j`:

```
1. Tomar la subcolumna x = A[j:, j]  (de la diagonal hacia abajo)

2. Calcular su norma:  ‖x‖

3. Elegir el objetivo:  α = −signo(x₀) · ‖x‖        ← EL PASO CRÍTICO

4. Construir el vector del espejo:  v = x − α·e₁,  normalizado

5. Aplicar la reflexión H = I − 2vvᵀ  a R  y acumularla en Q
```

### 3.5 Por qué el signo del paso 3 lo es todo

Si se eligiera `α` con el **mismo** signo que `x₀`, y el vector ya estuviera
casi alineado con el eje, la resta del paso 4 restaría dos cantidades **casi
iguales**:

```
x₀ = 5.0000001
α  = 5.0000000        ← mismo signo: MAL
v₀ = 0.0000001        ← quedan poquísimos dígitos significativos
```

Eso se llama **cancelación catastrófica** y arruina la precisión. Con el signo
opuesto, las magnitudes se suman:

```
x₀ =  5.0000001
α  = -5.0000000       ← signo opuesto: BIEN
v₀ = 10.0000001       ← precisión intacta
```

Es el error clásico al implementar Householder, y la razón de que el código use
`math.Copysign`.

### 3.6 Las reflexiones nunca se construyen

Aplicar `H = I − 2vvᵀ` **no** requiere crear la matriz `H` de m×m:

```
H · M  =  (I − 2vvᵀ) · M  =  M − 2v(vᵀM)
                              └──────────┘
                              producto matriz-vector
                              + actualización de rango uno
```

El coste baja de **O(m²)** a **O(m·n)** por reflexión, y no se reserva memoria
extra.

### 3.7 Estabilidad: la prueba

Midiendo `‖QᵀQ − I‖` (cuánto se aleja Q de ser ortogonal) sobre la matriz de
Läuchli, cuyas columnas son casi idénticas:

| ε | Householder | Gram-Schmidt clásico |
|---|---|---|
| 1e-6 | 4,4e-16 | 5,1e-05 |
| 1e-8 | 1,1e-15 | **5,0e-01** |
| 1e-10 | 4,4e-16 | **5,0e-01** |

Householder se mantiene en el epsilon de la máquina (~2,2e-16). Gram-Schmidt se
desploma a 0,5: las columnas quedan a unos 60° en vez de 90°, es decir, **dejan
de ser ortogonales**.

Se puede comprobar en vivo: el ejemplo *Mal condicionada* del frontend es esa
matriz.

### 3.8 Modos: reducida y completa

Para una matriz `A` de m×n, con `k = mín(m, n)`:

| Modo | Q | R | Cuándo |
|---|---|---|---|
| `reduced` (por defecto) | m×k | k×n | Uso práctico |
| `complete` | m×m | m×n | Definición formal |

Por qué la reducida es el defecto: para una matriz de **1000×3**, la Q completa
sería de 1000×1000 — **un millón de valores** frente a 3000.

### 3.9 Casos límite cubiertos

| Caso | Comportamiento |
|---|---|
| Matriz nula | 0 reflexiones, Q = identidad recortada |
| Columna exactamente nula | Se omite esa reflexión |
| Rango deficiente | `R[j][j] ≈ 0`, sin división por cero |
| Valores ~1e200 | Norma con escalado dinámico: no desborda |
| Valores ~1e-200 | Norma con escalado dinámico: no subdesborda |
| m < n (horizontal) | Funciona; R queda trapezoidal |
| 1×1 | Funciona |

### 3.10 Los ceros de R son exactos

Por debajo de la diagonal, `R` se pone a **cero exacto**, no a los residuos de
~1e-17 que deja el cálculo. Son ceros **estructurales** —el algoritmo los
produce por construcción— y devolver ruido de redondeo en su lugar sería
engañoso.

### 3.11 Rendimiento medido

```
BenchmarkDecompose/50x50      222 µs      87 KB    10 allocs/op
BenchmarkDecompose/200x50     3,4 ms     531 KB    10 allocs/op
BenchmarkDecompose/200x200   10,1 ms     1,3 MB    10 allocs/op
```

**10 asignaciones de memoria sea cual sea el tamaño.** Salen de dos decisiones:

1. Cada matriz se reserva en **un único bloque contiguo**, no una asignación por
   fila.
2. El vector `v` y el de proyecciones se reservan **una vez fuera del bucle** y
   se reutilizan en cada reflexión.

El recolector de basura no trabaja dentro del algoritmo.

### 3.12 Sobre `metadata.reflectors`

Es cuántas reflexiones se aplicaron, normalmente `mín(m,n)`.

> **No es un estimador del rango.** Sobre una matriz deficiente de rango pero
> sin ceros exactos, la subcolumna residual queda en ~1e-16 en vez de en cero, y
> se aplica una reflexión más sobre ruido de redondeo. Es el mismo
> comportamiento de LAPACK. Determinar el rango de forma fiable exige **QR con
> pivoteo por columnas**, que es otro algoritmo.

Solo baja cuando una columna es **exactamente** nula.

---

## 4. Autenticación

### 4.1 Por qué RS256 y no HS256

| | HS256 (simétrico) | RS256 (asimétrico) ✅ |
|---|---|---|
| Clave | Un secreto compartido | Par privada/pública |
| Quién firma | **Ambos servicios** | Solo Go |
| Quién verifica | Ambos | Ambos |
| Si comprometen Node | **Puede emitir tokens** | **No puede emitir nada** |

Go firma con la clave privada; Node solo tiene la pública. Es **incapaz** de
emitir tokens.

Y no es una convención del código: el `docker-compose.yml` monta
**exclusivamente** la clave pública en el contenedor de Node. Comprobado:

```
/keys/public.pem  → LEGIBLE
/keys/private.pem → ENOENT (no existe en ese contenedor)
```

### 4.2 Los dos tokens

| | Access token | Refresh token |
|---|---|---|
| Vida | 15 minutos | 7 días |
| Viaja en | Cabecera `Authorization` | Cookie `HttpOnly` |
| Lo lee JavaScript | Sí | **No** |
| ¿Revocable? | No | **Sí** |

El access token es de vida corta **porque no se puede revocar**: un JWT es
autocontenido y vale hasta que expira. El refresh token sí se puede revocar
porque hay un registro de sesiones contra el que contrastarlo.

### 4.3 Flujo de login

```
Cliente                          API en Go
  │                                  │
  │── POST /auth/login ─────────────▶│
  │   { username, password }         │
  │                                  │ compara en tiempo constante
  │                                  │ crea familia + sesión
  │                                  │ firma ambos tokens
  │                                  │
  │◀── 200 ──────────────────────────│
  │    body:   { accessToken, … }    │
  │    cookie: refresh_token         │  HttpOnly · Secure · SameSite
```

El refresh token **no aparece en el cuerpo**. Si apareciera, el frontend
tendría que guardarlo en algún sitio accesible desde JavaScript y un XSS
bastaría para robarlo.

### 4.4 Rotación y detección de reutilización

Cada refresco **invalida el token anterior** y emite uno nuevo:

```
login    →  token A
refresh(A)  →  token B     A queda marcado como "usado"
refresh(B)  →  token C     B queda marcado como "usado"
```

Si llega **A otra vez**, algo va mal: el usuario legítimo ya lo canjeó, así que
quien lo presenta ahora tiene una copia.

```
refresh(A)  →  401  +  SE REVOCA LA FAMILIA ENTERA
```

Se revocan **todos** los tokens de esa cadena, incluido el legítimo. Es
deliberado: no se puede saber cuál de las dos copias es del atacante, así que se
invalidan ambas y el usuario vuelve a autenticarse.

La **familia** (`familyId`) agrupa todos los tokens descendientes de un mismo
login, y es lo que permite cortar la cadena de golpe.

### 4.5 Defensa contra CSRF

**El problema.** En producción el frontend está en Vercel y la API en GCP:
dominios distintos. Para que la cookie viaje necesita `SameSite=None`, y eso
elimina la protección que `SameSite=Lax` da gratis en local.

Un sitio cualquiera podría hacer que el navegador de la víctima llame a
`/auth/refresh` con su cookie adjunta. CORS **no lo impide**: CORS decide quién
puede *leer* la respuesta, no quién puede *enviar* la petición.

No permitiría robar la sesión, pero sí forzar una rotación y con ella disparar
la detección de reutilización: **cierre de sesión forzado** del usuario
legítimo.

**La defensa.** Exigir una cabecera propia en las rutas cuya credencial es la
cookie:

```
X-Refresh-Request: 1
```

Un formulario HTML o una etiqueta `<img>` —los vectores clásicos de CSRF— **no
pueden añadir cabeceras**. Hacerlo convierte la petición en "no simple" según
CORS, lo que obliga al navegador a una consulta preliminar `OPTIONS` que la
política de orígenes rechazará.

El **valor** de la cabecera es irrelevante: lo que protege es que exista, porque
solo puede ponerla código que ya superó el control de origen.

| Ruta | ¿Exige la cabecera? | Por qué |
|---|---|---|
| `/auth/login` | No | Se autentica con el cuerpo, no con cookie |
| `/auth/refresh` | **Sí** | Se autentica con cookie |
| `/auth/logout` | **Sí** | Se autentica con cookie |

### 4.6 Otras medidas

- **Algoritmo fijado en la verificación.** Se exige RS256 explícitamente. Sin
  esto, un atacante podría cambiar la cabecera `alg` a `none`, o a HS256 usando
  la clave pública como secreto.
- **El tipo de token se comprueba.** Ambos se firman con la misma clave, así que
  la verificación criptográfica no los distingue. El claim `tokenType` impide
  usar un refresh de vida larga como token de acceso.
- **Comparación en tiempo constante.** Una comparación normal de cadenas aborta
  en el primer byte distinto, lo que filtra por temporización cuántos caracteres
  iniciales son correctos.
- **Mensaje único.** "Usuario o contraseña incorrectos" para ambos casos:
  distinguirlos permitiría enumerar usuarios válidos.
- **El arranque falla** si la configuración es insegura: contraseña por defecto
  en producción, `SameSite=None` sin `Secure`, o comodín en `CORS_ORIGINS`.

### 4.7 El almacén de sesiones

Está **en memoria**, con `sync.RWMutex`. Limitaciones asumidas y documentadas:

- Un reinicio invalida todas las sesiones (los usuarios vuelven a autenticarse;
  no se pierde ningún dato de negocio).
- Con varias réplicas, el refresco solo funcionaría contra la instancia que
  emitió el token.

Es aceptable porque el reto no exige persistencia. Y lo importante es que la
decisión es **reversible sin tocar el núcleo**: basta escribir un adaptador
contra Redis que implemente `port.SessionRepository`.

Hay además una limpieza periódica de sesiones caducadas: sin ella el mapa
crecería indefinidamente.

---

## 5. API en Node (Express 5)

### 5.1 Estructura

```
api-node/src/
├─ domain/
│  ├─ statistics.js        LÓGICA PURA — el cálculo
│  └─ errors.js
├─ application/
│  └─ analyze-matrices.usecase.js
├─ infrastructure/
│  ├─ http/
│  │  ├─ routes/           rutas
│  │  ├─ controllers/      adaptación HTTP ↔ caso de uso
│  │  ├─ middlewares/      auth, validación, errores
│  │  └─ schemas/          validación con zod
│  └─ jwt/                 SOLO verificación
├─ config/
└─ shared/
```

### 5.2 Por qué aquí no hay arquitectura hexagonal

**Porque este servicio no depende de nada externo.** No llama a ningún servicio,
no persiste nada, no tiene dependencias que invertir.

Montar puertos y adaptadores aquí sería ceremonia sin justificación.

> Se aplicó el nivel de arquitectura que cada servicio justifica. Aplicar la
> misma a ambos sería inconsistencia disfrazada de consistencia.

### 5.3 Las cinco métricas, en una sola pasada

El enunciado pide máximo, mínimo, promedio, suma total y detección de matriz
diagonal. Las cinco se calculan **recorriendo la matriz una única vez**, sin
crear arrays intermedios:

```js
for (let i = 0; i < rows; i += 1) {
  for (let j = 0; j < columns; j += 1) {
    const value = row[j];

    if (value > max) max = value;      // máximo
    if (value < min) min = value;      // mínimo
    sum.add(value);                    // suma (compensada)

    if (diagonal && i !== j && Math.abs(value) > tolerance) {
      diagonal = false;                // ¿es diagonal?
    }
  }
}
// promedio = suma / cantidad
```

Coste: **O(m·n)**. Sin `flatten`, sin `map`, sin arrays temporales.

### 5.4 Sin librerías matemáticas

No se usa `mathjs` ni equivalentes. Tres razones:

1. **La operación no lo justifica.** Calcular un máximo es una comparación.
2. **Peso.** `mathjs` arrastra `decimal.js` y `fraction.js`: cientos de KB para
   sustituir un `if`.
3. **Lo que proyecta.** `math.max(math.flatten(m))` en lugar de un bucle de una
   pasada no demuestra criterio, demuestra dependencia.

### 5.5 Suma compensada de Neumaier

Los valores de `Q` rondan 1e-1 y los de `R` pueden ser varios órdenes mayores.
Una suma ingenua acumula error de redondeo que se propaga al promedio.

Ejemplo real, cubierto por un test:

```js
[1e16, 1, -1e16, 0]

suma ingenua (+=):     1e16 + 1 = 1e16   (el 1 se pierde)
                       1e16 - 1e16 = 0   ← RESULTADO: 0

suma compensada:                         ← RESULTADO: 1  ✅
```

La compensación recupera el error perdido en cada operación:

```js
add(value) {
  const next = this.#sum + value;

  if (Math.abs(this.#sum) >= Math.abs(value)) {
    this.#compensation += this.#sum - next + value;
  } else {
    this.#compensation += value - next + this.#sum;
  }

  this.#sum = next;
}
// El total es sum + compensation
```

Cuesta un par de operaciones por elemento.

### 5.6 Detección de matriz diagonal

Una matriz es diagonal si **todo lo que está fuera de la diagonal principal es
cero**: `a[i][j] = 0` para todo `i ≠ j`.

**El problema con los flotantes.** Las matrices vienen de una factorización QR,
donde los ceros teóricos son en realidad residuos de ~1e-16:

```
Teoría:              Realidad:
┌ 4   0 ┐            ┌ 4.0        1.2e-17 ┐
│ 0   9 │            │ 3.4e-16    9.0     │
└       ┘            └                    ┘
```

Comparar contra cero exacto haría que **ninguna matriz real** se detectara como
diagonal. Por eso hay una **tolerancia**, configurable, con `1e-9` por defecto.

```
|a[i][j]| ≤ tolerancia   →  cuenta como cero
```

**Definición adoptada:** se usa la general (`a[i][j] = 0` para `i ≠ j`), que:

- Admite matrices **rectangulares** diagonales, como la Σ de una descomposición
  SVD.
- Considera diagonal la **matriz nula**, en línea con el álgebra lineal estándar.
- **No** exige que la diagonal principal sea distinta de cero.

### 5.7 Dos niveles de resultado

```jsonc
{
  "global": {            // ← lo que pide literalmente el enunciado
    "max": 5, "min": 0, "average": 1.75, "sum": 14,
    "isAnyDiagonal": true,
    "diagonalMatrices": ["Q"]        // ← además, CUÁL
  },
  "perMatrix": [         // ← el desglose
    { "name": "Q", "rows": 2, "columns": 2, "isDiagonal": true,  … },
    { "name": "R", "rows": 2, "columns": 2, "isDiagonal": false, … }
  ]
}
```

El desglose no cuesta ninguna pasada extra —el máximo global es el máximo de los
máximos— y saber *cuál* matriz es diagonal es más útil que saber que alguna lo
es.

### 5.8 Validación con zod

La validación vive en el borde HTTP para que el dominio pueda dar por buenos los
datos que recibe.

| Regla | Por qué |
|---|---|
| 1 a 16 matrices | Un payload ilimitado agota memoria |
| Filas no vacías | El cálculo presupone datos |
| **Todas las filas de igual longitud** | Una fila corta provocaría un fallo de índice |
| Números finitos | Un `Infinity` envenenaría máximo y promedio **en silencio** |
| Tolerancia ≥ 0 | Una negativa no tiene sentido |

Dato: en **zod 4**, `z.number()` ya rechaza `NaN` e `Infinity` de fábrica.

Los errores señalan la fila exacta:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "details": {
      "properties": { "matrices": { "items": [ { "properties": { "data": {
        "items": [ null, { "errors": [
          "La matriz no es rectangular: la fila 1 tiene 1 elemento(s) y se esperaban 2."
        ]}]
      }}}]}}
    }
  }
}
```

### 5.9 Este servicio solo verifica tokens

Conoce **únicamente la clave pública**. No puede emitir nada.

Comprueba: firma, algoritmo (RS256 fijado), emisor, audiencia, vigencia y que
`tokenType` sea `access`.

**Autentica antes de validar.** El orden importa: validar primero regalaría
capacidad de cómputo a peticiones anónimas.

### 5.10 Los logs censuran credenciales

`pino-http` registraba la cabecera `Authorization` completa. Cualquiera con
acceso a los logs habría tenido tokens válidos.

```js
redact: {
  paths: ["req.headers.authorization", "req.headers.cookie", "res.headers[\"set-cookie\"]"],
  censor: "[REDACTED]",
}
```

Verificado: ahora aparece `"authorization":"[REDACTED]"`.

---

## 6. El contrato compartido

Ambas APIs usan **el mismo sobre**, pese a estar en lenguajes distintos:

```jsonc
// Éxito
{ "success": true, "data": { … }, "metadata": { … } }

// Error
{ "success": false, "error": { "code": "…", "message": "…", "details": { … } } }
```

> El contrato pertenece al sistema, no al framework.

Consecuencia práctica: el frontend escribe **un único traductor de errores** que
entiende las respuestas de los dos servicios.

### Códigos de error

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `BAD_REQUEST` | El cuerpo no es JSON válido |
| 401 | `UNAUTHORIZED` | Token ausente, mal formado, expirado, de tipo incorrecto, o credenciales inválidas |
| 404 | `NOT_FOUND` | Ruta inexistente fuera de `/api/v1` |
| 422 | `VALIDATION_ERROR` | Matriz no rectangular, vacía, con valores no finitos, o parámetros fuera de rango |
| 502 | `UPSTREAM_ERROR` | La API de estadísticas falló o no se pudo contactar |
| 504 | `UPSTREAM_TIMEOUT` | La API de estadísticas no respondió a tiempo |
| 500 | `INTERNAL_ERROR` | Fallo no previsto |

En los 401, `error.details.reason` distingue el motivo, para que el frontend
decida si renovar el token o volver a autenticar:

`missing_header` · `malformed_header` · `token_invalid` · `token_expired` ·
`wrong_token_type` · `missing_refresh_cookie` · `missing_csrf_header`

> **Nota:** una ruta inexistente **dentro** de `/api/v1` responde `401`, no
> `404`. Es deliberado: no se revela qué rutas existen a quien no se ha
> autenticado. Fuera de ese prefijo, el 404 funciona con normalidad.

---

## 7. Referencia de endpoints

### API en Go — `http://localhost:8080`

#### `POST /api/v1/auth/login`

```json
{ "username": "admin", "password": "Reto2026.Demo" }
```

`200`:

```json
{
  "success": true,
  "data": {
    "accessToken": "eyJhbGciOiJSUzI1NiIs…",
    "tokenType": "Bearer",
    "expiresIn": 899,
    "expiresAt": "2026-08-20T18:00:00Z",
    "username": "admin"
  }
}
```

Además envía la cookie `refresh_token` (`HttpOnly`, `Path=/api/v1/auth`).

#### `POST /api/v1/auth/refresh`

Requiere la cookie y la cabecera `X-Refresh-Request`. Sin cuerpo. Devuelve lo
mismo que el login y **rota** la cookie.

#### `POST /api/v1/auth/logout`

Requiere la cabecera `X-Refresh-Request`. Revoca la familia completa. Responde
`200` aunque no haya cookie: el efecto deseado ya se cumple.

#### `POST /api/v1/qr` — requiere `Authorization: Bearer <token>`

```json
{ "matrix": [[12,-51,4],[6,167,-68],[-4,24,-41]], "mode": "reduced", "tolerance": 1e-9 }
```

| Campo | Tipo | Oblig. | Descripción |
|---|---|---|---|
| `matrix` | `number[][]` | Sí | Rectangular, no vacía, hasta 512×512 |
| `mode` | `"reduced"` \| `"complete"` | No | Por defecto `reduced` |
| `tolerance` | `number ≥ 0` | No | Umbral de cero para la detección de diagonal |

`200`:

```jsonc
{
  "success": true,
  "data": {
    "matrix": { "name": "A", "rows": 3, "columns": 3, "data": [[12,-51,4], …] },
    "q":      { "name": "Q", "rows": 3, "columns": 3, "data": [[-0.857, …], …] },
    "r":      { "name": "R", "rows": 3, "columns": 3, "data": [[-14,-21,14],[0,-175,70],[0,0,35]] },
    "statistics": {
      "global": {
        "matrices": 2, "count": 18,
        "max": 70, "min": -175, "average": -5.2178, "sum": -93.92,
        "isAnyDiagonal": false, "diagonalMatrices": []
      },
      "perMatrix": [ … ]
    }
  },
  "metadata": {
    "mode": "reduced",
    "reflectors": 3,
    "tolerance": 1e-9,
    "durationMs": 5.25,
    "computedAt": "2026-08-20T17:09:28.437Z"
  }
}
```

#### `GET /health`

Sin autenticar. La consultan Docker y el balanceador.

### API en Node — `http://api-node:3001` *(solo red interna)*

#### `POST /api/v1/statistics` — requiere `Authorization: Bearer <token>`

```json
{
  "matrices": [
    { "name": "Q", "data": [[1,0],[0,1]] },
    { "name": "R", "data": [[3,4],[0,5]] }
  ],
  "tolerance": 1e-9
}
```

`200`:

```json
{
  "success": true,
  "data": { "global": { … }, "perMatrix": [ … ] },
  "metadata": {
    "tolerance": 1e-9,
    "analyzedMatrices": 2,
    "analyzedValues": 8,
    "computedAt": "2026-08-20T17:09:28.437Z"
  }
}
```

`metadata.tolerance` devuelve la tolerancia **efectivamente aplicada**: el valor
de `isDiagonal` no es interpretable sin conocerla.

#### `GET /health`

Sin autenticar.

---

## 8. Variables de entorno

### API en Go

| Variable | Por defecto | Descripción |
|---|---|---|
| `APP_NAME` | `reto-2026-api-go` | Nombre del servicio |
| `APP_ENV` | `development` | `development` \| `production` |
| `PORT` | `8080` | Puerto HTTP |
| `LOG_LEVEL` | `debug` | `debug` \| `info` \| `warn` \| `error` |
| `BODY_LIMIT_BYTES` | `5242880` | Límite del cuerpo (5 MB) |
| `READ_TIMEOUT` | `15s` | Lectura de la petición |
| `WRITE_TIMEOUT` | `30s` | Escritura de la respuesta |
| `STATISTICS_API_URL` | `http://localhost:3001` | URL de la API de Node |
| `STATISTICS_TIMEOUT` | `3s` | Timeout **por intento** |
| `STATISTICS_TOTAL_TIMEOUT` | `8s` | Techo de la operación **completa** |
| `STATISTICS_MAX_RETRIES` | `2` | Reintentos ante fallos transitorios |
| `STATISTICS_RETRY_BACKOFF` | `100ms` | Espera base, crece exponencialmente |
| `JWT_ISSUER` | `reto-2026-api-go` | Emisor de los tokens |
| `JWT_AUDIENCE` | `reto-2026` | Audiencia |
| `JWT_ACCESS_TTL` | `15m` | Vida del access token |
| `JWT_REFRESH_TTL` | `168h` | Vida del refresh token (7 días) |
| `JWT_PRIVATE_KEY` | — | PEM o base64. Prioridad sobre la ruta |
| `JWT_PRIVATE_KEY_PATH` | `../keys/private.pem` | Ruta al PEM privado |
| `JWT_PUBLIC_KEY` | — | PEM o base64 |
| `JWT_PUBLIC_KEY_PATH` | `../keys/public.pem` | Ruta al PEM público |
| `AUTH_USERNAME` | `admin` | Usuario del login simulado |
| `AUTH_PASSWORD` | `admin123` | **Prohibido en producción** |
| `CORS_ORIGINS` | `http://localhost:5173` | Lista por comas. **Nunca `*`** |
| `REFRESH_COOKIE_NAME` | `refresh_token` | Nombre de la cookie |
| `REFRESH_COOKIE_PATH` | `/api/v1/auth` | Acota dónde se envía |
| `REFRESH_COOKIE_DOMAIN` | *(vacío)* | Dominio de la cookie |
| `REFRESH_COOKIE_SECURE` | `true` en producción | Exige HTTPS |
| `REFRESH_COOKIE_SAMESITE` | `None` en prod, `Lax` en dev | Ver la sección de infraestructura |

### API en Node

| Variable | Por defecto | Descripción |
|---|---|---|
| `NODE_ENV` | `development` | `development` \| `test` \| `production` |
| `PORT` | `3001` | Puerto HTTP |
| `LOG_LEVEL` | `debug` | Nivel de log |
| `DIAGONAL_TOLERANCE` | `1e-9` | Umbral de cero por defecto |
| `JWT_ENABLED` | `true` | **Prohibido desactivarlo en producción** |
| `JWT_ISSUER` | `reto-2026-api-go` | Emisor esperado |
| `JWT_AUDIENCE` | `reto-2026` | Audiencia esperada |
| `JWT_PUBLIC_KEY` | — | PEM o base64. Prioridad sobre la ruta |
| `JWT_PUBLIC_KEY_PATH` | `../keys/public.pem` | Ruta al PEM público |
| `CORS_ORIGINS` | *(vacío)* | Vacío = CORS deshabilitado |

---

## 9. Tests

### API en Go — 29 tests (51 con subtests)

```bash
cd api-go
go test ./... -cover
go test ./internal/matrix/ -bench=. -benchtime=20x
```

| Paquete | Cobertura | Qué comprueba |
|---|---|---|
| `internal/matrix` | **97,0 %** | `A = Q·R`, `QᵀQ = I`, 8 formas de matriz, casos límite, determinismo, magnitudes extremas |
| `internal/core/usecase` | **82,5 %** | Caso de uso con doble del puerto, login, rotación, reutilización, expiración, concurrencia |

Los tests de álgebra no comprueban valores concretos sino **identidades
matemáticas**, que es lo que realmente define una factorización correcta:

```go
assertReconstructsOriginal(t, a, f)   // A = Q·R
assertOrthonormalColumns(t, f.Q)      // QᵀQ = I
assertUpperTriangular(t, f.R)         // ceros EXACTOS bajo la diagonal
```

Los tests de autenticación usan el **servicio JWT real** con un par de claves
RSA efímero, y un **reloj falso** que permite simular "han pasado ocho días" de
forma instantánea y determinista.

### API en Node — 41 tests

```bash
cd api-node
npm test
npm run test:coverage      # 87,6 % statements
```

| Suite | Qué comprueba |
|---|---|
| `statistics.test.js` | Las cinco métricas, precisión de Neumaier, diagonal con tolerancia, rectangulares, matriz nula |
| `analyze-matrices.test.js` | Resolución de tolerancia, incluido el caso `tolerance = 0` |
| `statistics.routes.test.js` | Aplicación completa con supertest: auth, validación, cálculo, errores |

Los tests de integración generan su propio par de claves RSA en memoria: son
autocontenidos y no dependen de ningún fichero ni secreto.

### End-to-end — 38 comprobaciones

```bash
docker compose up -d --build
node tests/e2e/run.mjs
```

Sin dependencias, solo `fetch` nativo. No solo comprueba códigos HTTP:
**recalcula la corrección matemática** y verifica que la API de Node sea
inalcanzable desde el host.

### Con detector de carreras

```bash
docker build --target test ./api-go     # go test -race, dentro del contenedor
docker build --target test ./api-node   # jest, dentro del contenedor
```

El detector de carreras necesita cgo, no disponible en Windows: por eso el
Dockerfile tiene una etapa `test` con `gcc` que lo ejecuta en Linux.
