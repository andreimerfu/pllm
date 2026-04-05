import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ReactFlow,
  ReactFlowProvider,
  Node,
  Edge,
  Background,
  Controls,
  MarkerType,
  Handle,
  Position,
  NodeToolbar,
  applyNodeChanges,
  type OnNodesChange,
} from '@xyflow/react';
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { getProviderLogo } from "@/lib/provider-logos";
import { detectProvider } from "@/lib/providers";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import type { RouteModel } from "@/types/api";
import '@xyflow/react/dist/style.css';

interface RouteFlowDiagramProps {
  models: RouteModel[];
  strategy: string;
  routeName: string;
  routeSlug: string;
  availableModels: string[];
  onChange: (models: RouteModel[]) => void;
}

// Entry node: the route itself
function RouteEntryNode({ data }: any) {
  return (
    <>
      <div className="px-6 py-4 border-2 border-primary rounded-xl bg-primary/5 dark:bg-primary/10 shadow-md min-w-[200px]">
        <div className="flex items-center gap-3 mb-2">
          <div className="p-2 rounded-lg bg-primary text-primary-foreground">
            <Icon icon={icons.routes} width="20" height="20" />
          </div>
          <div>
            <div className="font-semibold text-sm">{data.routeName || 'Route'}</div>
            <code className="font-mono text-xs text-muted-foreground mt-0.5 block">
              &quot;{data.routeSlug || 'slug'}&quot;
            </code>
          </div>
        </div>
        <span className="text-xs font-medium text-primary capitalize">
          {data.strategy || 'priority'}
        </span>
      </div>
      <Handle type="source" position={Position.Right} className="!w-3 !h-3 !bg-primary" />
    </>
  );
}

// Model node: one per model in the route
function RouteModelNode({ data, id, selected }: any) {
  const providerInfo = detectProvider(data.modelName, "");
  const providerLogo = getProviderLogo(providerInfo.name);

  return (
    <>
      <NodeToolbar isVisible={selected} position={Position.Top}>
        <button
          onClick={() => data.onDelete?.(id)}
          className="px-2 py-1 bg-destructive text-destructive-foreground rounded text-xs hover:bg-destructive/90 flex items-center gap-1"
        >
          <Icon icon={icons.close} className="w-3 h-3" />
        </button>
      </NodeToolbar>

      <Handle type="target" position={Position.Left} className="!w-3 !h-3 !bg-primary" />

      <div className="px-4 py-3 border-2 rounded-lg bg-background shadow-sm border-border hover:border-primary/40 transition-colors min-w-[220px]">
        <div className="flex items-center gap-3 mb-3">
          <div className={`p-2 rounded-lg border ${providerInfo.bgColor} ${providerInfo.borderColor}`}>
            <Icon
              icon={providerLogo}
              width="20"
              height="20"
              className={providerInfo.color}
            />
          </div>
          <div className="min-w-0 flex-1">
            <div className="font-medium text-sm truncate">{data.modelName}</div>
            <div className={`text-xs ${providerInfo.color}`}>{providerInfo.name}</div>
          </div>
          <div
            className="nodrag nowheel"
            onPointerDownCapture={(e) => e.stopPropagation()}
            onMouseDownCapture={(e) => e.stopPropagation()}
          >
            <Switch
              checked={data.enabled}
              onCheckedChange={(checked: boolean) => data.onToggle?.(id, checked)}
              className="scale-75"
            />
          </div>
        </div>

        <div
          className="space-y-2 nodrag nowheel"
          onPointerDownCapture={(e) => e.stopPropagation()}
          onMouseDownCapture={(e) => e.stopPropagation()}
        >
          <div className="flex items-center justify-between">
            <Label className="text-xs text-muted-foreground">Weight</Label>
            <span className="text-xs font-mono font-medium">{data.weight}</span>
          </div>
          <input
            type="range"
            value={data.weight}
            min={1}
            max={100}
            step={1}
            onChange={(e) => data.onWeightChange?.(id, parseInt(e.target.value))}
            className="w-full h-2 accent-primary cursor-pointer"
          />

          <div className="flex items-center gap-2">
            <Label className="text-xs text-muted-foreground whitespace-nowrap">Priority</Label>
            <Input
              type="number"
              value={data.priority}
              min={1}
              max={100}
              onChange={(e) => data.onPriorityChange?.(id, parseInt(e.target.value) || 1)}
              className="h-6 text-xs w-16"
            />
          </div>
        </div>
      </div>
    </>
  );
}

const nodeTypes = {
  routeEntryNode: RouteEntryNode,
  routeModelNode: RouteModelNode,
};

function RouteFlowDiagramInner({
  models,
  strategy,
  routeName,
  routeSlug,
  availableModels,
  onChange,
}: RouteFlowDiagramProps) {
  const [popoverOpen, setPopoverOpen] = useState(false);

  // Create node update handlers
  const handleDelete = useCallback((nodeId: string) => {
    const modelName = nodeId.replace('model-', '');
    onChange(models.filter(m => m.model_name !== modelName));
  }, [models, onChange]);

  const handleToggle = useCallback((nodeId: string, checked: boolean) => {
    const modelName = nodeId.replace('model-', '');
    onChange(models.map(m => m.model_name === modelName ? { ...m, enabled: checked } : m));
  }, [models, onChange]);

  const handleWeightChange = useCallback((nodeId: string, weight: number) => {
    const modelName = nodeId.replace('model-', '');
    onChange(models.map(m => m.model_name === modelName ? { ...m, weight } : m));
  }, [models, onChange]);

  const handlePriorityChange = useCallback((nodeId: string, priority: number) => {
    const modelName = nodeId.replace('model-', '');
    onChange(models.map(m => m.model_name === modelName ? { ...m, priority } : m));
  }, [models, onChange]);

  // Persist node positions across data updates so dragging isn't reset
  const nodePositions = useRef<Record<string, { x: number; y: number }>>({});
  const [displayNodes, setDisplayNodes] = useState<Node[]>([]);

  // Rebuild nodes when data changes, preserving drag positions
  useEffect(() => {
    const defaultEntryY = models.length > 0 ? (models.length - 1) * 60 : 0;
    const entryNode: Node = {
      id: 'entry',
      type: 'routeEntryNode',
      position: nodePositions.current['entry'] || { x: 0, y: defaultEntryY },
      data: { routeName, routeSlug, strategy },
      draggable: true,
    };

    const modelNodes: Node[] = models.map((rm, i) => ({
      id: `model-${rm.model_name}`,
      type: 'routeModelNode',
      position: nodePositions.current[`model-${rm.model_name}`] || { x: 400, y: i * 160 },
      data: {
        modelName: rm.model_name,
        weight: rm.weight,
        priority: rm.priority,
        enabled: rm.enabled,
        onDelete: handleDelete,
        onToggle: handleToggle,
        onWeightChange: handleWeightChange,
        onPriorityChange: handlePriorityChange,
      },
      draggable: true,
    }));

    setDisplayNodes([entryNode, ...modelNodes]);
  }, [models, routeName, routeSlug, strategy, handleDelete, handleToggle, handleWeightChange, handlePriorityChange]);

  // Handle node position changes (drag)
  const onNodesChange: OnNodesChange = useCallback((changes) => {
    setDisplayNodes((nds) => {
      const updated = applyNodeChanges(changes, nds);
      for (const node of updated) {
        nodePositions.current[node.id] = node.position;
      }
      return updated as Node[];
    });
  }, []);

  // Build edges
  const edges: Edge[] = useMemo(() => {
    const maxWeight = Math.max(...models.map(m => m.weight), 1);
    return models.map((rm) => ({
      id: `e-entry-${rm.model_name}`,
      source: 'entry',
      target: `model-${rm.model_name}`,
      type: 'smoothstep',
      animated: rm.enabled,
      style: {
        stroke: rm.enabled ? '#14B8A6' : '#374151',
        strokeWidth: Math.max(1, (rm.weight / maxWeight) * 4),
        opacity: rm.enabled ? 1 : 0.5,
        strokeDasharray: rm.enabled ? undefined : '4 4',
      },
      label: `w:${rm.weight}`,
      labelStyle: { fontSize: 10, fill: '#6B7280' },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: rm.enabled ? '#14B8A6' : '#374151',
      },
    }));
  }, [models]);

  // Models not yet in the route
  const selectableModels = useMemo(() => {
    const inRoute = new Set(models.map(m => m.model_name));
    return availableModels.filter(m => !inRoute.has(m));
  }, [availableModels, models]);

  const addModel = useCallback((modelName: string) => {
    onChange([...models, { model_name: modelName, weight: 50, priority: 50, enabled: true }]);
    setPopoverOpen(false);
  }, [models, onChange]);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="text-sm font-medium">Route Diagram</div>
        <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
          <PopoverTrigger asChild>
            <Button variant="outline" size="sm" disabled={selectableModels.length === 0} className="gap-1.5">
              <Icon icon={icons.plus} className="w-4 h-4" />
              Add Model
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-64 p-0" align="end">
            <Command>
              <CommandInput placeholder="Search models..." />
              <CommandList>
                <CommandEmpty>No models available</CommandEmpty>
                <CommandGroup>
                  {selectableModels.map((modelName) => {
                    const info = detectProvider(modelName, "");
                    const logo = getProviderLogo(info.name);
                    return (
                      <CommandItem
                        key={modelName}
                        value={modelName}
                        onSelect={() => addModel(modelName)}
                      >
                        <Icon
                          icon={logo}
                          width="16"
                          height="16"
                          className={`mr-2 ${info.color}`}
                        />
                        <span className="truncate">{modelName}</span>
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
      </div>

      <div className="h-[600px] w-full border border-border rounded-lg overflow-hidden bg-[#0F1117] dark:bg-[#0F1117]">
        <ReactFlow
          nodes={displayNodes}
          edges={edges}
          onNodesChange={onNodesChange}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.3 }}
          nodesDraggable={true}
          nodesConnectable={false}
          elementsSelectable={true}
          panOnDrag={true}
          zoomOnScroll={true}
          deleteKeyCode="Delete"
          proOptions={{ hideAttribution: false }}
        >
          <Background color="#1F2937" gap={24} size={1} />
          <Controls className="[&>button]:bg-[#1F2937] [&>button]:border-[#374151] [&>button]:text-[#14B8A6]" />
        </ReactFlow>
      </div>
    </div>
  );
}

export function RouteFlowDiagram(props: RouteFlowDiagramProps) {
  return (
    <ReactFlowProvider>
      <RouteFlowDiagramInner {...props} />
    </ReactFlowProvider>
  );
}
