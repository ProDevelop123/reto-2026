import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center justify-center rounded-full border px-2 py-0.5 text-xs font-medium w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive transition-[color,box-shadow] overflow-hidden",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-primary text-primary-foreground [a&]:hover:bg-primary/90",
        secondary:
          "bg-blue-50 text-blue-600 border border-blue-200 dark:bg-blue-500/15 dark:text-blue-300 dark:border-blue-500/30",
        destructive:
          "border-transparent bg-destructive text-white [a&]:hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 dark:bg-destructive/60",
        outline:
          "inline-flex rounded-full bg-gray-50 px-2 py-0.5 text-theme-xs font-medium text-gray-500 dark:bg-gray-500/15 dark:text-gray-400 dark:border-gray-700",
        serie:
          "dark:bg-white/20 bg-gray-200 text-slate-800 dark:text-white hover:bg-primary/20 dark:hover:bg-white/10 select-none uppercase font-semibold",
        old: "border-transparent bg-red-100 text-red-700 line-through decoration-red-500/50 dark:bg-red-900/30 dark:text-red-400 dark:border-red-900/50",
        new: "border-transparent bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300 dark:border-blue-900/50 font-semibold",
        active:
          "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20",
        inactive:
          "bg-zinc-500/10 text-zinc-500 dark:text-zinc-400 border border-zinc-500/20",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

function Badge({
  className,
  variant,
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "span";

  return (
    <Comp
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export { Badge, badgeVariants };
