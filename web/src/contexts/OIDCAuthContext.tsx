import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { User, UserManager } from "oidc-client-ts";
import oidcConfig from "@/config/auth";
import { useNavigate } from "react-router-dom";
import { useToast } from "@/hooks/use-toast";

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

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: () => Promise<void>;
  logout: () => Promise<void>;
  loginWithCredentials: (username: string, password: string) => Promise<void>;
  getAccessToken: () => string | null;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Create UserManager instance
const userManager = new UserManager(oidcConfig);

// Setup event handlers
userManager.events.addUserLoaded((user) => {
  console.log("User loaded:", user);
});

userManager.events.addUserUnloaded(() => {
  console.log("User unloaded");
});

userManager.events.addAccessTokenExpired(() => {
  console.log("Access token expired");
});

userManager.events.addSilentRenewError((error) => {
  console.error("Silent renew error:", error);
});

export function OIDCAuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const navigate = useNavigate();
  const { toast } = useToast();

  useEffect(() => {
    // Check if returning from redirect
    if (window.location.pathname === "/ui/callback" || window.location.pathname === "/callback") {
      console.log("Handling callback...");
      handleCallback();
      return;
    }

    // Check for existing user session
    loadUser();
  }, []);

  const loadUser = async () => {
    try {
      const currentUser = await userManager.getUser();
      if (currentUser && !currentUser.expired) {
        setUser(currentUser);
        // Set the token in localStorage for API calls
        localStorage.setItem("authToken", currentUser.access_token);
      } else if (currentUser?.expired) {
        // Try to silently renew the token
        try {
          const renewedUser = await userManager.signinSilent();
          if (renewedUser) {
            setUser(renewedUser);
            localStorage.setItem("authToken", renewedUser.access_token);
          }
        } catch (error) {
          console.error("Silent renew failed:", error);
          setUser(null);
          localStorage.removeItem("authToken");
        }
      }
    } catch (error) {
      console.error("Error loading user:", error);
      setUser(null);
      localStorage.removeItem("authToken");
    } finally {
      setIsLoading(false);
    }
  };

  const handleCallback = async () => {
    try {
      console.log("Processing callback...");
      
      // Get the authorization code from URL params
      const urlParams = new URLSearchParams(window.location.search);
      const code = urlParams.get('code');
      const state = urlParams.get('state');
      
      if (!code) {
        throw new Error("No authorization code received");
      }
      
      console.log("Got authorization code, exchanging for token...");
      
      // Get the PKCE verifier from session storage
      const codeVerifier = sessionStorage.getItem('code_verifier');
      console.log("Code verifier found:", !!codeVerifier);
      
      // Exchange the code for tokens using our backend endpoint
      const response = await fetch("/api/admin/auth/token", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          code: code,
          redirect_uri: `${window.location.origin}/ui/callback`,
          code_verifier: codeVerifier || undefined,
        }),
      });
      
      // Clear the code verifier after use
      sessionStorage.removeItem('code_verifier');
      
      if (!response.ok) {
        const errorData = await response.json();
        console.error("Token exchange error:", errorData);
        throw new Error(errorData.error_description || "Token exchange failed");
      }
      
      const tokenResponse = await response.json();
      console.log("Token exchange successful");
      
      // Fetch user info using the access token
      const userInfoResponse = await fetch("/api/admin/auth/userinfo", {
        method: "GET",
        headers: {
          "Authorization": `Bearer ${tokenResponse.access_token}`,
        },
      });
      
      let userProfile: any = {
        sub: "oauth-user",
        iss: oidcConfig.authority!,
        aud: oidcConfig.client_id!,
        exp: Math.floor(Date.now() / 1000) + (tokenResponse.expires_in || 3600),
        iat: Math.floor(Date.now() / 1000),
      };
      
      if (userInfoResponse.ok) {
        const userInfo = await userInfoResponse.json();
        console.log("User info fetched:", userInfo);
        // Merge the user info with the profile
        userProfile = {
          ...userProfile,
          ...userInfo,
          name: userInfo.name || userInfo.preferred_username || userInfo.email,
          email: userInfo.email,
          picture: userInfo.picture,
          groups: userInfo.groups || [],
        };
      } else {
        console.error("Failed to fetch user info");
      }
      
      // Parse state if available
      let parsedState: any = {};
      try {
        if (state) {
          parsedState = JSON.parse(atob(state));
        }
      } catch (e) {
        console.error("Failed to parse state:", e);
      }
      
      // Create a User object from the token response
      const user = new User({
        access_token: tokenResponse.access_token,
        id_token: tokenResponse.id_token,
        refresh_token: tokenResponse.refresh_token,
        token_type: tokenResponse.token_type,
        scope: tokenResponse.scope,
        expires_at: Math.floor(Date.now() / 1000) + (tokenResponse.expires_in || 3600),
        profile: userProfile,
      });
      
      // Store the user
      await userManager.storeUser(user);
      setUser(user);
      localStorage.setItem("authToken", user.access_token);
      
      // Get the return URL from state or default to dashboard
      const returnUrl = parsedState?.returnUrl || "/ui/dashboard";
      console.log("Navigating to:", returnUrl);
      
      // Use replace to avoid back button issues
      navigate(returnUrl, { replace: true });
      
      toast({
        title: "Success",
        description: "Logged in successfully",
      });
      
      // Update loading state
      setIsLoading(false);
    } catch (error) {
      console.error("Callback error:", error);
      toast({
        title: "Error",
        description: error instanceof Error ? error.message : "Failed to complete login",
        variant: "destructive",
      });
      setIsLoading(false);
      navigate("/ui/login", { replace: true });
    }
  };

  const login = async () => {
    try {
      // Store current location to return after login
      const returnUrl = window.location.pathname;
      
      // Generate PKCE challenge
      const codeVerifier = generateCodeVerifier();
      const codeChallenge = await generateCodeChallenge(codeVerifier);
      
      // Store the code verifier for the callback
      sessionStorage.setItem('code_verifier', codeVerifier);
      
      // Build the authorization URL with PKCE
      const params = new URLSearchParams({
        client_id: oidcConfig.client_id!,
        redirect_uri: oidcConfig.redirect_uri!,
        response_type: 'code',
        scope: 'openid profile email',
        state: btoa(JSON.stringify({ returnUrl })),
        code_challenge: codeChallenge,
        code_challenge_method: 'S256',
      });
      
      const authUrl = `${oidcConfig.authority}/auth?${params.toString()}`;
      console.log("Redirecting to:", authUrl);
      
      // Redirect to the authorization endpoint
      window.location.href = authUrl;
    } catch (error) {
      console.error("Login error:", error);
      toast({
        title: "Error",
        description: "Failed to initiate login",
        variant: "destructive",
      });
    }
  };

  const loginWithCredentials = async (username: string, password: string) => {
    try {
      // For static credentials, we'll use the resource owner password flow
      // This requires Dex to be configured to accept password grants
      const response = await fetch("http://localhost:5556/dex/token", {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: new URLSearchParams({
          grant_type: "password",
          username: username,
          password: password,
          client_id: oidcConfig.client_id!,
          client_secret: oidcConfig.client_secret || "",
          scope: oidcConfig.scope!,
        }),
      });

      if (!response.ok) {
        throw new Error("Invalid credentials");
      }

      const tokenResponse = await response.json();
      
      // Create a User object from the token response
      const user = new User({
        access_token: tokenResponse.access_token,
        id_token: tokenResponse.id_token,
        refresh_token: tokenResponse.refresh_token,
        token_type: tokenResponse.token_type,
        scope: tokenResponse.scope,
        expires_at: Math.floor(Date.now() / 1000) + tokenResponse.expires_in,
        profile: {
          sub: "password-user",
          iss: oidcConfig.authority!,
          aud: oidcConfig.client_id!,
          exp: Math.floor(Date.now() / 1000) + tokenResponse.expires_in,
          iat: Math.floor(Date.now() / 1000),
        } as any,
      });

      // Store the user
      await userManager.storeUser(user);
      
      // Load user info
      const userWithInfo = await userManager.getUser();
      setUser(userWithInfo);
      
      if (userWithInfo) {
        localStorage.setItem("authToken", userWithInfo.access_token);
      }

      toast({
        title: "Success",
        description: "Logged in successfully",
      });

      navigate("/dashboard");
    } catch (error) {
      console.error("Login with credentials error:", error);
      toast({
        title: "Error",
        description: error instanceof Error ? error.message : "Login failed",
        variant: "destructive",
      });
      throw error;
    }
  };

  const logout = async () => {
    try {
      // Clear local session
      await userManager.removeUser();
      await userManager.clearStaleState();
      setUser(null);
      localStorage.removeItem("authToken");
      sessionStorage.clear();
      
      // Navigate to login page
      navigate("/ui/login");
      
      toast({
        title: "Success",
        description: "Logged out successfully",
      });
    } catch (error) {
      console.error("Logout error:", error);
      // Fallback: clear everything anyway
      setUser(null);
      localStorage.removeItem("authToken");
      sessionStorage.clear();
      navigate("/ui/login");
    }
  };

  const getAccessToken = () => {
    return user?.access_token || null;
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user && !user.expired,
        isLoading,
        login,
        logout,
        loginWithCredentials,
        getAccessToken,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an OIDCAuthProvider");
  }
  return context;
}