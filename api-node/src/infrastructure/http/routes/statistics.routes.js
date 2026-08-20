import { Router } from 'express';

import { analyzeMatricesController } from '../controllers/statistics.controller.js';
import { authMiddleware } from '../middlewares/auth.middleware.js';
import { validateBody } from '../middlewares/validate.middleware.js';
import { analyzeMatricesSchema } from '../schemas/statistics.schema.js';

export const statisticsRouter = Router();

/**
 * POST /api/v1/statistics
 *
 * Recibe una coleccion de matrices (en el flujo del reto, la Q y la R
 * producidas por la API en Go) y devuelve el valor maximo, el minimo, el
 * promedio, la suma total y si alguna de ellas es diagonal.
 *
 * La cadena de middlewares expresa el orden de defensa: primero se comprueba
 * QUIEN llama (auth), y solo despues se valida y procesa QUE envia. Validar
 * antes de autenticar regalaria capacidad de computo a peticiones anonimas.
 */
statisticsRouter.post(
  '/',
  authMiddleware,
  validateBody(analyzeMatricesSchema),
  analyzeMatricesController,
);
