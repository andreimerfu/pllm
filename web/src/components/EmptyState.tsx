import { Icon } from "@iconify/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

interface EmptyStateProps {
  icon: string;
  title: string;
  description: string;
  action?: {
    label: string;
    href?: string;
    onClick?: () => void;
  };
  variant?: "default" | "chart";
}

export function EmptyState({ 
  icon, 
  title, 
  description, 
  action,
  variant = "default" 
}: EmptyStateProps) {
  const content = (
    <div className={`flex flex-col items-center justify-center text-center ${
      variant === "chart" ? "h-full min-h-[200px]" : "p-8"
    }`}>
      <div className="mb-4">
        <div className="h-12 w-12 rounded-lg bg-muted/50 flex items-center justify-center mb-3">
          <Icon 
            icon={icon} 
            width="24" 
            height="24" 
            className="text-muted-foreground" 
          />
        </div>
      </div>
      
      <h3 className="text-lg font-medium mb-2">{title}</h3>
      <p className="text-muted-foreground text-sm max-w-sm mb-4">
        {description}
      </p>
      
      {action && (
        <Button
          variant="outline"
          size="sm"
          onClick={action.onClick}
          asChild={!!action.href}
        >
          {action.href ? (
            <a href={action.href} target="_blank" rel="noopener noreferrer">
              {action.label}
            </a>
          ) : (
            action.label
          )}
        </Button>
      )}
    </div>
  );

  if (variant === "chart") {
    return content;
  }

  return (
    <Card>
      <CardContent className="p-0">
        {content}
      </CardContent>
    </Card>
  );
}