import { z } from 'zod';

/**
 * Esquema de validacion del cuerpo de POST /api/v1/statistics.
 *
 * La validacion se declara aqui, en el borde HTTP, para que el dominio pueda
 * asumir datos ya saneados y concentrarse solo en calcular. Es tambien el
 * contrato ejecutable entre esta API y la API en Go: si el contrato cambia,
 * cambia este fichero.
 *
 * Nota: en Zod 4 `z.number()` ya rechaza NaN e Infinity, de modo que no hace
 * falta un `.finite()` adicional. Esto importa porque una matriz con Infinity
 * envenenaria el maximo y el promedio silenciosamente.
 */

/** Una fila: al menos un numero finito. */
const rowSchema = z.array(z.number()).min(1, 'Cada fila debe tener al menos un elemento.');

/**
 * Matriz rectangular: array no vacio de filas de identica longitud.
 *
 * La comprobacion de rectangularidad se hace con `superRefine` para poder
 * senalar la fila exacta que rompe la forma, en lugar de devolver un error
 * generico sobre toda la matriz.
 */
const matrixDataSchema = z
  .array(rowSchema)
  .min(1, 'La matriz debe tener al menos una fila.')
  .superRefine((rows, ctx) => {
    const width = rows[0].length;

    rows.forEach((row, index) => {
      if (row.length !== width) {
        ctx.addIssue({
          code: 'custom',
          path: [index],
          message:
            `La matriz no es rectangular: la fila ${index} tiene ${row.length} ` +
            `elemento(s) y se esperaban ${width}.`,
        });
      }
    });
  });

/** Una matriz identificada. El nombre es opcional y sirve para trazabilidad (Q, R, ...). */
const matrixSchema = z.object({
  name: z.string().trim().min(1).max(64).optional(),
  data: matrixDataSchema,
});

export const analyzeMatricesSchema = z.object({
  /**
   * Coleccion de matrices a analizar. Se modela como lista generica y no como
   * un par { Q, R } a proposito: esta API es un servicio de estadisticas puro
   * y no debe saber nada de factorizacion QR. Esa ignorancia deliberada la hace
   * reutilizable y mantiene la responsabilidad del algebra lineal del lado de Go.
   */
  matrices: z
    .array(matrixSchema)
    .min(1, 'Se requiere al menos una matriz.')
    .max(16, 'Se admiten como maximo 16 matrices por peticion.'),

  /**
   * Tolerancia opcional para la deteccion de matriz diagonal. Permite al
   * cliente ajustar el umbral cuando conoce el condicionamiento numerico de
   * sus datos. Si se omite, se usa el valor de configuracion del servicio.
   */
  tolerance: z.number().nonnegative().optional(),
});
