import { useState } from "react";
import { ChevronRight, Copy } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface PayloadPanelProps {
  request: unknown;
  response: unknown;
}

function JsonBlock({ title, value }: { title: string; value: unknown }) {
  const text = JSON.stringify(value, null, 2);

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-xs font-medium">{title}</p>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-6 px-2 text-xs"
          onClick={() => void navigator.clipboard.writeText(text)}
        >
          <Copy className="size-3" />
          Copiar
        </Button>
      </div>
      <pre className="bg-muted/50 max-h-64 overflow-auto rounded-md border p-3 text-xs leading-relaxed">
        <code>{text}</code>
      </pre>
    </div>
  );
}

/**
 * Petición y respuesta HTTP en crudo.
 *
 * Existe para la comodidad de quien revisa: permite ver el contrato exacto de la
 * API sin abrir las herramientas de desarrollo ni un cliente HTTP aparte.
 *
 * Es de solo lectura a propósito. Hacerlo editable obligaría a sincronizar en
 * ambos sentidos el texto JSON con la cuadrícula, y a decidir qué hacer cuando
 * alguien pega una matriz de 200×200 que el grid no puede representar. El
 * beneficio —ver el contrato— se obtiene igual sin asumir esa complejidad.
 */
export function PayloadPanel({ request, response }: PayloadPanelProps) {
  const [open, setOpen] = useState(false);

  return (
    <div className="rounded-lg border">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="hover:bg-muted/50 flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors"
        aria-expanded={open}
      >
        <ChevronRight className={cn("size-4 transition-transform", open && "rotate-90")} />
        Petición y respuesta HTTP
        <span className="text-muted-foreground ml-auto text-xs font-normal">
          POST /api/v1/qr
        </span>
      </button>

      {open ? (
        <div className="space-y-3 border-t p-3">
          <JsonBlock title="Petición" value={request} />
          <JsonBlock title="Respuesta" value={response} />
        </div>
      ) : null}
    </div>
  );
}
