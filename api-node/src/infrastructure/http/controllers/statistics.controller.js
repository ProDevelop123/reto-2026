import { analyzeMatrices } from '../../../application/analyze-matrices.usecase.js';

/**
 * Controlador de POST /api/v1/statistics.
 *
 * Capa fina por diseno: el cuerpo ya viene validado por el middleware de
 * esquema, asi que aqui solo se adapta entre el mundo HTTP y el caso de uso.
 * Toda la logica testeable vive por debajo.
 */
export function analyzeMatricesController(req, res) {
  const { matrices, tolerance } = req.body;

  const { statistics, metadata } = analyzeMatrices({ matrices, tolerance });

  res.status(200).json({ success: true, data: statistics, metadata });
}
