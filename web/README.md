# Frontend (React 19 + Vite + shadcn/ui)

Interfaz que consume el sistema: introduce una matriz, la factoriza y muestra
Q, R, las estadísticas y una **verificación matemática independiente**.

> **Documentación detallada:** [`docs/frontend.md`](../docs/frontend.md)
> — los flujos paso a paso, la verificación en el navegador, el manejo de
> sesión, los componentes y la presentación de números.

![Workspace](./docs/workspace.png)

---

## Arranque

```bash
npm install
cp .env.example .env      # VITE_API_URL=http://localhost:8080
npm run dev
```

→ **http://localhost:5173** · usuario `admin` · contraseña `Reto2026.Demo`

Requiere el backend levantado. Desde la raíz del repositorio:

```bash
docker compose up -d --build
```

El puerto **5173 es fijo** (`strictPort`): es el origen que la API declara en
`CORS_ORIGINS`. Si Vite eligiera otro al estar ocupado, el navegador bloquearía
las peticiones y el fallo sería difícil de diagnosticar.

## Comandos

```bash
npm run dev       # desarrollo con recarga en caliente
npm run build     # compilación de producción
npm run preview   # sirve la compilación
npm run lint      # análisis estático
```

## Las tres ideas

**1. La primera factorización cuesta un clic.** Al entrar ya hay una matriz
cargada, más cinco ejemplos que demuestran cada uno una propiedad concreta del
backend: la clásica de los libros, una rectangular, una diagonal, una de rango
deficiente y la de Läuchli, donde se ve que Householder conserva la
ortogonalidad allí donde Gram-Schmidt la pierde.

**2. El navegador verifica al backend.** El panel de verificación no muestra
datos de la API: recalcula `Q·R` y `QᵀQ` en JavaScript y mide los residuos. La
corrección del algoritmo deja de ser una afirmación del servidor.

**3. Cada resultado dice de dónde viene.** Las matrices llevan la etiqueta *API
en Go*, las estadísticas *API en Node*, y los residuos *calculado en el
navegador*. Documenta la arquitectura visualmente.

## Estructura

```
src/
├─ app/            router (3 rutas) + layouts
├─ features/
│  ├─ auth/        login, store, TokenService, aviso de caducidad
│  └─ matrix/      grid, resultados, verificación, payload
├─ shared/lib/     cliente HTTP y formato
└─ components/ui/  shadcn
```

Tres features, frente a las 33 del sistema de diseño del que se reutilizan los
componentes.

## Variables de entorno

| Variable | Por defecto | Descripción |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | URL base de la API en Go |

Solo hay una: el frontend **nunca habla con la API de Node**, que no está
expuesta. Su resultado ya viene en la respuesta de Go.
