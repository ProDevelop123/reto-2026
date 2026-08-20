import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center select-none justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-all disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none shrink-0 [&_svg]:shrink-0 outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
  // [&_svg:not([class*='size-'])]:size-4
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground hover:bg-primary/90 dark:bg-brand-500 dark:hover:bg-brand-600",
        destructive:
          "bg-destructive text-white focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 dark:bg-destructive/60",
        outline:
          "text-gray-800 bg-white hover:bg-white shadow-xs hover:bg-accent hover:text-accent-foreground dark:text-gray-400 dark:bg-white/3 dark:hover:bg-white/1 border border-gray-200 dark:border-gray-800",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-secondary/80",
        ghost:
          "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
        link: "text-primary underline-offset-4 hover:underline",
        edit: "transition-colors border bg-blue-50/50 border-blue-300 text-blue-600 hover:bg-white hover:border-blue-400 hover:text-blue-700 active:bg-blue-200 active:border-blue-500 active:text-blue-800 dark:bg-blue-900/20 dark:border-blue-700/60 dark:text-blue-400 dark:hover:bg-blue-800/40 dark:hover:border-blue-500 dark:hover:text-blue-300 dark:active:bg-blue-800/60 dark:active:border-blue-400 dark:active:text-blue-200",
        delete:
          "transition-colors border bg-red-50/50 border-red-300 text-red-600 hover:bg-white hover:border-red-400 hover:text-red-600 active:bg-red-200 active:border-red-500 active:text-red-800 dark:bg-red-900/20 dark:border-red-700/60 dark:text-red-400 dark:hover:bg-red-800/40 dark:hover:border-red-500 dark:hover:text-red-300 dark:active:bg-red-800/60 dark:active:border-red-400 dark:active:text-red-200",
        detail:
          "transition-colors border bg-gray-100 border-gray-300 text-gray-600 hover:bg-gray-200 hover:border-gray-400 hover:text-gray-700 active:bg-gray-300 active:border-gray-500 active:text-gray-800 dark:bg-gray-700/30 dark:border-gray-600/60 dark:text-gray-400 dark:hover:bg-gray-700/60 dark:hover:border-gray-500 dark:hover:text-gray-300 dark:active:bg-gray-700/80 dark:active:border-gray-400 dark:active:text-gray-200",
        none: "bg-transparent",
      },
      size: {
        default: "h-8 xl:h-10 py-2.5 px-4 has-[>svg]:px-3",
        sm: "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
        lg: "h-10 rounded-md px-6 has-[>svg]:px-4",
        icon: "size-9",
        "icon-sm": "size-8",
        "icon-lg": "size-10",
        action: "size-7.5 rounded-sm p-1",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  }) {
  const Comp = asChild ? Slot : "button";

  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
