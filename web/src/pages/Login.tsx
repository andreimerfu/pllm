import { useState } from "react";
import { useNavigate } from "react-router-dom";
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
import { Icon } from "@iconify/react";
import { useToast } from "@/hooks/use-toast";

export default function Login() {
  const [masterKey, setMasterKey] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const navigate = useNavigate();
  const { toast } = useToast();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!masterKey.trim()) {
      toast({
        title: "Error",
        description: "Please enter a master key",
        variant: "destructive",
      });
      return;
    }

    setIsLoading(true);

    try {
      // For now, we'll use the master key directly as a Bearer token
      // In production, you'd want to exchange this for a JWT token
      localStorage.setItem("authToken", masterKey);

      // Test the key by making a simple API call
      const response = await fetch("http://localhost:8080/api/admin/stats", {
        headers: {
          Authorization: `Bearer ${masterKey}`,
        },
      });

      if (response.ok) {
        toast({
          title: "Success",
          description: "Logged in successfully",
        });
        navigate("/");
      } else {
        throw new Error("Invalid master key");
      }
    } catch (error) {
      toast({
        title: "Error",
        description: "Invalid master key",
        variant: "destructive",
      });
      localStorage.removeItem("authToken");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <div className="flex items-center justify-center mb-4">
            <div className="p-3 rounded-lg bg-primary/10">
              <Icon
                icon="lucide:shield"
                width="32"
                height="32"
                className="text-primary"
              />
            </div>
          </div>
          <CardTitle className="text-2xl text-center">Admin Login</CardTitle>
          <CardDescription className="text-center">
            Enter your master key to access the admin panel
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleLogin} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="masterKey">Master Key</Label>
              <Input
                id="masterKey"
                type="password"
                placeholder="Enter your master key"
                value={masterKey}
                onChange={(e) => setMasterKey(e.target.value)}
                disabled={isLoading}
                autoFocus
              />
            </div>
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading ? (
                <div className="flex items-center space-x-2">
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
                  <span>Logging in...</span>
                </div>
              ) : (
                <>
                  <Icon
                    icon="lucide:log-in"
                    width="16"
                    height="16"
                    className="mr-2"
                  />
                  Login
                </>
              )}
            </Button>
          </form>

          <div className="mt-6 pt-6 border-t">
            <div className="text-sm text-muted-foreground">
              <p className="font-semibold mb-2">Default Master Key:</p>
              <code className="bg-muted px-2 py-1 rounded text-xs">
                sk-pllm-test-key-2024
              </code>
              <p className="mt-2 text-xs">
                For production, set the MASTER_KEY environment variable
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
