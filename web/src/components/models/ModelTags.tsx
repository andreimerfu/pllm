import { Badge } from "@/components/ui/badge";
import { HoverCard, HoverCardTrigger, HoverCardContent } from "@/components/ui/hover-card";

interface ModelTagsProps {
  tags?: string[];
  maxVisible?: number;
}

export default function ModelTags({ tags, maxVisible = 3 }: ModelTagsProps) {
  if (!tags || tags.length === 0) {
    return null;
  }

  const visibleTags = tags.slice(0, maxVisible);
  const remainingCount = tags.length - maxVisible;
  const remainingTags = tags.slice(maxVisible);

  return (
    <div className="flex items-center gap-1 flex-wrap">
      {visibleTags.map((tag, index) => (
        <Badge key={index} variant="outline" className="text-xs">
          {tag}
        </Badge>
      ))}

      {remainingCount > 0 && (
        <HoverCard>
          <HoverCardTrigger asChild>
            <Badge variant="secondary" className="text-xs cursor-help hover:bg-secondary/80 transition-colors">
              +{remainingCount} more
            </Badge>
          </HoverCardTrigger>
          <HoverCardContent className="w-auto max-w-sm">
            <div className="space-y-2">
              <h4 className="text-sm font-semibold">All Tags</h4>
              <div className="flex flex-wrap gap-1">
                {remainingTags.map((tag, index) => (
                  <Badge key={index} variant="outline" className="text-xs">
                    {tag}
                  </Badge>
                ))}
              </div>
            </div>
          </HoverCardContent>
        </HoverCard>
      )}
    </div>
  );
}
