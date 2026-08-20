import { CheckCircle2, XCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { formatResidual } from "@/shared/lib/format";

import type { Check } from "../lib/verification";

interface VerificationPanelProps {
  checks: Check[];
}

/**
 * Verificación matemática realizada en el navegador.
 *
 * Es el panel que convierte la demostración en una prueba: los residuos que se
 * muestran los calcula el cliente por su cuenta, sin confiar en el servicio que
 * produjo el resultado. Que ‖A − Q·R‖ salga en el orden de 1e-16 no es una
 * afirmación del backend, es una comprobación independiente.
 */
export function VerificationPanel({ checks }: VerificationPanelProps) {
  return (
    <div className="space-y-2">
      {checks.map((check) => (
        <div
          key={check.label}
          className="flex items-center gap-3 rounded-lg border px-3 py-2"
        >
          {check.passed ? (
            <CheckCircle2 className="size-4 shrink-0 text-emerald-600 dark:text-emerald-500" />
          ) : (
            <XCircle className="text-destructive size-4 shrink-0" />
          )}

          <div className="min-w-0 flex-1">
            <p className="font-mono text-sm font-medium">{check.label}</p>
            <p className="text-muted-foreground truncate text-xs">{check.meaning}</p>
          </div>

          <span
            className={cn(
              "tabular shrink-0 text-sm font-semibold",
              check.passed
                ? "text-emerald-600 dark:text-emerald-500"
                : "text-destructive",
            )}
          >
            {formatResidual(check.residual)}
          </span>
        </div>
      ))}

      <p className="text-muted-foreground text-xs">
        Calculado en el navegador a partir de la respuesta. El epsilon de la máquina en doble
        precisión ronda 2.2e-16, así que residuos de ese orden son el resultado esperado.
      </p>
    </div>
  );
}
