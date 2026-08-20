/**
 * Formato de numeros para la interfaz.
 *
 * Las matrices mezclan valores muy dispares —ceros exactos, enteros, decimales
 * de quince digitos y magnitudes de 1e-8— y mostrarlos todos igual haria la
 * cuadricula ilegible.
 */

/**
 * Tolerancia relativa con la que un valor se considera entero a efectos de
 * presentacion.
 *
 * Importa porque los resultados de una factorizacion casi nunca son enteros
 * exactos: la matriz clasica del ejemplo produce -20.999999999999993 donde la
 * teoria dice -21. Sin esta tolerancia, la misma matriz mostraria "-14" en una
 * celda y "-21.0000" en la de al lado, lo que sugiere una diferencia de
 * precision entre ambas que no existe.
 *
 * Redondear a "-21" no oculta nada: a las cuatro cifras decimales que se
 * muestran, "-21.0000" y "-21" son el mismo numero. El valor completo sigue
 * disponible en el panel de peticion y respuesta.
 */
const INTEGER_TOLERANCE = 1e-9;

/** Indica si un valor esta a distancia despreciable de un entero. */
function isNearInteger(value: number): boolean {
  const nearest = Math.round(value);
  return Math.abs(value - nearest) <= INTEGER_TOLERANCE * Math.max(1, Math.abs(value));
}

/**
 * Formatea un valor de matriz para mostrarlo en una celda.
 *
 * Por orden de prioridad:
 *  - Cero exacto: "0" a secas. En R son ceros estructurales, no el resultado de
 *    un calculo, y escribir "0.0000" sugeriria lo contrario.
 *  - Practicamente entero: sin decimales, segun el criterio de arriba.
 *  - Magnitud extrema: notacion cientifica, porque 0.00000001 en una celda
 *    estrecha se leeria como cero.
 *  - Resto: cuatro decimales.
 */
export function formatCell(value: number): string {
  if (value === 0) return "0";

  const magnitude = Math.abs(value);

  if (magnitude < 1e-4 || magnitude >= 1e6) return value.toExponential(2);
  if (isNearInteger(value)) return String(Math.round(value));

  return value.toFixed(4);
}

/** Formatea una metrica estadistica, donde importa mas la escala que el detalle. */
export function formatMetric(value: number): string {
  if (!Number.isFinite(value)) return "—";
  if (value === 0) return "0";

  const magnitude = Math.abs(value);

  if (magnitude < 1e-3 || magnitude >= 1e7) return value.toExponential(3);
  if (isNearInteger(value)) return String(Math.round(value));

  return value.toFixed(4);
}

/**
 * Formatea un residuo de verificacion.
 *
 * Siempre en notacion cientifica: lo relevante de estos valores es el ORDEN DE
 * MAGNITUD. La diferencia entre 1e-16 y 1e-1 es la que separa un algoritmo
 * estable de uno que no lo es, y en notacion decimal esa diferencia se pierde
 * de vista.
 */
export function formatResidual(value: number | null): string {
  if (value === null) return "—";
  if (value === 0) return "0";
  return value.toExponential(2);
}
