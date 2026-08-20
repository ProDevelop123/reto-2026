import { Router } from 'express';

import { healthRouter } from './health.routes.js';
import { statisticsRouter } from './statistics.routes.js';

/**
 * Punto unico de montaje de rutas.
 *
 * El versionado va en la URL (/api/v1) para poder introducir cambios de
 * contrato incompatibles sin romper a los clientes ya desplegados.
 */
export const apiRouter = Router();

apiRouter.use('/health', healthRouter);
apiRouter.use('/api/v1/statistics', statisticsRouter);
