import { useMemo, useState } from "react";
import { AlertCircle, Cpu, Globe, MonitorCheck } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { toApiError } from "@/shared/lib/api";

import { MatrixInput } from "../components/MatrixInput";
import { MatrixView } from "../components/MatrixView";
import { PayloadPanel } from "../components/PayloadPanel";
import { StatisticsPanel } from "../components/StatisticsPanel";
import { VerificationPanel } from "../components/VerificationPanel";
import { DEFAULT_PRESET_ID, PRESETS } from "../lib/presets";
import { verifyFactorization } from "../lib/verification";
import { factorize } from "../services/qr.service";
import type {
  FactorizationMode,
  FactorizeRequest,
  FactorizeResponse,
} from "../types/qr.types";

/**
 * Etiqueta de procedencia.
 *
 * Cada bloque del resultado indica qué componente del sistema lo produjo. No es
 * adorno: documenta la arquitectura visualmente. Quien evalúa ve de un vistazo
 * que la factorización la calcula Go, que las estadísticas vienen del servicio
 * en Node, y que los residuos los computa el propio navegador.
 */
function Origin({ icon: Icon, children }: { icon: typeof Cpu; children: string }) {
  return (
    <span className="text-muted-foreground flex items-center gap-1.5 text-xs font-normal">
      <Icon className="size-3.5" />
      {children}
    </span>
  );
}

const defaultPreset = PRESETS.find((p) => p.id === DEFAULT_PRESET_ID) ?? PRESETS[0];

export function WorkspacePage() {
  // Se arranca con un ejemplo ya cargado para que la primera factorización
  // cueste un solo clic.
  const [matrix, setMatrix] = useState<number[][]>(() =>
    defaultPreset.matrix.map((row) => [...row]),
  );
  const [mode, setMode] = useState<FactorizationMode>("reduced");
  const [tolerance, setTolerance] = useState("");

  const [result, setResult] = useState<FactorizeResponse | null>(null);
  const [sentRequest, setSentRequest] = useState<FactorizeRequest | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    setLoading(true);
    setError(null);

    const parsedTolerance = tolerance.trim() === "" ? undefined : Number(tolerance);

    const request: FactorizeRequest = {
      matrix,
      mode,
      // Se omite si no es un número válido, para que la API aplique su valor
      // por defecto en lugar de recibir NaN.
      ...(parsedTolerance !== undefined && Number.isFinite(parsedTolerance)
        ? { tolerance: parsedTolerance }
        : {}),
    };

    try {
      const response = await factorize(request);
      setResult(response);
      setSentRequest(request);
    } catch (caught) {
      const apiError = toApiError(caught);
      setError(apiError.message);
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  /**
   * Verificación de las identidades de la factorización.
   *
   * Se memoriza porque implica productos de matrices y solo debe recalcularse
   * cuando llega un resultado nuevo, no en cada renderizado.
   */
  const checks = useMemo(() => {
    if (!result) return null;
    return verifyFactorization(result.data.matrix.data, result.data.q.data, result.data.r.data);
  }, [result]);

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)]">
      {/* --- Entrada --- */}
      <Card className="h-fit lg:sticky lg:top-6">
        <CardHeader>
          <CardTitle>Matriz de entrada</CardTitle>
        </CardHeader>
        <CardContent>
          <MatrixInput
            matrix={matrix}
            onMatrixChange={setMatrix}
            mode={mode}
            onModeChange={setMode}
            tolerance={tolerance}
            onToleranceChange={setTolerance}
            onSubmit={() => void handleSubmit()}
            loading={loading}
          />
        </CardContent>
      </Card>

      {/* --- Resultado --- */}
      <div className="space-y-6">
        {error ? (
          <div className="border-destructive/40 bg-destructive/5 text-destructive flex items-start gap-3 rounded-lg border p-4 text-sm">
            <AlertCircle className="mt-0.5 size-4 shrink-0" />
            <p>{error}</p>
          </div>
        ) : null}

        {!result && !error ? (
          <Card>
            <CardContent className="text-muted-foreground py-16 text-center text-sm">
              Pulsa <span className="text-foreground font-medium">Factorizar</span> para
              descomponer la matriz.
              <br />
              Ya hay un ejemplo cargado: no hace falta escribir nada.
            </CardContent>
          </Card>
        ) : null}

        {result && checks ? (
          <>
            {/* Factorización — API en Go */}
            <Card>
              <CardHeader>
                <CardTitle className="flex flex-wrap items-center justify-between gap-2">
                  Factorización QR
                  <Origin icon={Cpu}>API en Go · Householder</Origin>
                </CardTitle>
              </CardHeader>
              <CardContent className="flex flex-wrap items-start gap-x-10 gap-y-6">
                <MatrixView matrix={result.data.matrix} caption="original" />
                <MatrixView matrix={result.data.q} caption="ortonormal" />
                <MatrixView
                  matrix={result.data.r}
                  caption="triangular superior"
                  dimStructuralZeros
                />
              </CardContent>
            </Card>

            {/* Estadísticas — API en Node */}
            <Card>
              <CardHeader>
                <CardTitle className="flex flex-wrap items-center justify-between gap-2">
                  Estadísticas
                  <Origin icon={Globe}>API en Node · vía la API en Go</Origin>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <StatisticsPanel statistics={result.data.statistics} />
              </CardContent>
            </Card>

            {/* Verificación — navegador */}
            <Card>
              <CardHeader>
                <CardTitle className="flex flex-wrap items-center justify-between gap-2">
                  Verificación independiente
                  <Origin icon={MonitorCheck}>calculado en el navegador</Origin>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <VerificationPanel checks={checks} />
              </CardContent>
            </Card>

            {/* Metadatos y payload */}
            <Card>
              <CardContent className="space-y-4">
                <dl className="tabular text-muted-foreground flex flex-wrap gap-x-6 gap-y-2 text-xs">
                  <div>
                    <dt className="inline font-medium">Modo:</dt>{" "}
                    <dd className="inline">{result.metadata.mode}</dd>
                  </div>
                  <div>
                    <dt className="inline font-medium">Reflexiones:</dt>{" "}
                    <dd className="inline">{result.metadata.reflectors}</dd>
                  </div>
                  <div>
                    <dt className="inline font-medium">Tolerancia:</dt>{" "}
                    <dd className="inline">{result.metadata.tolerance}</dd>
                  </div>
                  <div>
                    <dt className="inline font-medium">Duración:</dt>{" "}
                    <dd className="inline">{result.metadata.durationMs} ms</dd>
                  </div>
                </dl>

                <Separator />

                <PayloadPanel request={sentRequest} response={result} />
              </CardContent>
            </Card>
          </>
        ) : null}
      </div>
    </div>
  );
}
