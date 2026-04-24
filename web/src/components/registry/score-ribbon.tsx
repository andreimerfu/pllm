import { cn } from "@/lib/utils";
import type { EnrichmentScore } from "@/types/api";

// Score bucket → color. Reserved colors so users learn them fast.
function toneFor(score: number) {
  if (score >= 80) return "emerald";
  if (score >= 50) return "amber";
  return "rose";
}

const TONE_CLASSES = {
  emerald: {
    bar: "bg-emerald-500",
    text: "text-emerald-700 dark:text-emerald-300",
    track: "bg-emerald-500/10",
  },
  amber: {
    bar: "bg-amber-500",
    text: "text-amber-700 dark:text-amber-300",
    track: "bg-amber-500/10",
  },
  rose: {
    bar: "bg-rose-500",
    text: "text-rose-700 dark:text-rose-300",
    track: "bg-rose-500/10",
  },
} as const;

// Inline ribbon: score on the left, a tiny bar rail on the right.
// Used inside card footers to glanceably communicate "how trustworthy is this?"
export function ScoreChip({
  label,
  score,
  compact = false,
}: {
  label?: string;
  score: number;
  compact?: boolean;
}) {
  const tone = TONE_CLASSES[toneFor(score)];
  const pct = Math.max(0, Math.min(100, score));
  return (
    <div className="flex items-center gap-2 min-w-0">
      {label && (
        <span className="text-[10px] uppercase tracking-widest text-muted-foreground font-mono">
          {label}
        </span>
      )}
      <div className={cn("relative h-1.5 rounded-full overflow-hidden", tone.track, compact ? "w-12" : "w-20")}>
        <div
          className={cn("absolute inset-y-0 left-0 rounded-full", tone.bar)}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className={cn("text-xs font-semibold tabular-nums", tone.text)}>
        {Math.round(score)}
      </span>
    </div>
  );
}

// ScoresPanel is the detail-view expansion. Lists every scanner output
// (OSV, Scorecard, etc.) with its own bar and summary.
export function ScoresPanel({ scores }: { scores: EnrichmentScore[] }) {
  if (!scores.length) return null;
  return (
    <div className="space-y-2">
      <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        Enrichment
      </p>
      <div className="space-y-1.5">
        {scores.map((s) => {
          const tone = TONE_CLASSES[toneFor(s.score)];
          const pct = Math.max(0, Math.min(100, s.score));
          return (
            <div key={s.id} className="flex items-center gap-3 py-1.5">
              <span
                className={cn(
                  "text-[10px] font-mono uppercase tracking-wider px-1.5 py-0.5 rounded",
                  tone.track,
                  tone.text,
                )}
              >
                {s.type}
              </span>
              <div className={cn("relative flex-1 h-1.5 rounded-full overflow-hidden", tone.track)}>
                <div className={cn("absolute inset-y-0 left-0", tone.bar)} style={{ width: `${pct}%` }} />
              </div>
              <span className={cn("text-sm font-semibold tabular-nums shrink-0", tone.text)}>
                {Math.round(s.score)}
              </span>
              {s.summary && (
                <span className="text-xs text-muted-foreground truncate shrink-0 max-w-[14rem]">
                  {s.summary}
                </span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
