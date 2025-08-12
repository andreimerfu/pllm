import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export default function Settings() {
  return (
    <div className="space-y-4 lg:space-y-6">
      <div>
        <h1 className="text-2xl lg:text-3xl font-bold">Settings</h1>
        <p className="text-sm lg:text-base text-muted-foreground">
          Configure gateway settings and preferences
        </p>
      </div>

      <Tabs defaultValue="general" className="space-y-4">
        <TabsList className="grid w-full grid-cols-2 lg:grid-cols-4">
          <TabsTrigger value="general" className="text-xs lg:text-sm">General</TabsTrigger>
          <TabsTrigger value="database" className="text-xs lg:text-sm">Database</TabsTrigger>
          <TabsTrigger value="security" className="text-xs lg:text-sm">Security</TabsTrigger>
          <TabsTrigger value="notifications" className="text-xs lg:text-sm">Notifications</TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg lg:text-xl">General Settings</CardTitle>
              <CardDescription>
                Configure basic gateway settings
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="gateway-name">Gateway Name</Label>
                <Input id="gateway-name" defaultValue="pLLM Gateway" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="api-url">API URL</Label>
                <Input id="api-url" defaultValue="http://localhost:8080" />
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-3 border rounded-lg">
                <div className="flex-1">
                  <Label htmlFor="enable-logging" className="font-medium">Enable Logging</Label>
                  <p className="text-sm text-muted-foreground mt-1">
                    Log all API requests and responses
                  </p>
                </div>
                <Switch id="enable-logging" defaultChecked />
              </div>
              <div className="flex flex-col sm:flex-row gap-2">
                <Button className="w-full sm:w-auto">Save Changes</Button>
                <Button variant="outline" className="w-full sm:w-auto">Reset to Defaults</Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="database" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg lg:text-xl">Database Configuration</CardTitle>
              <CardDescription>
                Configure database connection settings
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="db-url">Database URL</Label>
                <Input
                  id="db-url"
                  type="password"
                  placeholder="postgresql://user:pass@localhost/db"
                />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="max-connections">Max Connections</Label>
                  <Input id="max-connections" type="number" defaultValue="10" />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="connection-timeout">
                    Connection Timeout (seconds)
                  </Label>
                  <Input
                    id="connection-timeout"
                    type="number"
                    defaultValue="30"
                  />
                </div>
              </div>
              <div className="flex flex-col sm:flex-row gap-2">
                <Button className="w-full sm:w-auto">Test Connection</Button>
                <Button className="w-full sm:w-auto">Save Changes</Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg lg:text-xl">Security Settings</CardTitle>
              <CardDescription>
                Configure security and authentication
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-3 border rounded-lg">
                <div className="flex-1">
                  <Label htmlFor="require-auth" className="font-medium">Require Authentication</Label>
                  <p className="text-sm text-muted-foreground mt-1">
                    Require API key for all requests
                  </p>
                </div>
                <Switch id="require-auth" defaultChecked />
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-3 border rounded-lg">
                <div className="flex-1">
                  <Label htmlFor="enable-cors" className="font-medium">Enable CORS</Label>
                  <p className="text-sm text-muted-foreground mt-1">
                    Allow cross-origin requests
                  </p>
                </div>
                <Switch id="enable-cors" defaultChecked />
              </div>
              <div className="space-y-2">
                <Label htmlFor="rate-limit">Rate Limit (requests/minute)</Label>
                <Input id="rate-limit" type="number" defaultValue="60" className="max-w-xs" />
              </div>
              <Button className="w-full sm:w-auto">Save Changes</Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="notifications" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg lg:text-xl">Notification Settings</CardTitle>
              <CardDescription>
                Configure alerts and notifications
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-3 border rounded-lg">
                <div className="flex-1">
                  <Label htmlFor="email-alerts" className="font-medium">Email Alerts</Label>
                  <p className="text-sm text-muted-foreground mt-1">
                    Send email for critical issues
                  </p>
                </div>
                <Switch id="email-alerts" />
              </div>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-3 border rounded-lg">
                <div className="flex-1">
                  <Label htmlFor="slack-integration" className="font-medium">Slack Integration</Label>
                  <p className="text-sm text-muted-foreground mt-1">
                    Send notifications to Slack
                  </p>
                </div>
                <Switch id="slack-integration" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="webhook-url">Webhook URL</Label>
                <Input
                  id="webhook-url"
                  placeholder="https://hooks.slack.com/services/..."
                />
              </div>
              <Button className="w-full sm:w-auto">Save Changes</Button>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
