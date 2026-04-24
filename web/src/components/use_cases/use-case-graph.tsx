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
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";

import type { UseCaseWithRefs, UseCaseRef } from "@/types/api";

// Node kinds the graph knows how to render. UseCase is the central node;
// the rest are component kinds. Keeps a strict local type so we don't
// accidentally render an unsupported kind.
type GraphNodeKind = "use_case" | "server" | "agent" | "skill" | "prompt" | "llm";

type GraphNodeData = {
  kind: GraphNodeKind;
  label: string;
  subtitle?: string;
  version?: string;
  classifications?: string[];
  isInherited?: boolean;
  isUseCaseRoot?: boolean;
};

// Visual vocabulary — mirrors the kind-icon tint but tuned for nodes
// that live in a white canvas. Darker foreground, brighter borders.
const KIND_STYLES: Record<
  GraphNodeKind,
  { border: string; bg: string; fg: string; icon: string; label: string }
> = {
  use_case: {
    border: "border-primary",
    bg: "bg-primary/10",
    fg: "text-primary",
    icon: icons.useCase,
    label: "use case",
  },
  server: {
    border: "border-slate-500/60",
    bg: "bg-slate-500/10",
    fg: "text-slate-700 dark:text-slate-300",
    icon: icons.mcp,
    label: "mcp server",
  },
  agent: {
    border: "border-indigo-500/60",
    bg: "bg-indigo-500/10",
    fg: "text-indigo-700 dark:text-indigo-300",
    icon: icons.agent,
    label: "agent",
  },
  skill: {
    border: "border-emerald-500/60",
    bg: "bg-emerald-500/10",
    fg: "text-emerald-700 dark:text-emerald-300",
    icon: icons.skill,
    label: "skill",
  },
  prompt: {
    border: "border-amber-500/60",
    bg: "bg-amber-500/10",
    fg: "text-amber-700 dark:text-amber-300",
    icon: icons.prompt,
    label: "prompt",
  },
  llm: {
    border: "border-violet-500/60",
    bg: "bg-violet-500/10",
    fg: "text-violet-700 dark:text-violet-300",
    icon: icons.llm,
    label: "llm",
  },
};

// Custom node. The React Flow default is too plain for the editorial look
// we've committed to across the app.
function ComponentNode({ data }: NodeProps) {
  const { kind, label, subtitle, version, classifications, isUseCaseRoot } =
    data as GraphNodeData;
  const style = KIND_STYLES[kind];
  return (
    <div
      className={cn(
        "rounded-lg border-2 bg-background shadow-sm min-w-[200px] max-w-[260px]",
        style.border,
        isUseCaseRoot && "ring-2 ring-primary/30 ring-offset-2 ring-offset-background",
      )}
    >
      {!isUseCaseRoot && (
        <Handle
          type="target"
          position={Position.Left}
          className="!bg-muted-foreground/30 !border-none"
        />
      )}

      <div className="px-3 py-2 space-y-2">
        <div className="flex items-center gap-2">
          <div
            className={cn(
              "h-7 w-7 rounded-md flex items-center justify-center shrink-0",
              style.bg,
              style.fg,
            )}
          >
            <Icon icon={style.icon} className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground leading-none">
              {style.label}
            </p>
            <p className="text-sm font-semibold leading-tight truncate">
              {label}
            </p>
          </div>
          {version && (
            <span className="text-[10px] font-mono tabular-nums text-muted-foreground shrink-0">
              v{version}
            </span>
          )}
        </div>
        {subtitle && (
          <p className="text-[11px] text-muted-foreground line-clamp-1 font-mono">
            {subtitle}
          </p>
        )}
        {classifications && classifications.length > 0 && (
          <div className="flex items-center gap-1 flex-wrap">
            {classifications.slice(0, 4).map((c) => (
              <span
                key={c}
                className="inline-flex items-center h-[16px] px-1 rounded bg-muted text-[9px] font-mono uppercase tracking-wider text-muted-foreground ring-1 ring-border"
              >
                {c}
              </span>
            ))}
            {classifications.length > 4 && (
              <span className="text-[9px] font-mono text-muted-foreground">
                +{classifications.length - 4}
              </span>
            )}
          </div>
        )}
      </div>

      <Handle
        type="source"
        position={Position.Right}
        className="!bg-muted-foreground/30 !border-none"
      />
    </div>
  );
}

const NODE_TYPES = { component: ComponentNode };

// UseCaseGraph renders a two-column dependency tree: the use case on the
// left, its components fanned out to the right, with horizontal spacing
// tuned so 5-10 components still fit on a reasonable viewport without
// overlap or excessive scroll.
export function UseCaseGraph({ data }: { data: UseCaseWithRefs }) {
  const { nodes, edges } = useMemo(() => buildGraph(data), [data]);

  return (
    <div className="h-[480px] rounded-lg border border-border overflow-hidden bg-muted/20">
      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={NODE_TYPES}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          proOptions={{ hideAttribution: true }}
          nodesDraggable={true}
          nodesConnectable={false}
          edgesReconnectable={false}
          className="bg-transparent"
        >
          <Background color="rgba(100,100,100,0.15)" gap={24} />
          <Controls showInteractive={false} className="!shadow-none !border-border" />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// Compact left-to-right layout: use case node anchored on the left; each
// component kind gets its own horizontal swim-lane row, stacked in a tidy
// column on the right. Cheap, predictable, no layout library needed.
function buildGraph(data: UseCaseWithRefs): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [];
  const edges: Edge[] = [];

  const rootId = "uc:" + data.use_case.id;
  nodes.push({
    id: rootId,
    type: "component",
    position: { x: 0, y: 220 },
    data: {
      kind: "use_case",
      label: data.use_case.name,
      subtitle: data.use_case.slug,
      classifications: data.use_case.effective_classifications ?? data.use_case.data_classifications,
      isUseCaseRoot: true,
    } as GraphNodeData,
  });

  const groups: { kind: GraphNodeKind; refs: UseCaseRef[] | undefined }[] = [
    { kind: "server", refs: data.servers },
    { kind: "agent", refs: data.agents },
    { kind: "llm", refs: data.llms },
    { kind: "skill", refs: data.skills },
    { kind: "prompt", refs: data.prompts },
  ];

  // Right-hand column stacking. Each group gets its own y-band so refs
  // of the same kind cluster together.
  let cursor = 0;
  const vSpacing = 110;
  const xCol = 380;

  groups.forEach((g) => {
    const refs = g.refs ?? [];
    refs.forEach((r, i) => {
      const id = `${g.kind}:${r.target_name}:${r.target_version ?? "latest"}:${i}`;
      nodes.push({
        id,
        type: "component",
        position: { x: xCol, y: cursor * vSpacing },
        data: {
          kind: g.kind,
          label: r.target_name.split("/").slice(-1)[0],
          subtitle: r.target_name,
          version: r.target_version,
        } as GraphNodeData,
      });
      edges.push({
        id: `e:${rootId}->${id}`,
        source: rootId,
        target: id,
        type: "smoothstep",
        animated: false,
        style: { stroke: "rgba(120,120,120,0.4)", strokeWidth: 1.25 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: "rgba(120,120,120,0.4)",
        },
        label: r.local_name ? r.local_name : undefined,
        labelStyle: {
          fontSize: 10,
          fill: "rgba(120,120,120,0.9)",
          fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
        },
        labelBgStyle: {
          fill: "rgba(255,255,255,0.9)",
        },
      });
      cursor++;
    });
  });

  // Center the use case vertically against the right column.
  const rootY = Math.max(0, ((cursor - 1) * vSpacing) / 2);
  nodes[0] = {
    ...nodes[0],
    position: { x: 0, y: rootY },
  };

  return { nodes, edges };
}
