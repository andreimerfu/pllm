import { useState } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { formatDistanceToNow } from "date-fns";

// Uniform shape for any registry kind. Version row has a name we can match
// back against `target.version` and a published date for readable ordering.
export type VersionItem = {
  version: string;
  is_latest?: boolean;
  status?: string;
  published_at?: string;
};

// Popover dropdown so picking an older version doesn't require leaving the
// drawer. Matches the visual weight of a badge rather than a button.
export function VersionPicker({
  versions,
  selected,
  onSelect,
  loading,
}: {
  versions: VersionItem[];
  selected: string | undefined;
  onSelect: (v: string) => void;
  loading?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const selectedRow = versions.find((v) => v.version === selected);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={cn(
            "inline-flex items-center gap-1.5 h-6 px-2 rounded-md",
            "bg-foreground/5 hover:bg-foreground/10 transition-colors",
            "ring-1 ring-border/60",
          )}
        >
          <span className="text-xs font-mono tabular-nums">
            v{selected ?? "…"}
          </span>
          {selectedRow?.is_latest && (
            <span className="text-[9px] font-mono uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
              latest
            </span>
          )}
          <Icon icon={icons.chevronDown} className="h-3 w-3 text-muted-foreground" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-0" align="start">
        <div className="py-1 max-h-80 overflow-y-auto">
          {loading && (
            <div className="py-8 text-center">
              <div className="inline-block animate-spin rounded-full h-5 w-5 border-b-2 border-primary" />
            </div>
          )}
          {!loading && versions.length === 0 && (
            <p className="px-3 py-4 text-xs text-muted-foreground text-center">
              No version history
            </p>
          )}
          {!loading &&
            versions.map((v) => {
              const active = v.version === selected;
              return (
                <button
                  key={v.version}
                  onClick={() => {
                    onSelect(v.version);
                    setOpen(false);
                  }}
                  className={cn(
                    "flex items-center justify-between w-full px-3 py-2 text-left text-xs",
                    "hover:bg-muted/60 transition-colors",
                    active && "bg-muted/80",
                  )}
                >
                  <span className="flex items-center gap-2 min-w-0">
                    <span className="font-mono tabular-nums">v{v.version}</span>
                    {v.is_latest && (
                      <span className="text-[9px] font-mono uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
                        latest
                      </span>
                    )}
                    {v.status === "deprecated" && (
                      <span className="text-[9px] font-mono uppercase tracking-widest text-amber-600 dark:text-amber-400">
                        deprecated
                      </span>
                    )}
                  </span>
                  {v.published_at && (
                    <span className="text-[10px] text-muted-foreground shrink-0 ml-2">
                      {formatDistanceToNow(new Date(v.published_at), { addSuffix: true })}
                    </span>
                  )}
                </button>
              );
            })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
