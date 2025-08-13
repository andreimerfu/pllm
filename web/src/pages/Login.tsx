import { useState } from "react";
import { useAuth } from "@/contexts/OIDCAuthContext";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Icon } from "@iconify/react";
import { useToast } from "@/hooks/use-toast";
import { Loader2, Github, Mail, Shield } from "lucide-react";

export default function Login() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const { login, loginWithCredentials } = useAuth();
  const { toast } = useToast();

  const handleCredentialsLogin = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!username.trim() || !password.trim()) {
      toast({
        title: "Error",
        description: "Please enter username and password",
        variant: "destructive",
      });
      return;
    }

    setIsLoading(true);
    try {
      await loginWithCredentials(username, password);
    } catch (error) {
      // Error is handled in the context
    } finally {
      setIsLoading(false);
    }
  };

  const handleOAuthLogin = () => {
    // For now, we'll use the generic login which will redirect to Dex
    // Dex will show the provider selection screen
    login();
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <div className="flex items-center justify-center mb-4">
            <div className="p-3 rounded-lg bg-primary/10">
              <Shield className="h-8 w-8 text-primary" />
            </div>
          </div>
          <CardTitle className="text-2xl text-center">pLLM Gateway</CardTitle>
          <CardDescription className="text-center">
            Sign in to access the admin panel
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* OAuth Provider Buttons */}
          <div className="space-y-2">
            <Button
              variant="outline"
              className="w-full"
              onClick={handleOAuthLogin}
              disabled={isLoading}
            >
              <Github className="mr-2 h-4 w-4" />
              Continue with GitHub
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={handleOAuthLogin}
              disabled={isLoading}
            >
              <Mail className="mr-2 h-4 w-4" />
              Continue with Google
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={handleOAuthLogin}
              disabled={isLoading}
            >
              <Icon icon="mdi:microsoft" className="mr-2 h-4 w-4" />
              Continue with Microsoft
            </Button>
          </div>

          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <Separator />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-background px-2 text-muted-foreground">
                Or continue with
              </span>
            </div>
          </div>

          {/* Static Credentials Form */}
          <form onSubmit={handleCredentialsLogin} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Email or Username</Label>
              <Input
                id="username"
                type="text"
                placeholder="admin@pllm.local"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={isLoading}
                autoComplete="username"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                placeholder="Enter your password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isLoading}
                autoComplete="current-password"
              />
            </div>
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Signing in...
                </>
              ) : (
                "Sign in"
              )}
            </Button>
          </form>

          {/* Development/Testing Info */}
          {import.meta.env.DEV && (
            <div className="mt-6 pt-6 border-t">
              <div className="text-sm text-muted-foreground space-y-2">
                <p className="font-semibold">Test Accounts:</p>
                <div className="space-y-1 text-xs">
                  <div>
                    <span className="font-medium">Admin:</span>{" "}
                    <code className="bg-muted px-1 rounded">admin@pllm.local / admin</code>
                  </div>
                  <div>
                    <span className="font-medium">User:</span>{" "}
                    <code className="bg-muted px-1 rounded">user@pllm.local / user</code>
                  </div>
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}