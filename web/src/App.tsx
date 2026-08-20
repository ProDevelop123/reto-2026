import { RouterProvider } from "react-router-dom";
import { ThemeProvider } from "next-themes";

import { TooltipProvider } from "@/components/ui/tooltip";

import { router } from "./app/router";

export default function App() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      {/* Un unico proveedor de tooltips en la raiz: montarlo por componente
          duplicaria listeners sin aportar nada. */}
      <TooltipProvider delayDuration={200}>
        <RouterProvider router={router} />
      </TooltipProvider>
    </ThemeProvider>
  );
}
