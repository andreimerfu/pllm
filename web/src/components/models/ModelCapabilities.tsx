import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { HoverCard, HoverCardTrigger, HoverCardContent } from "@/components/ui/hover-card";
import { ModelCapabilities as IModelCapabilities } from "@/types/api";
import {
  Eye,
  Mic,
  Volume2,
  Zap,
  FileText,
  MessageSquare,
  Brain,
  Search,
  Code,
  Layers
} from "lucide-react";

interface ModelCapabilitiesProps {
  capabilities?: IModelCapabilities;
  showLabels?: boolean;
  maxVisible?: number;
}

export default function ModelCapabilities({ capabilities, showLabels = false, maxVisible = 5 }: ModelCapabilitiesProps) {
  if (!capabilities) {
    return null;
  }

  const capabilityConfig = [
    {
      key: 'function_calling',
      label: 'Function Calls',
      icon: Code,
      description: 'Supports structured function calling'
    },
    {
      key: 'parallel_function_calling',
      label: 'Parallel Functions',
      icon: Layers,
      description: 'Supports parallel function execution'
    },
    {
      key: 'vision',
      label: 'Vision',
      icon: Eye,
      description: 'Can process and understand images'
    },
    {
      key: 'audio_input',
      label: 'Audio Input',
      icon: Mic,
      description: 'Supports audio input processing'
    },
    {
      key: 'audio_output',
      label: 'Audio Output',
      icon: Volume2,
      description: 'Can generate audio responses'
    },
    {
      key: 'prompt_caching',
      label: 'Caching',
      icon: Zap,
      description: 'Supports prompt caching for efficiency'
    },
    {
      key: 'response_schema',
      label: 'Schema',
      icon: FileText,
      description: 'Supports structured response schemas'
    },
    {
      key: 'system_messages',
      label: 'System',
      icon: MessageSquare,
      description: 'Supports system message instructions'
    },
    {
      key: 'reasoning',
      label: 'Reasoning',
      icon: Brain,
      description: 'Advanced reasoning capabilities'
    },
    {
      key: 'web_search',
      label: 'Web Search',
      icon: Search,
      description: 'Can perform web searches'
    }
  ];

  const enabledCapabilities = capabilityConfig.filter(
    (config) => capabilities[config.key as keyof IModelCapabilities]
  );

  if (enabledCapabilities.length === 0) {
    return (
      <div className="text-muted-foreground text-sm">
        No capabilities listed
      </div>
    );
  }

  const visibleCapabilities = enabledCapabilities.slice(0, maxVisible);
  const remainingCount = enabledCapabilities.length - maxVisible;
  const remainingCapabilities = enabledCapabilities.slice(maxVisible);

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <TooltipProvider>
        {visibleCapabilities.map((config) => {
          const Icon = config.icon;

          if (showLabels) {
            return (
              <Tooltip key={config.key}>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="gap-1 cursor-help">
                    <Icon className="h-3 w-3" />
                    {config.label}
                  </Badge>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{config.description}</p>
                </TooltipContent>
              </Tooltip>
            );
          }

          return (
            <Tooltip key={config.key}>
              <TooltipTrigger asChild>
                <Badge variant="outline" className="p-1 cursor-help">
                  <Icon className="h-3 w-3" />
                </Badge>
              </TooltipTrigger>
              <TooltipContent>
                <p>{config.label}: {config.description}</p>
              </TooltipContent>
            </Tooltip>
          );
        })}
      </TooltipProvider>

      {remainingCount > 0 && (
        <HoverCard>
          <HoverCardTrigger asChild>
            <Badge variant="outline" className="cursor-help hover:bg-secondary/80 transition-colors">
              +{remainingCount} more
            </Badge>
          </HoverCardTrigger>
          <HoverCardContent className="w-auto max-w-md">
            <div className="space-y-3">
              <h4 className="text-sm font-semibold">Additional Capabilities</h4>
              <div className="space-y-2">
                {remainingCapabilities.map((config) => {
                  const Icon = config.icon;
                  return (
                    <div key={config.key} className="flex items-start gap-2">
                      <Icon className="h-4 w-4 mt-0.5 text-muted-foreground flex-shrink-0" />
                      <div>
                        <div className="text-sm font-medium">{config.label}</div>
                        <div className="text-xs text-muted-foreground">{config.description}</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </HoverCardContent>
        </HoverCard>
      )}
    </div>
  );
}
