# API de Estadísticas (Node.js + Express 5)

Segunda API del reto. Recibe un conjunto de matrices —en el flujo del sistema,
la **Q** y la **R** producidas por la API en Go— y calcula el valor máximo, el
mínimo, el promedio, la suma total y si alguna es diagonal.

Este servicio **no sabe qué es una factorización QR**. Recibe una lista genérica
de matrices y devuelve métricas. Esa ignorancia es deliberada: mantiene toda la
responsabilidad del álgebra del lado de Go y hace el servicio reutilizable.

> **Documentación detallada:** [`docs/backend.md`](../docs/backend.md)
> — el cálculo en una pasada, la suma compensada de Neumaier, la detección de
> matriz diagonal con tolerancia, referencia del endpoint y variables de entorno.

---

## Arranque

Lo mas comodo es levantar todo el sistema desde la raiz del repositorio, que
genera las claves automaticamente:

```bash
docker compose up -d --build
```

Para ejecutar solo este servicio, fuera de Docker:

```bash
npm install
sh ../scripts/generate-keys.sh ../keys   # solo si no hay claves todavia
cp .env.example .env
npm run dev
```

## Tests

```bash
npm test                       # 41 tests
npm run test:coverage          # 87,6 % statements
docker build --target test .   # dentro del contenedor
```

Los tests de integración generan su propio par de claves RSA en memoria: son
autocontenidos y no dependen de ningún fichero ni secreto del entorno.

## Estructura

```
src/
├─ domain/            lógica pura, sin HTTP ni configuración
├─ application/       caso de uso
├─ infrastructure/    Express · JWT · esquemas zod
└─ config/
```

No hay puertos ni adaptadores: **este servicio no tiene dependencias externas
que invertir**. No llama a nadie ni persiste nada.

## Endpoints

| Método | Ruta | Autenticación |
|---|---|---|
| `POST` | `/api/v1/statistics` | `Authorization: Bearer` |
| `GET` | `/health` | — |

## En una línea

Cinco métricas en una sola pasada O(m·n) · suma compensada de Neumaier para no
perder precisión · tolerancia configurable porque los ceros de una QR real son
residuos de ~1e-16 · solo verifica tokens, nunca los emite.
