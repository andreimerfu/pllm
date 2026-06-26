import { useMemo } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

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
    <div className="flex h-full min-h-0 flex-col">
      <header className="px-1 pb-4">
        <h1 className="text-2xl font-semibold tracking-tight">Playground</h1>
        <p className="mt-1 text-[13px] text-muted-foreground">
          Try any primitive — a prompt, agent, MCP server, or use case — before wiring it up.
        </p>
        <div className="mt-4">
          <ModeTabs current={mode} onChange={setMode} />
        </div>
      </header>
      <div className="min-h-0 flex-1 overflow-auto">
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
    <div className="inline-flex items-center gap-0.5 rounded-[10px] border border-border bg-surface-2 p-1">
      {MODES.map((m) => {
        const active = m.id === current;
        return (
          <button
            key={m.id}
            onClick={() => onChange(m.id)}
            className={cn(
              "h-8 rounded-md px-3.5 text-[13px] font-medium transition-colors",
              active
                ? "bg-card text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
            title={m.description}
          >
            {m.label}
          </button>
        );
      })}
    </div>
  );
}
