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
import { detectProvider } from "@/lib/providers";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { Plus, X } from "lucide-react";
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
      <div className="px-6 py-4 border-2 border-blue-500 rounded-xl bg-blue-50 dark:bg-blue-950/50 shadow-md min-w-[200px]">
        <div className="flex items-center gap-3 mb-2">
          <div className="p-2 rounded-lg bg-blue-500 text-white">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M6 3v12" /><path d="M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6z" /><path d="M6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6z" /><path d="M15 6a9 9 0 0 0-9 9" />
            </svg>
          </div>
          <div>
            <div className="font-semibold text-sm">{data.routeName || 'Route'}</div>
            <Badge variant="outline" className="font-mono text-xs mt-0.5">
              {data.routeSlug || 'slug'}
            </Badge>
          </div>
        </div>
        <Badge className="text-xs bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
          {data.strategy || 'priority'}
        </Badge>
      </div>
      <Handle type="source" position={Position.Right} className="!w-3 !h-3 !bg-blue-500" />
    </>
  );
}

// Model node: one per model in the route
function RouteModelNode({ data, id, selected }: any) {
  const providerInfo = detectProvider(data.modelName, "");

  return (
    <>
      <NodeToolbar isVisible={selected} position={Position.Top}>
        <button
          onClick={() => data.onDelete?.(id)}
          className="px-2 py-1 bg-red-500 text-white rounded text-xs hover:bg-red-600"
        >
          <X className="w-3 h-3" />
        </button>
      </NodeToolbar>

      <Handle type="target" position={Position.Left} className="!w-3 !h-3 !bg-blue-500" />

      <div className="px-4 py-3 border-2 rounded-lg bg-background shadow-sm border-border min-w-[220px]">
        <div className="flex items-center gap-3 mb-3">
          <div className={`p-2 rounded-lg border ${providerInfo.bgColor} ${providerInfo.borderColor}`}>
            <Icon
              icon={providerInfo.icon}
              width="20"
              height="20"
              className={providerInfo.color}
            />
          </div>
          <div className="min-w-0 flex-1">
            <div className="font-medium text-sm truncate">{data.modelName}</div>
            <div className={`text-xs ${providerInfo.color}`}>{providerInfo.name}</div>
          </div>
          {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions */}
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

        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions */}
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
        stroke: rm.enabled ? '#3b82f6' : '#94a3b8',
        strokeWidth: Math.max(1, (rm.weight / maxWeight) * 4),
        opacity: rm.enabled ? 1 : 0.4,
      },
      label: `w:${rm.weight}`,
      labelStyle: { fontSize: 10, fill: '#64748b' },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: rm.enabled ? '#3b82f6' : '#94a3b8',
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
            <Button variant="outline" size="sm" disabled={selectableModels.length === 0}>
              <Plus className="w-4 h-4 mr-1" />
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
                    return (
                      <CommandItem
                        key={modelName}
                        value={modelName}
                        onSelect={() => addModel(modelName)}
                      >
                        <Icon
                          icon={info.icon}
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

      <div className="h-[600px] w-full border rounded-lg overflow-hidden bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-800">
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
          <Background variant={'dots' as any} gap={20} size={1} />
          <Controls />
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
