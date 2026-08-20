import { api } from "@/shared/lib/api";

import type { FactorizeRequest, FactorizeResponse } from "../types/qr.types";

/**
 * Llamada al endpoint de factorizacion de la API en Go.
 *
 * Una sola peticion dispara el pipeline completo del sistema: Go calcula la
 * factorizacion QR y llama a la API de estadisticas en Node, y devuelve ambos
 * resultados juntos. El frontend consume las dos APIs, aunque solo hable
 * directamente con una: la de Node no esta —ni debe estar— expuesta al
 * navegador.
 */
export async function factorize(request: FactorizeRequest): Promise<FactorizeResponse> {
  const { data } = await api.post<FactorizeResponse>("/api/v1/qr", request);
  return data;
}
