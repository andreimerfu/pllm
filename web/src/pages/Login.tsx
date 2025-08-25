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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Icon } from "@iconify/react";
import { useToast } from "@/hooks/use-toast";
import { Loader2, Github, Mail, Shield, Key } from "lucide-react";

export default function Login() {
  const [masterKey, setMasterKey] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [showMasterKeyDialog, setShowMasterKeyDialog] = useState(false);
  const { loginWithMasterKey } = useAuth();
  const { toast } = useToast();

  const handleMasterKeyLogin = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!masterKey.trim()) {
      toast({
        title: "Error",
        description: "Please enter master key",
        variant: "destructive",
      });
      return;
    }

    setIsLoading(true);
    try {
      await loginWithMasterKey(masterKey);
      setShowMasterKeyDialog(false);
      setMasterKey("");
    } catch (error) {
      // Error is handled in the context
    } finally {
      setIsLoading(false);
    }
  };

  // const handleDexLogin = async () => {
  //   setIsLoading(true);
  //   try {
  //     await login();
  //   } catch (error) {
  //     // Error is handled in the context
  //   } finally {
  //     setIsLoading(false);
  //   }
  // };

  const handleOAuthLogin = async (
    provider: "github" | "google" | "microsoft",
  ) => {
    // Redirect directly to the specific OAuth provider through Dex
    // This bypasses the Dex UI selection screen
    const returnUrl = window.location.pathname;

    // Generate PKCE challenge
    const codeVerifier = generateCodeVerifier();
    const codeChallenge = await generateCodeChallenge(codeVerifier);

    // Store the code verifier for the callback
    sessionStorage.setItem("code_verifier", codeVerifier);

    // Build the authorization URL with the connector parameter
    const params = new URLSearchParams({
      client_id: "pllm-web",
      redirect_uri: `${window.location.origin}/ui/callback`,
      response_type: "code",
      scope: "openid profile email",
      state: btoa(JSON.stringify({ returnUrl })),
      code_challenge: codeChallenge,
      code_challenge_method: "S256",
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
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=/g, "");
  }

  async function generateCodeChallenge(verifier: string): Promise<string> {
    const encoder = new TextEncoder();
    const data = encoder.encode(verifier);
    const digest = await crypto.subtle.digest("SHA-256", data);
    return btoa(
      String.fromCharCode.apply(null, Array.from(new Uint8Array(digest))),
    )
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=/g, "");
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
            Authenticate to access the admin panel
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Primary Login Button */}
          {/*<Button
            className="w-full h-12 text-base font-semibold shadow-lg hover:shadow-xl transition-all duration-200"
            onClick={handleDexLogin}
            disabled={isLoading}
          >
            {isLoading ? (
              <>
                <Loader2 className="mr-2 h-5 w-5 animate-spin" />
                Connecting...
              </>
            ) : (
              <>
                <Shield className="mr-2 h-5 w-5" />
                Login with Dex
              </>
            )}
          </Button>*/}

          {/* OAuth Provider Buttons */}
          {/*<div className="space-y-2">*/}
          {/*<p className="text-xs text-center text-muted-foreground mb-3">
              Or choose a specific provider:
            </p>*/}
          <Button
            variant="outline"
            className="w-full"
            onClick={() => handleOAuthLogin("github")}
            disabled={isLoading}
          >
            <Github className="mr-2 h-4 w-4" />
            Continue with GitHub
          </Button>
          <Button
            variant="outline"
            className="w-full"
            onClick={() => handleOAuthLogin("google")}
            disabled={isLoading}
          >
            <Mail className="mr-2 h-4 w-4" />
            Continue with Google
          </Button>
          <Button
            variant="outline"
            className="w-full"
            onClick={() => handleOAuthLogin("microsoft")}
            disabled={isLoading}
          >
            <Icon icon="mdi:microsoft" className="mr-2 h-4 w-4" />
            Continue with Microsoft
          </Button>
          {/*</div>*/}

          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <Separator />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-background px-2 text-muted-foreground">
                Admin Access
              </span>
            </div>
          </div>

          {/* Master Key Access */}
          <Dialog
            open={showMasterKeyDialog}
            onOpenChange={setShowMasterKeyDialog}
          >
            <DialogTrigger asChild>
              <Button
                variant="ghost"
                className="w-full text-xs text-muted-foreground hover:text-foreground"
              >
                <Key className="mr-2 h-3 w-3" />
                Admin Login with Master Key
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Admin Access</DialogTitle>
                <DialogDescription>
                  Enter the master key to access admin functions directly.
                </DialogDescription>
              </DialogHeader>
              <form onSubmit={handleMasterKeyLogin} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="masterKey">Master Key</Label>
                  <Input
                    id="masterKey"
                    type="password"
                    placeholder="Enter master key"
                    value={masterKey}
                    onChange={(e) => setMasterKey(e.target.value)}
                    disabled={isLoading}
                    autoComplete="off"
                  />
                </div>
                <div className="flex justify-end space-x-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setShowMasterKeyDialog(false);
                      setMasterKey("");
                    }}
                    disabled={isLoading}
                  >
                    Cancel
                  </Button>
                  <Button type="submit" disabled={isLoading}>
                    {isLoading ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        Authenticating...
                      </>
                    ) : (
                      "Login"
                    )}
                  </Button>
                </div>
              </form>
            </DialogContent>
          </Dialog>
        </CardContent>
      </Card>
    </div>
  );
}
