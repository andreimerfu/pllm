import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { HoverCard, HoverCardTrigger, HoverCardContent } from "@/components/ui/hover-card";
import { Icon } from "@iconify/react";
import { ModelCapabilities as IModelCapabilities } from "@/types/api";

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
      icon: 'solar:code-linear',
      description: 'Supports structured function calling'
    },
    {
      key: 'parallel_function_calling',
      label: 'Parallel Functions',
      icon: 'solar:layers-linear',
      description: 'Supports parallel function execution'
    },
    {
      key: 'vision',
      label: 'Vision',
      icon: 'solar:eye-linear',
      description: 'Can process and understand images'
    },
    {
      key: 'audio_input',
      label: 'Audio Input',
      icon: 'solar:microphone-linear',
      description: 'Supports audio input processing'
    },
    {
      key: 'audio_output',
      label: 'Audio Output',
      icon: 'solar:volume-loud-linear',
      description: 'Can generate audio responses'
    },
    {
      key: 'prompt_caching',
      label: 'Caching',
      icon: 'solar:bolt-linear',
      description: 'Supports prompt caching for efficiency'
    },
    {
      key: 'response_schema',
      label: 'Schema',
      icon: 'solar:file-text-linear',
      description: 'Supports structured response schemas'
    },
    {
      key: 'system_messages',
      label: 'System',
      icon: 'solar:chat-round-linear',
      description: 'Supports system message instructions'
    },
    {
      key: 'reasoning',
      label: 'Reasoning',
      icon: 'solar:cpu-bolt-linear',
      description: 'Advanced reasoning capabilities'
    },
    {
      key: 'web_search',
      label: 'Web Search',
      icon: 'solar:magnifer-linear',
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
          if (showLabels) {
            return (
              <Tooltip key={config.key}>
                <TooltipTrigger asChild>
                  <Badge variant="secondary" className="gap-1 cursor-help">
                    <Icon icon={config.icon} className="h-3 w-3" />
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
                  <Icon icon={config.icon} className="h-3 w-3" />
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
                  return (
                    <div key={config.key} className="flex items-start gap-2">
                      <Icon icon={config.icon} className="h-4 w-4 mt-0.5 text-muted-foreground flex-shrink-0" />
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
