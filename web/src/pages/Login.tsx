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
  const { loginWithCredentials } = useAuth();
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

  const handleOAuthLogin = async (provider: 'github' | 'google' | 'microsoft') => {
    // Redirect directly to the specific OAuth provider through Dex
    // This bypasses the Dex UI selection screen
    const returnUrl = window.location.pathname;
    
    // Generate PKCE challenge
    const codeVerifier = generateCodeVerifier();
    const codeChallenge = await generateCodeChallenge(codeVerifier);
    
    // Store the code verifier for the callback
    sessionStorage.setItem('code_verifier', codeVerifier);
    
    // Build the authorization URL with the connector parameter
    const params = new URLSearchParams({
      client_id: 'pllm-web',
      redirect_uri: `${window.location.origin}/ui/callback`,
      response_type: 'code',
      scope: 'openid profile email',
      state: btoa(JSON.stringify({ returnUrl })),
      code_challenge: codeChallenge,
      code_challenge_method: 'S256',
      // This tells Dex to skip the selection screen and go directly to the provider
      connector_id: provider,
    });
    
    const authUrl = `http://localhost:5556/dex/auth?${params.toString()}`;
    console.log(`Redirecting to ${provider}:`, authUrl);
    
    // Redirect to the authorization endpoint
    window.location.href = authUrl;
  };

  // PKCE helper functions
  function generateCodeVerifier(): string {
    const array = new Uint8Array(32);
    crypto.getRandomValues(array);
    return btoa(String.fromCharCode.apply(null, Array.from(array)))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=/g, '');
  }

  async function generateCodeChallenge(verifier: string): Promise<string> {
    const encoder = new TextEncoder();
    const data = encoder.encode(verifier);
    const digest = await crypto.subtle.digest('SHA-256', data);
    return btoa(String.fromCharCode.apply(null, Array.from(new Uint8Array(digest))))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=/g, '');
  }

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
              onClick={() => handleOAuthLogin('github')}
              disabled={isLoading}
            >
              <Github className="mr-2 h-4 w-4" />
              Continue with GitHub
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={() => handleOAuthLogin('google')}
              disabled={isLoading}
            >
              <Mail className="mr-2 h-4 w-4" />
              Continue with Google
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={() => handleOAuthLogin('microsoft')}
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