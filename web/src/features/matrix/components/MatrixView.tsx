import { cn } from "@/lib/utils";
import { formatCell } from "@/shared/lib/format";

import type { MatrixPayload } from "../types/qr.types";

interface MatrixViewProps {
  matrix: MatrixPayload;
  /** Rotula la matriz con su significado matemático. */
  caption?: string;
  /**
   * Atenúa los ceros por debajo de la diagonal.
   *
   * Se activa en R: sus ceros son estructurales —los produce el algoritmo por
   * construcción, no son el resultado de un cálculo— y pintarlos en gris hace
   * que la forma triangular superior salte a la vista de inmediato.
   */
  dimStructuralZeros?: boolean;
}

/** Renderiza una matriz con delimitadores tipográficos y cifras alineadas. */
export function MatrixView({ matrix, caption, dimStructuralZeros = false }: MatrixViewProps) {
  return (
    <figure className="space-y-2">
      <figcaption className="flex items-baseline gap-2">
        <span className="font-mono text-base font-semibold">{matrix.name}</span>
        <span className="text-muted-foreground tabular text-xs">
          {matrix.rows}×{matrix.columns}
        </span>
        {caption ? <span className="text-muted-foreground text-xs">{caption}</span> : null}
      </figcaption>

      <div className="flex items-stretch gap-1 overflow-x-auto">
        {/* Corchetes dibujados con bordes: escalan con el alto real de la
            matriz, cosa que un caracter "[" no haria. */}
        <div className="border-foreground/40 w-2 shrink-0 rounded-l-sm border-y border-l" />

        <table className="tabular border-separate border-spacing-x-3 border-spacing-y-1 text-sm">
          <tbody>
            {matrix.data.map((row, i) => (
              <tr key={i}>
                {row.map((value, j) => {
                  const isStructuralZero = dimStructuralZeros && j < i && value === 0;

                  return (
                    <td
                      key={j}
                      className={cn(
                        "text-right whitespace-nowrap",
                        isStructuralZero && "text-muted-foreground/40",
                      )}
                    >
                      {formatCell(value)}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>

        <div className="border-foreground/40 w-2 shrink-0 rounded-r-sm border-y border-r" />
      </div>
    </figure>
  );
}
