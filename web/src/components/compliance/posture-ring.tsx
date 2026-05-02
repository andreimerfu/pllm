import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { InfoTip } from "@/components/governance/info-tip";

type Tone = "emerald" | "amber" | "rose";

function toneFor(score: number): Tone {
  if (score >= 85) return "emerald";
  if (score >= 60) return "amber";
  return "rose";
}

const ringText: Record<Tone, string> = {
  emerald: "text-emerald-500",
  amber: "text-amber-500",
  rose: "text-rose-500",
};

const labelText: Record<Tone, string> = {
  emerald: "Healthy",
  amber: "Watch",
  rose: "At risk",
};

export function PostureRing({ score }: { score: number }) {
  const tone = toneFor(score);
  const r = 58;
  const c = 2 * Math.PI * r;
  const offset = c - (Math.max(0, Math.min(100, score)) / 100) * c;

  return (
    <InfoTip
      label="Composite score: 50% enforcement coverage + 30% 24h denial pressure + 20% webhook delivery health. ≥85 healthy · 60–84 watch · <60 at risk."
      side="right"
    >
      <div className="relative w-36 h-36 shrink-0 cursor-help">
        <svg viewBox="0 0 140 140" className="w-full h-full -rotate-90">
          <circle
            cx="70"
            cy="70"
            r={r}
            stroke="currentColor"
            strokeWidth="8"
            fill="none"
            className="text-muted/50"
          />
          <circle
            cx="70"
            cy="70"
            r={r}
            stroke="currentColor"
            strokeWidth="8"
            fill="none"
            strokeDasharray={c}
            strokeDashoffset={offset}
            strokeLinecap="round"
            className={cn(ringText[tone], "transition-all duration-700 ease-out")}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground">
            Posture
          </span>
          <span className={cn("text-4xl font-bold tabular-nums leading-none mt-1", ringText[tone])}>
            {score}
          </span>
          <span className="text-[10px] text-muted-foreground mt-0.5">{labelText[tone]}</span>
        </div>
      </div>
    </InfoTip>
  );
}

type Breakdown = {
  label: string;
  value: number;
  hint?: string;
  tip?: string;
};

export function PostureBreakdown({ items }: { items: Breakdown[] }) {
  return (
    <div className="flex flex-col gap-2 flex-1 min-w-0">
      {items.map((it) => {
        const tone = toneFor(it.value);
        const labelEl = (
          <span
            className={cn(
              "text-[11px] text-foreground font-medium truncate",
              it.tip && "decoration-dotted decoration-muted-foreground/50 underline underline-offset-4 cursor-help",
            )}
          >
            {it.label}
          </span>
        );
        return (
          <div key={it.label} className="space-y-1">
            <div className="flex items-baseline justify-between gap-2">
              <div className="flex items-center gap-1.5 min-w-0">
                <Icon
                  icon={
                    tone === "emerald"
                      ? icons.check
                      : tone === "amber"
                        ? icons.warning
                        : icons.error
                  }
                  className={cn("h-3 w-3 shrink-0", ringText[tone])}
                />
                {it.tip ? <InfoTip label={it.tip}>{labelEl}</InfoTip> : labelEl}
                {it.hint && (
                  <span className="text-[10px] text-muted-foreground truncate">
                    · {it.hint}
                  </span>
                )}
              </div>
              <span className={cn("text-[12px] font-mono tabular-nums font-semibold", ringText[tone])}>
                {it.value}
              </span>
            </div>
            <div className="h-1 w-full bg-muted/60 rounded-full overflow-hidden">
              <div
                className={cn(
                  "h-full rounded-full transition-all",
                  tone === "emerald" && "bg-emerald-500",
                  tone === "amber" && "bg-amber-500",
                  tone === "rose" && "bg-rose-500",
                )}
                style={{ width: `${Math.max(0, Math.min(100, it.value))}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
