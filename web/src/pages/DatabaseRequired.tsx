import { Icon } from "@iconify/react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function DatabaseRequired() {
  const handleRefresh = () => {
    window.location.reload();
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-800">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 h-12 w-12 rounded-full bg-destructive/10 flex items-center justify-center">
            <Icon icon="lucide:database" width="24" height="24" className="text-destructive" />
          </div>
          <CardTitle>Database Connection Required</CardTitle>
          <CardDescription>
            The admin UI requires a database connection to function properly
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-lg bg-muted p-4">
            <div className="flex items-start space-x-2">
              <Icon icon="lucide:alert-circle" width="20" height="20" className="text-amber-500 mt-0.5" />
              <div className="text-sm">
                <p className="font-medium">Configure Database</p>
                <p className="text-muted-foreground mt-1">
                  Please ensure PostgreSQL is running and the DATABASE_URL
                  environment variable is set correctly.
                </p>
              </div>
            </div>
          </div>

          <div className="space-y-2 text-sm text-muted-foreground">
            <p className="font-medium">Quick Setup:</p>
            <ol className="list-decimal list-inside space-y-1">
              <li>Start PostgreSQL using Docker Compose</li>
              <li>Set DATABASE_URL in your .env file</li>
              <li>Restart the pLLM gateway</li>
            </ol>
          </div>

          <div className="bg-muted rounded-lg p-3">
            <code className="text-xs">docker compose up -d postgres</code>
          </div>

          <Button onClick={handleRefresh} className="w-full">
            <Icon icon="lucide:refresh-cw" width="16" height="16" className="mr-2" />
            Retry Connection
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
