import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import { detectProvider } from "@/lib/providers";
import { getModelIcon } from "@/lib/chat-utils";
import { getModels, getRoutes } from "@/lib/api";
import type { ModelsResponse, RoutesResponse } from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

// ---------------------------------------------------------------------------
// Merged models + routes list. Routes are addressable by their slug in the
// /v1/chat/completions `model` field — the gateway resolves them as virtual
// endpoints that distribute across underlying models. We flag them with
// `owned_by: "route"` so the combobox can label and icon them distinctly.
// ---------------------------------------------------------------------------

export type ChatTarget = {
  id: string;
  name?: string;
  owned_by?: string;
  kind: "model" | "route";
};

export function useChatTargets() {
  return useQuery({
    queryKey: ["chat-targets"],
    queryFn: async (): Promise<ChatTarget[]> => {
      const [modelsRes, routesRes] = await Promise.all([
        getModels().catch(() => ({ data: [] } as unknown as ModelsResponse)),
        getRoutes().catch(() => ({ routes: [] } as unknown as RoutesResponse)),
      ]);
      const models: ChatTarget[] = (modelsRes.data ?? [])
        .filter((m) => m.id && m.id.trim() !== "")
        .map((m) => ({
          id: m.id,
          name: m.id,
          owned_by: m.owned_by,
          kind: "model" as const,
        }));
      const routes: ChatTarget[] = (routesRes.routes ?? [])
        .filter((r) => r.enabled)
        .map((r) => ({
          id: r.slug,
          name: r.name,
          owned_by: "route",
          kind: "route" as const,
        }));
      return [...models, ...routes];
    },
  });
}

// ---------------------------------------------------------------------------
// Combobox — same visual chrome as the Chat page's ModelCombobox, but reused.
// ---------------------------------------------------------------------------

export function TargetCombobox({
  targets,
  value,
  onChange,
  className,
  widthClass = "w-[220px]",
  placeholder = "Select a model…",
}: {
  targets: ChatTarget[];
  value: string;
  onChange: (id: string) => void;
  className?: string;
  widthClass?: string;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const selected = targets.find((t) => t.id === value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn("h-7 justify-between", widthClass, className)}
        >
          <div className="flex items-center gap-1.5 min-w-0">
            {selected ? (
              <>
                <Icon
                  icon={
                    selected.kind === "route"
                      ? icons.routes
                      : detectProvider(selected.id, selected.owned_by ?? "").icon
                  }
                  className={cn(
                    "h-3.5 w-3.5 shrink-0",
                    selected.kind === "route" && "text-primary",
                  )}
                />
                <span className="truncate text-xs font-mono">
                  {selected.name ?? selected.id}
                </span>
                {selected.kind === "route" && (
                  <span className="text-[9px] font-mono uppercase tracking-widest text-primary shrink-0">
                    route
                  </span>
                )}
              </>
            ) : (
              <span className="truncate text-xs text-muted-foreground">
                {placeholder}
              </span>
            )}
          </div>
          <Icon
            icon={icons.chevronsUpDown}
            className="ml-2 h-3 w-3 shrink-0 opacity-50"
          />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[340px] p-0" align="start">
        <Command>
          <CommandInput placeholder="Search models and routes…" className="h-9" />
          <CommandList>
            <CommandEmpty>No matching target.</CommandEmpty>
            <TargetGroup
              heading="Routes"
              items={targets.filter((t) => t.kind === "route")}
              value={value}
              onSelect={(id) => {
                onChange(id);
                setOpen(false);
              }}
              kind="route"
            />
            <TargetGroup
              heading="Models"
              items={targets.filter((t) => t.kind === "model")}
              value={value}
              onSelect={(id) => {
                onChange(id);
                setOpen(false);
              }}
              kind="model"
            />
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function TargetGroup({
  heading,
  items,
  value,
  onSelect,
  kind,
}: {
  heading: string;
  items: ChatTarget[];
  value: string;
  onSelect: (id: string) => void;
  kind: "model" | "route";
}) {
  if (items.length === 0) return null;
  return (
    <CommandGroup heading={heading}>
      {items.map((t) => (
        <CommandItem
          key={t.id}
          value={`${kind}:${t.id}:${t.name ?? ""}`}
          onSelect={() => onSelect(t.id)}
        >
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <Icon
              icon={
                kind === "route"
                  ? icons.routes
                  : getModelIcon(t.id)
              }
              className={cn(
                "h-4 w-4 shrink-0",
                kind === "route" && "text-primary",
              )}
            />
            <div className="flex flex-col min-w-0">
              <span className="text-sm font-medium truncate">
                {t.name ?? t.id}
              </span>
              {t.name && t.name !== t.id && (
                <span className="text-[11px] font-mono text-muted-foreground truncate">
                  {t.id}
                </span>
              )}
            </div>
          </div>
          <Icon
            icon={icons.check}
            className={cn(
              "ml-auto h-4 w-4",
              value === t.id ? "opacity-100" : "opacity-0",
            )}
          />
        </CommandItem>
      ))}
    </CommandGroup>
  );
}
