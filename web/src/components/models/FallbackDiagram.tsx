import { useMemo } from 'react';
import {
  ReactFlow,
  ReactFlowProvider,
  Node,
  Edge,
  Background,
  Controls,
  MarkerType,
} from '@xyflow/react';
import { Icon } from "@iconify/react";
import { detectProvider } from "@/lib/providers";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import '@xyflow/react/dist/style.css';

interface FallbackDiagramProps {
  primaryModel: string;
  fallbacks: string[];
  allFallbacksConfig?: Record<string, string[]>;
}

// Custom node component
function ModelNode({ data }: { data: any }) {
  const providerInfo = detectProvider(data.modelId, data.provider);

  return (
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
      {data.status && (
        <div className="mt-2 pt-2 border-t">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground">Status</span>
            <Badge variant={data.status === 'healthy' ? 'default' : data.status === 'degraded' ? 'secondary' : 'destructive'}>
              {data.status}
            </Badge>
          </div>
        </div>
      )}
    </div>
  );
}

const nodeTypes = {
  modelNode: ModelNode,
};

export function FallbackDiagram({ primaryModel, allFallbacksConfig = {} }: FallbackDiagramProps) {
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

  const { nodes, edges } = useMemo(() => {
    const nodes: Node[] = [];
    const edges: Edge[] = [];

    const completeChain = buildCompleteChain(primaryModel);
    const uniqueChain = [...new Set([primaryModel, ...completeChain])];

    // Create nodes with explicit dimensions for proper edge rendering
    uniqueChain.forEach((modelName, index) => {
      nodes.push({
        id: index === 0 ? 'primary' : `model-${index}`,
        type: 'modelNode',
        position: {
          x: index * 300,
          y: 100
        },
        width: 250,
        height: 100,
        data: {
          modelId: modelName,
          provider: modelName.includes("claude") ? "anthropic" :
                   modelName.includes("gpt") ? "openai" :
                   modelName.includes("gemini") ? "google" : "openrouter",
          isPrimary: index === 0,
          status: Math.random() > 0.3 ? 'healthy' : Math.random() > 0.1 ? 'degraded' : 'unhealthy',
        },
      });
    });

    // Create animated edges between consecutive models
    for (let i = 0; i < uniqueChain.length - 1; i++) {
      const sourceId = i === 0 ? 'primary' : `model-${i}`;
      const targetId = `model-${i + 1}`;

      edges.push({
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
        labelStyle: {
          fontSize: '11px',
          fontWeight: i === 0 ? 600 : 500,
          fill: '#64748b'
        },
        labelBgStyle: { fill: 'white', fillOpacity: 0.9 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: i === 0 ? '#3b82f6' : '#6366f1',
        },
      });
    }

    return { nodes, edges };
  }, [primaryModel, allFallbacksConfig]);

  const completeChain = buildCompleteChain(primaryModel);
  const uniqueChain = [...new Set([primaryModel, ...completeChain])];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Fallback Configuration</CardTitle>
        <CardDescription>
          Complete fallback chain - requests will cascade through these models in order if failures occur
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="h-80 w-full border rounded-lg overflow-hidden bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-800">
          <ReactFlowProvider>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              fitView
              fitViewOptions={{ padding: 0.2 }}
              nodesDraggable={false}
              nodesConnectable={false}
              edgesFocusable={false}
              elementsSelectable={false}
              panOnDrag={uniqueChain.length > 3}
              zoomOnScroll={false}
              zoomOnPinch={false}
              zoomOnDoubleClick={false}
              proOptions={{ hideAttribution: false }}
            >
              <Background variant={'dots' as any} gap={20} size={1} />
              <Controls showInteractive={false} showZoom={uniqueChain.length > 3} />
            </ReactFlow>
          </ReactFlowProvider>
        </div>

        <div className="mt-4 space-y-3">
          <div className="text-sm font-medium">Fallback Chain Details</div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
            <div>
              <div className="font-medium mb-2">Chain Order:</div>
              <ol className="space-y-1 text-muted-foreground">
                {uniqueChain.map((model, index) => (
                  <li key={model} className="flex items-center gap-2">
                    <span className={`w-5 h-5 rounded-full flex items-center justify-center text-xs font-medium ${
                      index === 0 ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300' :
                      'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300'
                    }`}>
                      {index + 1}
                    </span>
                    <span className={index === 0 ? 'font-medium text-foreground' : ''}>{model}</span>
                    {index === 0 && <Badge variant="outline" className="text-xs">Primary</Badge>}
                  </li>
                ))}
              </ol>
            </div>
            <div>
              <div className="font-medium mb-2">Flow Legend:</div>
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-0.5 bg-blue-500 relative">
                    <div className="absolute inset-0 bg-blue-400 animate-pulse"></div>
                  </div>
                  <span className="text-muted-foreground">Primary failover path</span>
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-8 h-0.5 bg-indigo-500 relative">
                    <div className="absolute inset-0 bg-indigo-400 animate-pulse"></div>
                  </div>
                  <span className="text-muted-foreground">Chain continuation</span>
                </div>
              </div>
              <p className="text-xs text-muted-foreground mt-2">
                Animated arrows show the direction of fallback flow when models become unavailable.
              </p>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
