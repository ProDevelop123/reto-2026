# Frontend (React 19 + Vite + shadcn/ui)

Interfaz que consume el sistema: introduce una matriz, la factoriza y muestra
Q, R, las estadísticas y una **verificación matemática independiente**.

![Workspace](./docs/workspace.png)

---

## Las tres ideas de la interfaz

### 1. La primera factorización cuesta un clic

Al entrar ya hay una matriz cargada. Quien evalúa no tiene que escribir nada
para ver el sistema completo funcionando: pulsa `Factorizar` y listo.

Además hay cinco ejemplos precargados, y **ninguno es decorativo**:

| Ejemplo | Qué demuestra |
|---|---|
| Clásica 3×3 | La matriz canónica de los libros. R reproduce `[[-14,-21,14],[0,-175,70],[0,0,35]]` con desviación de ~1e-14 |
| Rectangular 3×2 | El requisito literal del enunciado: matriz no cuadrada |
| Diagonal | Activa `isAnyDiagonal = true`, la quinta métrica exigida |
| Rango deficiente | `R[1][1] ≈ 0` sin división por cero — el caso que rompe implementaciones ingenuas |
| Mal condicionada | Matriz de Läuchli. `‖QᵀQ−I‖` se mantiene en ~1e-15 donde Gram-Schmidt daría 0.5 |

El botón de matriz aleatoria respeta las dimensiones puestas y genera enteros en
`[−20, 20]`: con decimales de quince dígitos la cuadrícula quedaría ilegible.

### 2. El navegador verifica al backend

El panel de verificación **no muestra datos de la API**: recalcula `Q·R` y `QᵀQ`
en JavaScript y mide los residuos.

```
‖A − Q·R‖   2.84e-14   ✓   La factorización reconstruye la matriz original
‖QᵀQ − I‖   3.33e-16   ✓   Las columnas de Q son ortonormales
```

Son ~15 líneas en [verification.ts](src/features/matrix/lib/verification.ts) y
convierten la demostración en una prueba: la corrección del algoritmo deja de
ser una afirmación del servidor.

La segunda identidad es la interesante. Gram-Schmidt clásico también satisface
`A = Q·R`, pero pierde la ortogonalidad con matrices mal condicionadas. Carga el
ejemplo *Mal condicionada* y compruébalo.

### 3. Cada resultado dice de dónde viene

| Bloque | Etiqueta |
|---|---|
| Matrices Q y R | `API en Go · Householder` |
| Estadísticas | `API en Node · vía la API en Go` |
| Residuos | `calculado en el navegador` |

Esto documenta la arquitectura visualmente y responde a la lectura literal del
enunciado (*"un frontend que consuma ambas APIs"*): el frontend **sí** consume
ambas, en una sola petición que la API en Go orquesta. La API de Node no está
expuesta al navegador —ni debe estarlo— y la respuesta ya trae su resultado.

---

## Decisiones técnicas

### Renovación de sesión en el interceptor de respuesta

El sistema de diseño del que se reutilizan los componentes vigila el reloj con
un componente que renueva de forma preventiva. Aquí se usa el patrón canónico:
**401 → renovar → reintentar**, en el interceptor de respuesta de axios.

Reacciona al 401 real en lugar de a un reloj que puede desincronizarse con el
del servidor, y la renovación es invisible para quien llamó: recibe la respuesta
que esperaba.

**Detalle que importa:** las renovaciones comparten una única promesa
([api.ts](src/shared/lib/api.ts)). Sin esa guarda, tres peticiones que fallen a
la vez lanzarían tres renovaciones; y como el backend **rota** el refresh token
en cada canje, la segunda llegaría con un token ya consumido. El servidor lo
interpretaría como reutilización —la firma de un token robado— y revocaría la
familia entera, **cerrando la sesión del usuario legítimo**. Compartir la
promesa convierte N renovaciones en una.

### Dónde vive cada credencial

| Credencial | Dónde | Por qué |
|---|---|---|
| Access token | Memoria + `localStorage` | Vida de 15 min. Recargar no obliga a reautenticarse. Un XSS podría leerlo: riesgo asumido y acotado |
| Refresh token | Cookie `HttpOnly` | Vida larga. Inaccesible desde JavaScript, así que un XSS **no** puede robarlo |

### Cabecera de verificación de origen

Todas las peticiones envían `X-Refresh-Request`. La API la exige en las rutas
cuya credencial es la cookie (`/auth/refresh`, `/auth/logout`).

En producción la cookie necesita `SameSite=None` —frontend en Vercel, API en
GCP son dominios distintos— y eso elimina la protección que `Lax` daba gratis.
Un formulario de un sitio ajeno no puede añadir cabeceras propias, y hacerlo
obliga a una consulta preliminar de CORS que rechazará su origen.

### Límite del grid

La API admite hasta **512×512**, pero renderizar esa cuadrícula serían 262 144
campos de entrada. El grid se limita a **12×12** y lo indica en pantalla: el
techo es de la interfaz, no del backend.

Para inspeccionar el contrato real sin abrir las DevTools está el acordeón de
**petición y respuesta**, de solo lectura. Hacerlo editable obligaría a
sincronizar en ambos sentidos el JSON con la cuadrícula y a decidir qué hacer
cuando alguien pega una matriz que el grid no puede representar; el beneficio
—ver el contrato— se obtiene igual sin asumir esa complejidad.

### Presentación de números

Los resultados de una factorización casi nunca son enteros exactos: la matriz
clásica produce `-20.999999999999993` donde la teoría dice `-21`. Sin
tolerancia, la misma matriz mostraría `-14` en una celda y `-21.0000` en la de
al lado, sugiriendo una diferencia de precisión que no existe.

[format.ts](src/shared/lib/format.ts) redondea con tolerancia relativa de 1e-9.
No oculta nada: a las cuatro cifras que se muestran son el mismo número, y el
valor completo sigue en el panel de payload.

Los ceros bajo la diagonal de R se pintan en gris: son **estructurales** —los
produce el algoritmo por construcción— y atenuarlos hace visible la forma
triangular.

---

## Qué se reutiliza y qué se descartó

El sistema de diseño de referencia es un panel de administración con 33
features, RBAC, i18n, mapas y websockets. Su `router.tsx` tiene 455 líneas.

**Reutilizado:** React 19 · Vite · TypeScript · Tailwind v4 · componentes shadcn
y sus tokens · `TokenService` · el patrón `ProtectedLayout` · la estructura
`app/` + `features/` + `shared/` · zustand con `persist`.

**Descartado:** RBAC y `PermissionRoute`, i18n, Firebase, los 13 layouts, las 33
features, mapas, websockets, tablas y gráficos.

**Añadido:** el interceptor de respuesta con cola de reintentos, y la paleta
oscura —el proyecto de referencia declaraba el variante `dark` pero nunca
definió los tokens, así que los componentes seguían leyendo los valores claros.

```
src/
├─ app/            router (3 rutas) + layouts
├─ features/
│  ├─ auth/        login, store, TokenService, aviso de caducidad
│  └─ matrix/      grid, resultados, verificación, payload
├─ shared/lib/     cliente HTTP y formato
└─ components/ui/  shadcn
```

Tres features en lugar de 33.

---

## Ejecución

```bash
npm install
cp .env.example .env      # VITE_API_URL=http://localhost:8080
npm run dev               # http://localhost:5173
```

Requiere el backend levantado. Desde la raíz del repositorio:

```bash
docker compose up -d --build
```

El puerto **5173 es fijo** (`strictPort`): es el origen que la API declara en
`CORS_ORIGINS`. Si Vite eligiera otro al estar ocupado, el navegador bloquearía
las peticiones y el fallo sería difícil de diagnosticar.

**Credenciales:** `admin` / `Reto2026.Demo` (visibles también en la pantalla de
login, para comodidad de quien revisa).

```bash
npm run build     # compilación de producción
npm run lint
```
