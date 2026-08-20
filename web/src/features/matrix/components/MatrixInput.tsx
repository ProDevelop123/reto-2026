import { Dices, Play } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import {
  API_MAX_DIMENSION,
  MAX_GRID_DIMENSION,
  PRESETS,
  randomMatrix,
  resizeMatrix,
} from "../lib/presets";
import type { FactorizationMode } from "../types/qr.types";

interface MatrixInputProps {
  matrix: number[][];
  onMatrixChange: (matrix: number[][]) => void;
  mode: FactorizationMode;
  onModeChange: (mode: FactorizationMode) => void;
  tolerance: string;
  onToleranceChange: (tolerance: string) => void;
  onSubmit: () => void;
  loading: boolean;
}

/**
 * Panel de entrada: dimensiones, cuadricula editable, ejemplos y opciones.
 *
 * El objetivo de diseno es que la primera factorizacion cueste UN clic. Al
 * cargar ya hay una matriz puesta, de modo que quien evalua no tiene que
 * teclear nada para ver el sistema completo funcionando.
 */
export function MatrixInput({
  matrix,
  onMatrixChange,
  mode,
  onModeChange,
  tolerance,
  onToleranceChange,
  onSubmit,
  loading,
}: MatrixInputProps) {
  const rows = matrix.length;
  const columns = matrix[0]?.length ?? 0;

  const handleDimensionChange = (nextRows: number, nextColumns: number) => {
    const clamp = (value: number) => Math.min(Math.max(value, 1), MAX_GRID_DIMENSION);
    // Redimensionar CONSERVA lo escrito: cambiar de 3x3 a 5x3 mantiene las
    // nueve celdas anteriores. Perder los datos por tocar un selector seria
    // exasperante.
    onMatrixChange(resizeMatrix(matrix, clamp(nextRows), clamp(nextColumns)));
  };

  const handleCellChange = (row: number, column: number, raw: string) => {
    const next = matrix.map((r) => [...r]);
    // Una celda vacia o a medio escribir ("-", "1.") se trata como cero en el
    // modelo, pero el campo conserva el texto del usuario mientras escribe.
    next[row][column] = raw === "" || raw === "-" ? 0 : Number(raw);
    onMatrixChange(next);
  };

  return (
    <div className="space-y-5">
      {/* --- Dimensiones --- */}
      <div className="flex flex-wrap items-end gap-4">
        <div className="space-y-1.5">
          <Label htmlFor="rows">Filas</Label>
          <Input
            id="rows"
            type="number"
            min={1}
            max={MAX_GRID_DIMENSION}
            value={rows}
            onChange={(e) => handleDimensionChange(Number(e.target.value), columns)}
            className="tabular w-20"
          />
        </div>

        <span className="text-muted-foreground pb-2 text-lg">×</span>

        <div className="space-y-1.5">
          <Label htmlFor="columns">Columnas</Label>
          <Input
            id="columns"
            type="number"
            min={1}
            max={MAX_GRID_DIMENSION}
            value={columns}
            onChange={(e) => handleDimensionChange(rows, Number(e.target.value))}
            className="tabular w-20"
          />
        </div>

        <Tooltip>
          <TooltipTrigger asChild>
            <p className="text-muted-foreground cursor-help pb-2 text-xs">
              máx {MAX_GRID_DIMENSION}×{MAX_GRID_DIMENSION}
            </p>
          </TooltipTrigger>
          <TooltipContent className="max-w-xs">
            Límite de esta interfaz, no de la API: la API admite hasta{" "}
            {API_MAX_DIMENSION}×{API_MAX_DIMENSION}, pero renderizar esa cuadrícula serían{" "}
            {(API_MAX_DIMENSION * API_MAX_DIMENSION).toLocaleString("es")} campos de entrada y
            el navegador se bloquearía.
          </TooltipContent>
        </Tooltip>
      </div>

      {/* --- Cuadricula editable --- */}
      <div className="overflow-x-auto">
        <div
          className="inline-grid gap-1"
          style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
        >
          {matrix.map((row, i) =>
            row.map((value, j) => (
              <Input
                key={`${i}-${j}`}
                type="number"
                value={value}
                onChange={(e) => handleCellChange(i, j, e.target.value)}
                aria-label={`Fila ${i + 1}, columna ${j + 1}`}
                className="tabular h-9 w-[4.5rem] px-2 text-center text-sm"
              />
            )),
          )}
        </div>
      </div>

      <Separator />

      {/* --- Ejemplos --- */}
      <div className="space-y-2">
        <Label>Ejemplos</Label>
        <div className="flex flex-wrap gap-2">
          {PRESETS.map((preset) => (
            <Tooltip key={preset.id}>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => onMatrixChange(preset.matrix.map((r) => [...r]))}
                >
                  {preset.label}
                </Button>
              </TooltipTrigger>
              <TooltipContent className="max-w-sm">{preset.description}</TooltipContent>
            </Tooltip>
          ))}

          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => onMatrixChange(randomMatrix(rows, columns))}
          >
            <Dices className="size-4" />
            Aleatoria
          </Button>
        </div>
        <p className="text-muted-foreground text-xs">
          El botón de aleatoria respeta las dimensiones actuales y genera enteros en [−20, 20].
        </p>
      </div>

      <Separator />

      {/* --- Opciones --- */}
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-2">
          <Label>Modo</Label>
          <RadioGroup
            value={mode}
            onValueChange={(value) => onModeChange(value as FactorizationMode)}
            className="gap-2"
          >
            <div className="flex items-center gap-2">
              <RadioGroupItem value="reduced" id="mode-reduced" />
              <Label htmlFor="mode-reduced" className="font-normal">
                Reducida
              </Label>
            </div>
            <div className="flex items-center gap-2">
              <RadioGroupItem value="complete" id="mode-complete" />
              <Label htmlFor="mode-complete" className="font-normal">
                Completa
              </Label>
            </div>
          </RadioGroup>
          <p className="text-muted-foreground text-xs">
            Reducida: Q de m×k y R de k×k con k = mín(m,n). Completa: Q ortogonal de m×m.
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="tolerance">Tolerancia</Label>
          <Input
            id="tolerance"
            value={tolerance}
            onChange={(e) => onToleranceChange(e.target.value)}
            placeholder="1e-9"
            className="tabular"
          />
          <p className="text-muted-foreground text-xs">
            Umbral bajo el cual un valor cuenta como cero al comprobar si una matriz es
            diagonal. Los ceros de una QR real son residuos de ~1e-16, no cero exacto.
          </p>
        </div>
      </div>

      <Button onClick={onSubmit} disabled={loading} className="w-full" size="lg">
        <Play className="size-4" />
        {loading ? "Factorizando…" : "Factorizar"}
      </Button>
    </div>
  );
}
