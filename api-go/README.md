# API de Factorización QR (Go + Fiber v3)

Primera API del reto y orquestadora del sistema. Recibe una matriz rectangular,
calcula su **factorización QR** mediante reflexiones de Householder, envía las
matrices resultantes a la API de estadísticas en Node y devuelve todo en una
sola respuesta.

```
Cliente ──POST /api/v1/qr──▶ API Go (Fiber v3)
                              │ A = Q·R  (Householder)
                              │
                              ├──POST /api/v1/statistics──▶ API Node (Express)
                              │                              │ max, min, avg, sum,
                              │◀──── estadísticas ───────────┘ ¿diagonal?
                              │
        ◀── { matrix, q, r, statistics } ──┘
```

---

## Sobre la ambigüedad del enunciado

El PDF del reto dice **"rotación de la matriz"** en la lámina de arquitectura y
**"factorización QR"** en la de funcionalidad requerida. Se interpreta que manda
la segunda, por dos razones:

1. Es la que describe la funcionalidad exigible; la otra parece texto reciclado
   de un enunciado anterior sobre rotación de matrices.
2. Los términos no son ajenos entre sí. Una de las tres vías clásicas hacia la
   QR son las **rotaciones de Givens**, que anulan los elementos subdiagonales
   aplicando rotaciones planas hasta obtener exactamente `A = Q·R`.

Se implementa **Householder** y no Givens porque es el estándar para matrices
densas —lo que usan LAPACK y las bibliotecas de referencia—: anula una columna
entera por reflexión en lugar de un elemento por rotación. Givens es preferible
cuando la matriz es dispersa o ya casi triangular, que no es el caso aquí.

---

## Contrato HTTP

Formato de respuesta compartido con la API de Node, pese a estar en lenguajes
distintos: **el contrato pertenece al sistema, no al framework.**

```jsonc
// Éxito
{ "success": true, "data": { … }, "metadata": { … } }
// Error
{ "success": false, "error": { "code": "…", "message": "…", "details": { … } } }
```

### `POST /api/v1/qr` — requiere `Authorization: Bearer <token>`

```json
{ "matrix": [[1,2],[3,4],[5,6]], "mode": "reduced", "tolerance": 1e-9 }
```

| Campo | Tipo | Oblig. | Descripción |
|---|---|---|---|
| `matrix` | number[][] | sí | Matriz rectangular no vacía, hasta 512×512 |
| `mode` | `"reduced"` \| `"complete"` | no | Por defecto `reduced` |
| `tolerance` | number ≥ 0 | no | Umbral de cero para la detección de diagonal |

Respuesta `200`:

```jsonc
{
  "success": true,
  "data": {
    "matrix": { "name": "A", "rows": 3, "columns": 2, "data": [[1,2],[3,4],[5,6]] },
    "q":      { "name": "Q", "rows": 3, "columns": 2, "data": [[-0.169, 0.897], …] },
    "r":      { "name": "R", "rows": 2, "columns": 2, "data": [[-5.916,-7.437],[0,0.828]] },
    "statistics": { "global": { … }, "perMatrix": [ … ] }
  },
  "metadata": {
    "mode": "reduced", "reflectors": 2, "tolerance": 1e-9,
    "durationMs": 14.4, "computedAt": "2026-08-20T16:34:50.885Z"
  }
}
```

### Autenticación

| Ruta | Descripción |
|---|---|
| `POST /api/v1/auth/login` | Credenciales estáticas. Devuelve el *access token* en el cuerpo y el *refresh* en cookie HttpOnly |
| `POST /api/v1/auth/refresh` | Lee la cookie, **rota** el token y devuelve un par nuevo |
| `POST /api/v1/auth/logout` | Revoca la familia completa de tokens |
| `GET /health` | Sonda de vida, sin autenticar |

### Códigos de error

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `BAD_REQUEST` | El cuerpo no es JSON válido |
| 401 | `UNAUTHORIZED` | Token ausente, mal formado, expirado, de tipo incorrecto, o credenciales inválidas |
| 404 | `NOT_FOUND` | Ruta inexistente fuera de `/api/v1` |
| 422 | `VALIDATION_ERROR` | Matriz no rectangular, vacía, con valores no finitos, o parámetros fuera de rango |
| 502 | `UPSTREAM_ERROR` | La API de estadísticas no respondió o falló |
| 504 | `UPSTREAM_TIMEOUT` | La API de estadísticas agotó el tiempo de espera |

Una ruta inexistente **dentro** de `/api/v1` responde `401`, no `404`. Es
deliberado: no se revela qué rutas existen a quien no se ha autenticado.

---

## Arquitectura

```
cmd/api/main.go              punto de entrada y composición de dependencias
internal/
├─ matrix/                   álgebra lineal pura — cero dependencias externas
│  ├─ matrix.go
│  └─ householder.go         ← la pieza central
├─ core/
│  ├─ domain/                conceptos del negocio, sin HTTP ni JSON
│  ├─ port/                  interfaces que el núcleo necesita del exterior
│  └─ usecase/               orquestación
├─ infrastructure/
│  ├─ http/                  Fiber v3: router, handlers, DTOs, middlewares
│  ├─ jwt/                   emisión y verificación RS256
│  ├─ client/                adaptador HTTP → API de Node
│  ├─ persistence/memory/    almacén de sesiones
│  └─ system/                reloj y generador de identificadores
└─ config/
pkg/apperror/                error de aplicación con código HTTP
```

### Por qué hexagonal aquí y no en la API de Node

Porque **esta API depende de servicios externos y aquella no**. Declarar esa
dependencia como un puerto (`StatisticsProvider`) invierte la dirección: la
infraestructura implementa lo que el núcleo define.

El beneficio es concreto y verificable: `qr_usecase_test.go` ejercita el caso de
uso completo —factorización incluida— **sin levantar la API de Node, sin abrir
un socket y sin depender de la red**. Los tests corren en microsegundos y no
pueden fallar de forma intermitente.

Aplicar la misma ceremonia a un servicio que solo calcula cinco métricas sería
inconsistencia disfrazada de consistencia.

---

## Decisiones técnicas

### Householder, y por qué el signo importa

Tres caminos llevan a la QR:

- **Gram-Schmidt clásico** — intuitivo pero numéricamente inestable.
- **Rotaciones de Givens** — estable, óptimo para matrices dispersas.
- **Reflexiones de Householder** — estable e incondicional, estándar para densas.

La diferencia no es teórica. Midiendo `‖QᵀQ − I‖` sobre la matriz de Läuchli
(columnas casi idénticas):

| ε | Householder | Gram-Schmidt clásico |
|---|---|---|
| 1e-6 | 4.4e-16 | 5.1e-05 |
| 1e-8 | 1.1e-15 | **5.0e-01** |
| 1e-10 | 4.4e-16 | **5.0e-01** |

Householder se mantiene en el epsilon de la máquina; Gram-Schmidt pierde la
ortogonalidad por completo.

La clave está en el signo: `α = −copysign(‖x‖, x₀)`. Con el signo contrario, un
vector ya casi alineado con e₁ produciría una resta entre cantidades casi
iguales —cancelación catastrófica— y el resultado tendría muy pocos dígitos
significativos. Es el error clásico al implementar el algoritmo.

### Eficiencia

- **Las reflexiones nunca se materializan.** Aplicar `H = I − 2vvᵀ` a una matriz
  es `M − 2v(vᵀM)`: producto matriz-vector más actualización de rango uno.
  Nunca se construye la matriz `H` de m×m. Pasa de O(m²) a O(m·n) por reflexión.
- **Asignaciones constantes.** El benchmark da **10 allocations sea cual sea el
  tamaño** (50×50, 200×50, 200×200). Vienen de reservar cada matriz en un único
  bloque contiguo y de reutilizar el espacio de trabajo entre reflexiones: el
  recolector de basura no trabaja dentro del algoritmo.
- **Norma con escalado dinámico.** Elevar al cuadrado desbordaría con valores
  ~1e200 y subdesbordaría a cero con ~1e-200. Hay test para ambos extremos.
- **Modo reducido por defecto.** Para una matriz de 512×3, la Q completa serían
  262 144 valores frente a 1536.

### Seguridad

- **RS256 asimétrico.** Esta API es la única que posee la clave privada. Node
  solo recibe la pública: puede verificar pero no emitir, así que comprometerlo
  no permite suplantar a nadie. Con HS256 ambos podrían firmar.
- **Algoritmo fijado en la verificación**, lo que cierra el ataque de confusión
  de algoritmo (`alg: none`, o HS256 usando la clave pública como secreto).
- **El tipo de token se comprueba.** *Access* y *refresh* se firman con la misma
  clave, así que la verificación criptográfica no los distingue; el claim
  `tokenType` impide usar un refresh de vida larga como token de acceso.
- **Rotación de refresh tokens con detección de reutilización.** Cada refresco
  invalida el token anterior. Si llega uno ya canjeado, hay dos copias en
  circulación y una es de un atacante: como no se puede saber cuál, se revoca la
  **familia entera**. Verificado por HTTP en los tests.
- **Refresh en cookie HttpOnly**, nunca en el cuerpo: un XSS en el frontend no
  puede robarlo. El *access token* vive solo en memoria del cliente.
- **Comparación de credenciales en tiempo constante**, para no filtrar por
  temporización cuántos caracteres iniciales son correctos.
- **Mensaje único** para usuario inexistente y contraseña incorrecta:
  distinguirlos permitiría enumerar usuarios.
- **La causa interna nunca sale al cliente.** Se registra en el log; la respuesta
  lleva solo el mensaje redactado para ser expuesto.
- **Ni el cuerpo ni la cabecera `Authorization` se registran nunca.**
- **El arranque falla** si la configuración es insegura: contraseña por defecto
  en producción, `SameSite=None` sin `Secure`, o comodín en `CORS_ORIGINS`.

### Resiliencia

- **Timeout, reintentos y backoff exponencial** hacia la API de Node. Solo se
  reintentan fallos transitorios (red y 5xx); un 4xx no mejora al repetirse.
- **Fallo explícito, no degradación silenciosa.** Si Node cae, la respuesta es
  `502` en ~300 ms (3 intentos). Se consideró devolver Q y R con las
  estadísticas vacías y se descartó: el contrato promete estadísticas, y un 200
  a medias obligaría a todos los clientes a comprobar si llegaron.
- **`recover`** convierte un panic en un 500 en lugar de derribar el proceso.
- **Cierre ordenado** ante `SIGTERM`, con 15 s de gracia.
- **Limpieza periódica** de sesiones caducadas: sin ella el mapa crecería
  indefinidamente.

### Sobre `metadata.reflectors`

Es el número de reflexiones aplicadas, normalmente `min(m,n)`. **No es un
estimador del rango.** Sobre una matriz deficiente de rango pero sin ceros
exactos, la subcolumna residual queda en ~1e-16 en lugar de en cero, así que se
aplica una reflexión más sobre ruido de redondeo —el mismo comportamiento de
LAPACK—. Determinar el rango numérico de forma fiable exige **QR con pivoteo por
columnas**, que es un algoritmo distinto. El contador solo baja cuando una
columna es exactamente nula.

---

## Ejecución

```bash
sh ../scripts/generate-keys.sh ../keys   # solo la primera vez
cp .env.example .env
go run ./cmd/api
```

Requiere la API de Node escuchando en `STATISTICS_API_URL`.

### Tests

```bash
go test ./...              # 29 tests, 51 contando subtests
go test ./... -cover
go test ./internal/matrix/ -bench=. -benchtime=20x
```

El detector de carreras (`-race`) necesita cgo; se ejecuta en CI y en Docker,
donde el compilador de C está disponible.

### Variables de entorno

Ver [.env.example](.env.example). El punto que más fricción causa al desplegar:
con el frontend en Vercel y la API en GCP, la petición de refresco es de origen
cruzado, así que la cookie necesita `SameSite=None` **y** `Secure=true` (que
exige HTTPS). Está resuelto por defecto según el entorno.
