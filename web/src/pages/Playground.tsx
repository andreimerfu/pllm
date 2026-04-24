import { useMemo } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import Chat from "@/pages/Chat";
import { McpMode } from "@/components/playground/mcp-mode";
import { UseCaseMode } from "@/components/playground/usecase-mode";
import { AgentMode } from "@/components/playground/agent-mode";

type Mode = "prompt" | "agent" | "mcp" | "usecase";

const MODES: {
  id: Mode;
  label: string;
  icon: string;
  description: string;
}[] = [
  {
    id: "prompt",
    label: "Prompt",
    icon: icons.chat,
    description: "Plain chat against any model or route",
  },
  {
    id: "agent",
    label: "Agent",
    icon: icons.agent,
    description: "Run a registry agent end-to-end with its tools",
  },
  {
    id: "mcp",
    label: "MCP",
    icon: icons.mcp,
    description: "Call any MCP tool directly",
  },
  {
    id: "usecase",
    label: "Use case",
    icon: icons.useCase,
    description: "Chat under a bound use case's scope",
  },
];

function isMode(v: string | null): v is Mode {
  return v === "prompt" || v === "agent" || v === "mcp" || v === "usecase";
}

export default function Playground() {
  const [params, setParams] = useSearchParams();
  const raw = params.get("mode");
  const mode: Mode = useMemo(() => (isMode(raw) ? raw : "prompt"), [raw]);
  const navigate = useNavigate();

  const setMode = (m: Mode) => {
    const next = new URLSearchParams(params);
    next.set("mode", m);
    setParams(next, { replace: true });
  };

  return (
    <div className="flex flex-col h-full min-h-0">
      <ModeTabs current={mode} onChange={setMode} />
      <div className="flex-1 min-h-0 overflow-hidden">
        {mode === "prompt" && <Chat />}
        {mode === "agent" && <AgentMode onOpenRegistry={() => navigate("/registry")} />}
        {mode === "mcp" && <McpMode />}
        {mode === "usecase" && (
          <UseCaseMode onOpenUseCases={() => navigate("/use-cases")} />
        )}
      </div>
    </div>
  );
}

function ModeTabs({
  current,
  onChange,
}: {
  current: Mode;
  onChange: (m: Mode) => void;
}) {
  return (
    <div className="border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="px-4 pt-3">
        <div className="flex items-baseline justify-between gap-4 mb-2">
          <div>
            <h1 className="text-xl font-semibold tracking-tight leading-none">
              Playground
            </h1>
            <p className="text-[11px] text-muted-foreground mt-1">
              Try any primitive before wiring it up.
            </p>
          </div>
        </div>
        <nav className="flex items-center gap-1 -mb-px">
          {MODES.map((m) => {
            const active = m.id === current;
            return (
              <button
                key={m.id}
                onClick={() => onChange(m.id)}
                className={cn(
                  "group relative flex items-center gap-2 px-3 py-2 text-[13px] font-medium transition-colors",
                  "border-b-2",
                  active
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground",
                )}
                title={m.description}
              >
                <Icon
                  icon={m.icon}
                  className={cn(
                    "h-4 w-4 transition-colors",
                    active ? "text-primary" : "text-muted-foreground/70",
                  )}
                />
                {m.label}
              </button>
            );
          })}
        </nav>
      </div>
    </div>
  );
}
