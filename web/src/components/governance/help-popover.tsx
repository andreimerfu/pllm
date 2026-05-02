import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

// One-stop concept primer for the Governance section. Lives behind a
// "?" icon button so it stays out of the way for operators who already
// know the model — but is one click away on every page so newcomers
// can self-onboard without leaving the screen.
//
// We keep the same component on Use Cases and Compliance so the mental
// model is taught once, in one shape. The `accent` prop just bolds the
// step that matches the current page.

type Step = "declare" | "enforce" | "audit";

const STEPS: { key: Step; eyebrow: string; title: string; body: string }[] = [
  {
    key: "declare",
    eyebrow: "1 · Declare",
    title: "Use case",
    body: "A business activity (e.g. customer-research). Declares the MCP servers, agents, LLMs, skills, and prompts it depends on, plus risk tier and data classifications.",
  },
  {
    key: "enforce",
    eyebrow: "2 · Enforce",
    title: "Runtime scope",
    body: "API keys are bound to a use case. Calls to anything outside that scope are either logged (flag mode) or refused (blocking).",
  },
  {
    key: "audit",
    eyebrow: "3 · Audit",
    title: "Compliance",
    body: "Every flagged or denied call lands in the scope-event log, rolls into the posture score, and can be pushed to Slack / SIEM via webhooks.",
  },
];

export function GovernanceHelp({
  accent,
  align = "end",
}: {
  accent?: Step;
  align?: "start" | "center" | "end";
}) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="h-9 gap-1.5"
          aria-label="What is this?"
        >
          <Icon icon={icons.info} className="h-4 w-4" />
          What is this?
        </Button>
      </PopoverTrigger>
      <PopoverContent align={align} className="w-[22rem] p-4">
        <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-3">
          How governance fits together
        </p>
        <ol className="space-y-3">
          {STEPS.map((s) => (
            <li
              key={s.key}
              className={cn(
                "rounded-md border p-3",
                accent === s.key
                  ? "border-primary/40 bg-primary/5"
                  : "border-border/60 bg-background/40",
              )}
            >
              <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                {s.eyebrow}
              </p>
              <p className="text-[13px] font-semibold mt-0.5">{s.title}</p>
              <p className="text-[12px] text-muted-foreground leading-relaxed mt-1">
                {s.body}
              </p>
            </li>
          ))}
        </ol>
        <div className="mt-3 pt-3 border-t border-border/60 space-y-1 text-[11px] text-muted-foreground">
          <p>
            <span className="font-mono uppercase tracking-wider text-foreground/80">Tier 1–4</span>
            {" — "}
            risk class. Tier 1 = customer-facing decisions; Tier 4 = sandbox.
          </p>
          <p>
            <span className="font-mono uppercase tracking-wider text-foreground/80">Blocking vs flag</span>
            {" — "}
            blocking refuses out-of-scope calls; flag mode logs them and lets the call through.
          </p>
        </div>
      </PopoverContent>
    </Popover>
  );
}
