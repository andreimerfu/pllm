import { useState, useCallback } from 'react';
import {
  ReactFlow,
  ReactFlowProvider,
  Node,
  Edge,
  Background,
  Controls,
  MarkerType,
  addEdge,
  Connection,
  useNodesState,
  useEdgesState,
  NodeToolbar,
  Handle,
  Position,
  Panel,
} from '@xyflow/react';
import { Icon } from "@iconify/react";
import { detectProvider } from "@/lib/providers";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Plus, Save, X } from "lucide-react";
import '@xyflow/react/dist/style.css';

interface EditableFallbackDiagramProps {
  primaryModel: string;
  allFallbacksConfig?: Record<string, string[]>;
  onSave?: (fallbacks: Record<string, string[]>) => void;
}

// Custom editable node component
function EditableModelNode({ data, id, selected }: any) {
  const providerInfo = detectProvider(data.modelId, data.provider);

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

      <div className={`px-4 py-3 border-2 rounded-lg bg-background shadow-sm ${
        data.isPrimary ? 'border-blue-500 bg-blue-50 dark:bg-blue-950' : 'border-border'
      }`}>
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg border ${providerInfo.bgColor} ${providerInfo.borderColor}`}>
            <Icon
              icon={providerInfo.icon}
              width="20"
              height="20"
              className={providerInfo.color}
            />
          </div>
          <div className="min-w-0">
            <div className="font-medium text-sm truncate">{data.modelId}</div>
            <div className={`text-xs ${providerInfo.color}`}>
              {providerInfo.name}
            </div>
          </div>
          {data.isPrimary && (
            <Badge variant="default" className="text-xs">
              Primary
            </Badge>
          )}
        </div>
      </div>

      <Handle type="source" position={Position.Right} className="!w-3 !h-3 !bg-blue-500" />
    </>
  );
}

const nodeTypes = {
  editableModelNode: EditableModelNode,
};

function EditableFallbackDiagramInner({ primaryModel, allFallbacksConfig = {}, onSave }: EditableFallbackDiagramProps) {
  // Build complete fallback chain
  const buildCompleteChain = (modelName: string, visited = new Set<string>()): string[] => {
    if (visited.has(modelName)) {
      return [];
    }

    visited.add(modelName);
    const directFallbacks = allFallbacksConfig[modelName] || [];

    const chain: string[] = [];
    for (const fallback of directFallbacks) {
      chain.push(fallback);
      const subChain = buildCompleteChain(fallback, visited);
      chain.push(...subChain);
    }

    return chain;
  };

  const completeChain = buildCompleteChain(primaryModel);
  const uniqueChain = [...new Set([primaryModel, ...completeChain])];

  // Create initial nodes and edges
  const initialNodes: Node[] = uniqueChain.map((modelName, index) => ({
    id: index === 0 ? 'primary' : `model-${index}`,
    type: 'editableModelNode',
    position: {
      x: index * 300,
      y: 100
    },
    data: {
      modelId: modelName,
      provider: modelName.includes("claude") ? "anthropic" :
               modelName.includes("gpt") ? "openai" :
               modelName.includes("gemini") ? "google" : "openrouter",
      isPrimary: index === 0,
      onDelete: (nodeId: string) => {
        setNodes((nds) => nds.filter(n => n.id !== nodeId));
        setEdges((eds) => eds.filter(e => e.source !== nodeId && e.target !== nodeId));
      },
    },
  }));

  const initialEdges: Edge[] = [];
  for (let i = 0; i < uniqueChain.length - 1; i++) {
    const sourceId = i === 0 ? 'primary' : `model-${i}`;
    const targetId = `model-${i + 1}`;

    initialEdges.push({
      id: `e-${sourceId}-${targetId}`,
      source: sourceId,
      target: targetId,
      type: 'smoothstep',
      animated: true,
      style: {
        stroke: i === 0 ? '#3b82f6' : '#6366f1',
        strokeWidth: i === 0 ? 3 : 2,
      },
      label: i === 0 ? 'Primary Fallback' : `Fallback ${i}`,
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: i === 0 ? '#3b82f6' : '#6366f1',
      },
    });
  }

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);
  const [hasChanges, setHasChanges] = useState(false);

  // Handle new connections
  const onConnect = useCallback((params: Connection) => {
    setEdges((eds) => addEdge({
      ...params,
      type: 'smoothstep',
      animated: true,
      style: { stroke: '#6366f1', strokeWidth: 2 },
      markerEnd: { type: MarkerType.ArrowClosed, color: '#6366f1' },
    }, eds));
    setHasChanges(true);
  }, [setEdges]);

  // Add new model node
  const addNode = useCallback(() => {
    const newId = `model-${nodes.length}`;
    const newNode: Node = {
      id: newId,
      type: 'editableModelNode',
      position: {
        x: nodes.length * 300,
        y: 100,
      },
      data: {
        modelId: 'new-model',
        provider: 'openai',
        isPrimary: false,
        onDelete: (nodeId: string) => {
          setNodes((nds) => nds.filter(n => n.id !== nodeId));
          setEdges((eds) => eds.filter(e => e.source !== nodeId && e.target !== nodeId));
        },
      },
    };
    setNodes((nds) => [...nds, newNode]);
    setHasChanges(true);
  }, [nodes, setNodes, setEdges]);

  // Save fallback configuration
  const handleSave = useCallback(() => {
    // Build fallback config from edges
    const newConfig: Record<string, string[]> = {};

    edges.forEach(edge => {
      const sourceNode = nodes.find(n => n.id === edge.source);
      const targetNode = nodes.find(n => n.id === edge.target);

      if (sourceNode && targetNode) {
        const sourceModel = sourceNode.data.modelId as string;
        const targetModel = targetNode.data.modelId as string;

        if (!newConfig[sourceModel]) {
          newConfig[sourceModel] = [];
        }
        if (!newConfig[sourceModel].includes(targetModel)) {
          newConfig[sourceModel].push(targetModel);
        }
      }
    });

    onSave?.(newConfig);
    setHasChanges(false);
  }, [nodes, edges, onSave]);

  // Mark as changed when nodes/edges change
  const handleNodesChange = useCallback((changes: any) => {
    onNodesChange(changes);
    setHasChanges(true);
  }, [onNodesChange]);

  const handleEdgesChange = useCallback((changes: any) => {
    onEdgesChange(changes);
    setHasChanges(true);
  }, [onEdgesChange]);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Fallback Configuration (Editable)</CardTitle>
            <CardDescription>
              Draw connections between models to configure fallback chains. Drag to move nodes.
            </CardDescription>
          </div>
          <div className="flex gap-2">
            <Button onClick={addNode} variant="outline" size="sm">
              <Plus className="w-4 h-4 mr-1" />
              Add Model
            </Button>
            {hasChanges && (
              <Button onClick={handleSave} size="sm">
                <Save className="w-4 h-4 mr-1" />
                Save Changes
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="h-96 w-full border rounded-lg overflow-hidden bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-800">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={handleNodesChange}
            onEdgesChange={handleEdgesChange}
            onConnect={onConnect}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            nodesDraggable={true}
            nodesConnectable={true}
            elementsSelectable={true}
            panOnDrag={true}
            zoomOnScroll={true}
            deleteKeyCode="Delete"
            proOptions={{ hideAttribution: false }}
          >
            <Background variant={'dots' as any} gap={20} size={1} />
            <Controls />
            <Panel position="top-left" className="bg-white dark:bg-slate-800 p-3 rounded-lg shadow-md text-sm">
              <div className="font-semibold mb-2">Instructions:</div>
              <ul className="space-y-1 text-xs text-muted-foreground">
                <li>• Drag nodes to reposition</li>
                <li>• Drag from handles to create connections</li>
                <li>• Click node to select, press Delete to remove</li>
                <li>• Click "Add Model" to add new nodes</li>
                <li>• Click "Save Changes" when done</li>
              </ul>
            </Panel>
          </ReactFlow>
        </div>

        <div className="mt-4 space-y-3">
          <div className="text-sm font-medium">Current Configuration</div>
          <div className="text-xs text-muted-foreground">
            {edges.length === 0 ? (
              <div className="p-4 border rounded-lg text-center">
                No fallback connections configured. Draw connections between nodes to create fallback chains.
              </div>
            ) : (
              <div className="space-y-2">
                {Array.from(new Set(edges.map(e => e.source))).map(sourceId => {
                  const sourceNode = nodes.find(n => n.id === sourceId);
                  const targets = edges
                    .filter(e => e.source === sourceId)
                    .map(e => nodes.find(n => n.id === e.target))
                    .filter(Boolean);

                  return (
                    <div key={sourceId} className="flex items-center gap-2 p-2 border rounded">
                      <Badge variant="outline">{sourceNode?.data.modelId as string}</Badge>
                      <span>→</span>
                      <div className="flex gap-1 flex-wrap">
                        {targets.map((target, idx) => (
                          <Badge key={idx} variant="secondary" className="text-xs">
                            {target?.data.modelId as string}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export function EditableFallbackDiagram(props: EditableFallbackDiagramProps) {
  return (
    <ReactFlowProvider>
      <EditableFallbackDiagramInner {...props} />
    </ReactFlowProvider>
  );
}
