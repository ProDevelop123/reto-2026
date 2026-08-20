import { Badge } from "@/components/ui/badge";
import { formatMetric } from "@/shared/lib/format";

import type { Statistics } from "../types/qr.types";

interface StatisticsPanelProps {
  statistics: Statistics;
}

/** Una métrica global, presentada como tarjeta. */
function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-muted/40 rounded-lg border p-3">
      <p className="text-muted-foreground text-[0.7rem] font-medium tracking-wide uppercase">
        {label}
      </p>
      <p className="tabular mt-1 truncate text-lg font-semibold" title={value}>
        {value}
      </p>
    </div>
  );
}

/**
 * Estadísticas calculadas por la API de Node.
 *
 * Muestra las cinco métricas que exige el enunciado sobre el conjunto de las
 * matrices y, debajo, el desglose por matriz. El desglose importa: saber que
 * "alguna matriz es diagonal" es menos útil que saber CUÁL lo es.
 */
export function StatisticsPanel({ statistics }: StatisticsPanelProps) {
  const { global, perMatrix } = statistics;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
        <Metric label="Máximo" value={formatMetric(global.max)} />
        <Metric label="Mínimo" value={formatMetric(global.min)} />
        <Metric label="Promedio" value={formatMetric(global.average)} />
        <Metric label="Suma total" value={formatMetric(global.sum)} />
      </div>

      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="text-muted-foreground">Matriz diagonal:</span>
        {global.isAnyDiagonal ? (
          <>
            <Badge>Sí</Badge>
            <span className="text-muted-foreground text-xs">
              {global.diagonalMatrices.join(", ")}
            </span>
          </>
        ) : (
          <Badge variant="secondary">Ninguna</Badge>
        )}
        <span className="text-muted-foreground tabular ml-auto text-xs">
          {global.count} valores en {global.matrices} matrices
        </span>
      </div>

      {/* --- Desglose por matriz --- */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-muted-foreground border-b text-left text-xs">
              <th className="py-1.5 pr-3 font-medium">Matriz</th>
              <th className="py-1.5 pr-3 text-right font-medium">Máx</th>
              <th className="py-1.5 pr-3 text-right font-medium">Mín</th>
              <th className="py-1.5 pr-3 text-right font-medium">Promedio</th>
              <th className="py-1.5 pr-3 text-right font-medium">Suma</th>
              <th className="py-1.5 text-right font-medium">Diagonal</th>
            </tr>
          </thead>
          <tbody className="tabular">
            {perMatrix.map((m) => (
              <tr key={m.name} className="border-b last:border-0">
                <td className="py-1.5 pr-3 font-mono font-medium">
                  {m.name}
                  <span className="text-muted-foreground ml-1.5 text-xs">
                    {m.rows}×{m.columns}
                  </span>
                </td>
                <td className="py-1.5 pr-3 text-right">{formatMetric(m.max)}</td>
                <td className="py-1.5 pr-3 text-right">{formatMetric(m.min)}</td>
                <td className="py-1.5 pr-3 text-right">{formatMetric(m.average)}</td>
                <td className="py-1.5 pr-3 text-right">{formatMetric(m.sum)}</td>
                <td className="py-1.5 text-right">
                  {m.isDiagonal ? (
                    <Badge className="h-5 px-1.5 text-[0.7rem]">Sí</Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
