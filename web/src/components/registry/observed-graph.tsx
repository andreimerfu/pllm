import { useMemo } from "react";
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  ReactFlowProvider,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import { Icon } from "@iconify/react";
import { detectProvider } from "@/lib/providers";
import type { ObservedAgentCall } from "@/types/api";

// Visual vocabulary for observed-call nodes. Edge color mirrors the
// runtime outcome: emerald = allowed, amber = flagged, rose = denied.

type NodeKind = "agent_root" | "llm" | "mcp" | "vector_db" | "agent" | "http" | "unknown";

interface ObservedGraphNodeData {
  kind: NodeKind;
  label: string;
  sublabel?: string;
  count?: number;
  outcome?: string;
  icon?: string;
}

// Edge styling per outcome. Solid emerald for allowed; dashed for the
// can't-block cases (flagged/denied).
const OUTCOME_STROKE: Record<string, string> = {
  allowed: "#10b981",
  flagged: "#f59e0b",
  blocked: "#f43f5e",
  denied: "#f43f5e",
};

const NODE_TONE: Record<NodeKind, { ring: string; fill: string; iconClass: string }> = {
  agent_root: { ring: "ring-primary/40", fill: "bg-primary/10", iconClass: "text-primary" },
  llm:        { ring: "ring-indigo-300", fill: "bg-indigo-50 dark:bg-indigo-950/40", iconClass: "text-indigo-600" },
  mcp:        { ring: "ring-emerald-300", fill: "bg-emerald-50 dark:bg-emerald-950/40", iconClass: "text-emerald-600" },
  vector_db:  { ring: "ring-amber-300", fill: "bg-amber-50 dark:bg-amber-950/40", iconClass: "text-amber-600" },
  agent:      { ring: "ring-sky-300", fill: "bg-sky-50 dark:bg-sky-950/40", iconClass: "text-sky-600" },
  http:       { ring: "ring-slate-300", fill: "bg-slate-50 dark:bg-slate-900/40", iconClass: "text-slate-600" },
  unknown:    { ring: "ring-violet-300", fill: "bg-violet-50 dark:bg-violet-950/40", iconClass: "text-violet-600" },
};

const KIND_ICON: Record<NodeKind, string> = {
  agent_root: "solar:user-rounded-bold",
  llm: "solar:cpu-bolt-linear",
  mcp: "lucide:plug-zap",
  vector_db: "solar:database-linear",
  agent: "solar:users-group-rounded-linear",
  http: "solar:global-linear",
  unknown: "solar:question-circle-linear",
};

function vectorDbIcon(target: string): string | undefined {
  const t = target.toLowerCase();
  if (t.startsWith("qdrant")) return "logos:qdrant-icon";
  if (t.startsWith("pinecone")) return "simple-icons:pinecone";
  if (t.startsWith("weaviate")) return "simple-icons:weaviate";
  if (t.startsWith("chroma")) return "logos:chroma";
  return undefined;
}

function pickIcon(kind: NodeKind, target: string): string {
  if (kind === "llm") {
    const p = detectProvider(target, target);
    return p.icon;
  }
  if (kind === "vector_db") {
    return vectorDbIcon(target) ?? KIND_ICON.vector_db;
  }
  return KIND_ICON[kind];
}

// Custom node renderer. Pill-style with iconified header; the agent_root
// gets a slightly bigger ring to anchor the visual hierarchy.
function ObservedNode({ data }: NodeProps) {
  const d = data as unknown as ObservedGraphNodeData;
  const tone = NODE_TONE[d.kind];
  const isRoot = d.kind === "agent_root";
  return (
    <div
      className={`rounded-md border px-3 py-2 text-xs ${tone.fill} ring-1 ${tone.ring} ${
        isRoot ? "ring-2 shadow-md" : ""
      }`}
    >
      <Handle type="target" position={Position.Left} className="!bg-transparent !border-0" />
      <div className="flex items-center gap-2">
        <Icon icon={d.icon ?? KIND_ICON[d.kind]} className={`h-4 w-4 ${tone.iconClass}`} />
        <div className="min-w-0">
          <div className="font-mono text-[11px] truncate max-w-[200px]" title={d.label}>
            {d.label}
          </div>
          {d.sublabel && (
            <div className="text-[9px] text-muted-foreground truncate max-w-[200px]">
              {d.sublabel}
              {d.count != null ? ` · ×${d.count}` : ""}
            </div>
          )}
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!bg-transparent !border-0" />
    </div>
  );
}

const NODE_TYPES = { observed: ObservedNode };

interface Props {
  agentName: string;
  calls: ObservedAgentCall[];
}

// Lay out the agent on the left, observed targets stacked on the right.
// One row per unique target; vertical spacing scales to keep dense graphs
// legible without React Flow's auto-layout overhead.
export function ObservedGraph({ agentName, calls }: Props) {
  const { nodes, edges } = useMemo(() => {
    const rootId = "agent-root";
    const ns: Node[] = [
      {
        id: rootId,
        type: "observed",
        position: { x: 0, y: 0 },
        data: {
          kind: "agent_root",
          label: agentName,
          sublabel: "this agent",
          icon: KIND_ICON.agent_root,
        } as unknown as Record<string, unknown>,
      },
    ];
    const es: Edge[] = [];
    const Y_STEP = 76;
    const X_TARGET = 320;

    calls.forEach((c, i) => {
      const id = `call-${i}`;
      const kind = (c.kind as NodeKind);
      const stroke = OUTCOME_STROKE[c.last_outcome ?? "allowed"] ?? "#94a3b8";
      ns.push({
        id,
        type: "observed",
        position: { x: X_TARGET, y: i * Y_STEP - ((calls.length - 1) * Y_STEP) / 2 },
        data: {
          kind,
          label: c.target_name,
          sublabel: c.last_outcome ?? undefined,
          count: c.count,
          icon: pickIcon(kind, c.target_name),
        } as unknown as Record<string, unknown>,
      });
      es.push({
        id: `e-${id}`,
        source: rootId,
        target: id,
        animated: c.last_outcome !== "allowed" && c.last_outcome !== "" && c.last_outcome != null,
        style: {
          stroke,
          strokeWidth: 1.6,
          strokeDasharray:
            c.last_outcome !== "allowed" && c.last_outcome ? "5,4" : undefined,
        },
        markerEnd: { type: MarkerType.ArrowClosed, color: stroke },
        label: c.last_outcome,
        labelStyle: { fontSize: 9, fontFamily: "monospace", fill: stroke },
        labelBgStyle: { fill: "white", fillOpacity: 0.85 },
      });
    });
    return { nodes: ns, edges: es };
  }, [agentName, calls]);

  if (calls.length === 0) {
    return null;
  }

  const height = Math.max(220, calls.length * 76 + 60);

  return (
    <div className="rounded border border-border/60" style={{ height }}>
      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={NODE_TYPES}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          proOptions={{ hideAttribution: true }}
          panOnScroll={false}
          zoomOnScroll={false}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
        >
          <Background gap={16} size={1} className="opacity-40" />
          <Controls showInteractive={false} className="!shadow-none" />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
}
