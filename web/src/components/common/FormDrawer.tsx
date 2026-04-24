import * as React from "react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

/**
 * FormDrawer is the standard right-side drawer for create/edit forms.
 *
 * Dialogs (`<Dialog>`) are reserved for short confirmations; every
 * form of more than a few fields should live in this drawer so the
 * user keeps the list context behind them.
 *
 * Layout contract:
 *   ┌────────────────────────────────────────────┐
 *   │ Title                                      │  ← sticky header
 *   │ Description (optional)                     │
 *   ├────────────────────────────────────────────┤
 *   │                                            │
 *   │ form body (scrollable)                     │
 *   │                                            │
 *   ├────────────────────────────────────────────┤
 *   │                      [Cancel] [Primary]    │  ← sticky footer
 *   └────────────────────────────────────────────┘
 *
 * Pass the action buttons via `footer`, not inside children, so the
 * footer stays pinned while the body scrolls.
 */
export function FormDrawer({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  size = "md",
  className,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  size?: "sm" | "md" | "lg" | "xl";
  className?: string;
}) {
  const widthClass = {
    sm: "sm:max-w-md",
    md: "sm:max-w-lg",
    lg: "sm:max-w-2xl",
    xl: "sm:max-w-3xl",
  }[size];

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className={cn(
          "flex flex-col gap-0 p-0 w-full",
          widthClass,
          className,
        )}
      >
        <div className="border-b border-border px-6 py-4 shrink-0">
          <SheetTitle className="text-base font-semibold tracking-tight">
            {title}
          </SheetTitle>
          {description && (
            <SheetDescription className="text-[13px] mt-1">
              {description}
            </SheetDescription>
          )}
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>

        {footer && (
          <div className="border-t border-border px-6 py-3 shrink-0 flex items-center justify-end gap-2 bg-background/95">
            {footer}
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
