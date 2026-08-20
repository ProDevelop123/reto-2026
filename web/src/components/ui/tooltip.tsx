import * as React from "react";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";

import { cn } from "@/lib/utils";

function TooltipProvider({
  delayDuration = 0,
  ...props
}: React.ComponentProps<typeof TooltipPrimitive.Provider>) {
  return (
    <TooltipPrimitive.Provider
      data-slot="tooltip-provider"
      delayDuration={delayDuration}
      {...props}
    />
  );
}

function Tooltip({
  ...props
}: React.ComponentProps<typeof TooltipPrimitive.Root>) {
  return (
    <TooltipProvider>
      <TooltipPrimitive.Root data-slot="tooltip" {...props} />
    </TooltipProvider>
  );
}

function TooltipTrigger({
  ...props
}: React.ComponentProps<typeof TooltipPrimitive.Trigger>) {
  return <TooltipPrimitive.Trigger data-slot="tooltip-trigger" {...props} />;
}

// 1. Definimos una interfaz que extiende la de Radix
interface CustomTooltipContentProps extends React.ComponentPropsWithoutRef<
  typeof TooltipPrimitive.Content
> {
  classArrow?: string; // 👈 Aquí declaramos tu propiedad personalizada
}

function TooltipContent({
  className,
  classArrow, // 👈 Ahora TS ya sabe qué es esto
  sideOffset = 0,
  children,
  ...props
}: CustomTooltipContentProps) {
  // 👈 Usamos nuestra nueva interfaz
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Content
        data-slot="tooltip-content"
        sideOffset={sideOffset}
        className={cn(
          "bg-foreground text-background animate-in fade-in-0 zoom-in-95 z-50 w-fit rounded-md px-3 py-1.5 text-xs",
          className,
        )}
        {...props}
      >
        {children}
        <TooltipPrimitive.Arrow
          className={cn(
            // Importante: quitamos 'bg-foreground' de aquí para que
            // 'classArrow' pueda controlar el color si quieres
            "fill-current z-50 size-2.5",
            classArrow,
          )}
        />
      </TooltipPrimitive.Content>
    </TooltipPrimitive.Portal>
  );
}

export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider };
