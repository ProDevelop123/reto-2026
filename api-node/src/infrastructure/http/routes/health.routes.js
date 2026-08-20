import { Router } from 'express';

export const healthRouter = Router();

/**
 * GET /health
 *
 * Sonda de vida sin autenticar. La consumen el healthcheck de Docker y el
 * balanceador de la plataforma cloud, que no disponen de un token, por lo que
 * exigir JWT aqui haria que el contenedor se reportara siempre como no sano.
 * No expone informacion sensible.
 */
healthRouter.get('/', (_req, res) => {
  res.status(200).json({
    success: true,
    data: { status: 'ok', service: 'api-node-statistics', uptime: process.uptime() },
  });
});
