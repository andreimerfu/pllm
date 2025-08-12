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
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Settings</h1>
        <p className="text-muted-foreground">
          Configure gateway settings and preferences
        </p>
      </div>

      <Tabs defaultValue="general" className="space-y-4">
        <TabsList>
          <TabsTrigger value="general">General</TabsTrigger>
          <TabsTrigger value="database">Database</TabsTrigger>
          <TabsTrigger value="security">Security</TabsTrigger>
          <TabsTrigger value="notifications">Notifications</TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>General Settings</CardTitle>
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
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="enable-logging">Enable Logging</Label>
                  <p className="text-sm text-muted-foreground">
                    Log all API requests and responses
                  </p>
                </div>
                <Switch id="enable-logging" defaultChecked />
              </div>
              <Button>Save Changes</Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="database" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Database Configuration</CardTitle>
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
              <Button>Test Connection</Button>
              <Button className="ml-2">Save Changes</Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Security Settings</CardTitle>
              <CardDescription>
                Configure security and authentication
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="require-auth">Require Authentication</Label>
                  <p className="text-sm text-muted-foreground">
                    Require API key for all requests
                  </p>
                </div>
                <Switch id="require-auth" defaultChecked />
              </div>
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="enable-cors">Enable CORS</Label>
                  <p className="text-sm text-muted-foreground">
                    Allow cross-origin requests
                  </p>
                </div>
                <Switch id="enable-cors" defaultChecked />
              </div>
              <div className="space-y-2">
                <Label htmlFor="rate-limit">Rate Limit (requests/minute)</Label>
                <Input id="rate-limit" type="number" defaultValue="60" />
              </div>
              <Button>Save Changes</Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="notifications" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Notification Settings</CardTitle>
              <CardDescription>
                Configure alerts and notifications
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="email-alerts">Email Alerts</Label>
                  <p className="text-sm text-muted-foreground">
                    Send email for critical issues
                  </p>
                </div>
                <Switch id="email-alerts" />
              </div>
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="slack-integration">Slack Integration</Label>
                  <p className="text-sm text-muted-foreground">
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
              <Button>Save Changes</Button>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
