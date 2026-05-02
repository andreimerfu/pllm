import { type ReactNode } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

// Lightweight wrapper that bundles the Provider so any chip / label can
// adopt a tooltip with a single component swap. Keeps copy short — for
// multi-paragraph context, prefer <GovernanceHelp> instead.
export function InfoTip({
  label,
  children,
  side = "top",
  align = "center",
  delayDuration = 150,
  contentClassName,
}: {
  label: ReactNode;
  children: ReactNode;
  side?: "top" | "right" | "bottom" | "left";
  align?: "start" | "center" | "end";
  delayDuration?: number;
  contentClassName?: string;
}) {
  return (
    <TooltipProvider delayDuration={delayDuration}>
      <Tooltip>
        <TooltipTrigger asChild>{children}</TooltipTrigger>
        <TooltipContent
          side={side}
          align={align}
          className={cn("max-w-xs text-[12px] leading-relaxed", contentClassName)}
        >
          {label}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
