# API de Estadísticas (Node.js + Express)

Segunda API del reto. Recibe un conjunto de matrices — en el flujo del sistema,
la **Q** y la **R** producidas por la factorización QR de la API en Go — y
calcula sobre ellas el valor máximo, el mínimo, el promedio, la suma total y si
alguna es diagonal.

Este servicio **no sabe nada de factorización QR**. Recibe una lista genérica de
matrices y devuelve estadísticas. Esa ignorancia es deliberada: mantiene toda la
responsabilidad del álgebra lineal en la API de Go y hace este servicio
reutilizable para cualquier otro consumidor.

---

## Contrato HTTP

### `POST /api/v1/statistics`

Requiere `Authorization: Bearer <access_token>`.

**Petición**

```json
{
  "matrices": [
    { "name": "Q", "data": [[-0.1690, 0.8971], [-0.5071, 0.2760], [-0.8452, -0.3450]] },
    { "name": "R", "data": [[-5.9161, -7.4374], [0, 0.8281]] }
  ],
  "tolerance": 1e-9
}
```

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `matrices` | array (1..16) | sí | Matrices a analizar |
| `matrices[].name` | string (1..64) | no | Etiqueta de trazabilidad. Si se omite se asigna `matrix_<índice>` |
| `matrices[].data` | number[][] | sí | Matriz rectangular no vacía. Todas las filas deben tener la misma longitud |
| `tolerance` | number ≥ 0 | no | Umbral bajo el cual un valor se considera cero al comprobar si la matriz es diagonal. Por defecto, `DIAGONAL_TOLERANCE` |

**Respuesta `200`**

```json
{
  "success": true,
  "data": {
    "global": {
      "matrices": 2,
      "count": 10,
      "max": 0.8971,
      "min": -7.4374,
      "sum": -13.2186,
      "average": -1.3219,
      "isAnyDiagonal": false,
      "diagonalMatrices": []
    },
    "perMatrix": [
      {
        "name": "Q",
        "rows": 3, "columns": 2, "count": 6,
        "max": 0.8971, "min": -0.8452,
        "sum": -0.6932, "average": -0.1155,
        "isSquare": false, "isDiagonal": false
      }
    ]
  },
  "metadata": {
    "tolerance": 1e-9,
    "analyzedMatrices": 2,
    "analyzedValues": 10,
    "computedAt": "2026-08-20T15:57:26.352Z"
  }
}
```

`global` responde literalmente a lo que pide el enunciado (*"el valor máximo
encontrado **en las matrices**"*). `perMatrix` se añade porque saber *cuál* de
las dos matrices es la diagonal es más útil que saber solo que alguna lo es, y
calcularlo no cuesta ninguna pasada extra sobre los datos.

`metadata.tolerance` devuelve el umbral **efectivamente aplicado**: el valor de
`isDiagonal` no es interpretable sin conocerlo.

### `GET /health`

Sonda de vida sin autenticar, para el healthcheck de Docker y el balanceador de
la plataforma cloud —que no disponen de token—. No expone información sensible.

### Formato de error

Todas las respuestas de error comparten forma:

```json
{
  "success": false,
  "error": { "code": "VALIDATION_ERROR", "message": "...", "details": { } }
}
```

| Código HTTP | `error.code` | Cuándo |
|---|---|---|
| 401 | `UNAUTHORIZED` | Falta el token, está mal formado, expiró, lo firmó otra clave, o es un *refresh token* usado como token de acceso |
| 404 | `NOT_FOUND` | Ruta inexistente |
| 422 | `VALIDATION_ERROR` | El cuerpo no cumple el esquema. `details` es un árbol con la misma forma del payload, señalando la fila exacta que falla |
| 500 | `INTERNAL_ERROR` | Fallo no previsto. Se registra completo en el log, pero al cliente no se le filtran rutas ni trazas |

En `401`, `error.details.reason` distingue el motivo (`missing_header`,
`malformed_header`, `token_invalid`, `token_expired`, `wrong_token_type`), lo
que permite al frontend decidir si debe renovar el token o volver a autenticar.

---

## Decisiones técnicas

### Sin librería matemática

Las cinco métricas se calculan en **una sola pasada O(m·n)** sin arrays
intermedios. Traer `mathjs` (que arrastra `decimal.js` y `fraction.js`) para
calcular un máximo añadiría peso a la imagen y tiempo de arranque sin aportar
capacidad alguna.

### Suma compensada de Neumaier

Los valores de Q son del orden de 10⁻¹ mientras que los de R pueden ser varios
órdenes de magnitud mayores. Una suma ingenua con `+=` acumula error de redondeo
que se propaga al promedio. La compensación cuesta un par de operaciones por
elemento y mantiene la precisión. El caso está cubierto por un test: una matriz
con `[1e16, 1, -1e16, 0]` da `0` con suma ingenua y `1` con la compensada.

### Tolerancia en la detección de diagonal

Las matrices llegan de una factorización QR, donde los ceros teóricos son en
realidad residuos del orden de `1e-16`. Comparar contra cero exacto haría que
ninguna matriz real se detectara como diagonal. Por eso se usa un umbral
configurable, con `1e-9` por defecto.

Se adopta la definición general `a[i][j] = 0 para todo i ≠ j`, que admite
matrices **rectangulares** diagonales —las mismas que aparecen como matriz Σ en
una descomposición SVD— y considera diagonal la matriz nula, en línea con el
álgebra lineal estándar.

### JWT RS256: este servicio solo verifica

La API en Go firma con la clave **privada**; esta API conoce únicamente la
**pública**. Es incapaz de emitir tokens, de modo que comprometerla no permite
suplantar a nadie. Con un secreto compartido (HS256) ambos servicios podrían
firmar.

El algoritmo se fija explícitamente en la verificación, lo que cierra el ataque
clásico de confusión de algoritmo (cambiar `alg` a `HS256` o a `none`). Además se
valida `issuer`, `audience` y que `tokenType` sea `access`: un *refresh token*
está firmado con la misma clave y pasaría la verificación criptográfica, pero no
debe servir para consumir la API.

### Arquitectura en capas, no Clean Architecture completa

```
src/
├─ domain/            lógica pura, sin HTTP ni configuración
├─ application/       caso de uso: orquesta y resuelve configuración
├─ infrastructure/    Express, JWT, esquemas — todo lo reemplazable
└─ config/            entorno
```

No hay puertos ni adaptadores porque **este servicio no tiene dependencias
externas que invertir**: no llama a nadie ni persiste nada. Montar
ports & adapters aquí sería ceremonia sin justificación. La API en Go sí los
usa, porque allí sí existe una dependencia externa (este servicio) que conviene
poder sustituir por un doble en los tests.

### Otras

- **Autenticar antes de validar.** Validar primero regalaría capacidad de
  cómputo a peticiones anónimas.
- **Cabeceras censuradas en los logs.** `authorization`, `cookie` y
  `set-cookie` se reemplazan por `[REDACTED]` antes de escribir al log.
- **Fallar en el arranque.** `validateConfig()` aborta el proceso si la
  configuración es insegura (por ejemplo, `JWT_ENABLED=false` en producción). Un
  contenedor que no levanta es un problema visible; uno que sirve peticiones sin
  autenticación, no.
- **Límite de cuerpo de 5 MB** y máximo de 16 matrices por petición: una matriz
  grande es legítima, un payload ilimitado es un vector de agotamiento de memoria.
- **Cierre ordenado** ante `SIGTERM`/`SIGINT`, para no cortar peticiones a medio
  responder durante un redespliegue.

---

## Ejecución

### Local

```bash
npm install
sh ../scripts/generate-keys.sh ../keys   # solo la primera vez
cp .env.example .env
npm run dev
```

### Tests

```bash
npm test              # 41 tests
npm run test:coverage
```

Los tests de integración generan un par de claves RSA efímero en memoria: son
autocontenidos y no dependen de ningún fichero ni secreto del entorno.

### Docker

```bash
docker build -t reto-2026/api-node .
docker run --rm -p 3001:3001 \
  -e JWT_PUBLIC_KEY="$(cat ../keys/public.pem)" \
  reto-2026/api-node
```

## Variables de entorno

Ver [.env.example](.env.example). Las relevantes:

| Variable | Por defecto | Descripción |
|---|---|---|
| `PORT` | `3001` | Puerto HTTP |
| `NODE_ENV` | `development` | `development` \| `test` \| `production` |
| `DIAGONAL_TOLERANCE` | `1e-9` | Umbral de cero por defecto |
| `JWT_ENABLED` | `true` | Prohibido desactivarlo en producción |
| `JWT_ISSUER` | `reto-2026-api-go` | Emisor esperado |
| `JWT_AUDIENCE` | `reto-2026` | Audiencia esperada |
| `JWT_PUBLIC_KEY` | — | PEM completo o en base64. Tiene prioridad sobre la ruta |
| `JWT_PUBLIC_KEY_PATH` | `../keys/public.pem` | Ruta al PEM público |
| `CORS_ORIGINS` | vacío | Lista separada por comas. Vacío = CORS deshabilitado |
