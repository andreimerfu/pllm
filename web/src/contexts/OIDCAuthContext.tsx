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
  loginWithMasterKey: (masterKey: string) => Promise<void>;
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

userManager.events.addAccessTokenExpired(async () => {
  console.log("Access token expired");
  
  // Check if current user is a master key user
  try {
    const currentUser = await userManager.getUser();
    const isMasterKeyUser = currentUser?.profile?.master_key_auth === true || 
                           currentUser?.profile?.sub === "master-key-user";
    
    if (isMasterKeyUser) {
      console.log("Master key token expired, clearing session");
      await userManager.removeUser();
      localStorage.removeItem("authToken");
      // Redirect to login page
      window.location.href = "/ui/login";
      return;
    }
  } catch (error) {
    console.error("Error checking user during token expiration:", error);
  }
});

userManager.events.addSilentRenewError(async (error) => {
  console.error("Silent renew error:", error);
  
  // Check if this is a master key user - if so, stop automatic renewal
  try {
    const currentUser = await userManager.getUser();
    const isMasterKeyUser = currentUser?.profile?.master_key_auth === true || 
                           currentUser?.profile?.sub === "master-key-user";
    
    if (isMasterKeyUser) {
      console.log("Stopping silent renewal for master key user");
      userManager.stopSilentRenew();
      return;
    }
  } catch (err) {
    console.error("Error checking user during silent renew error:", err);
  }
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
        
        // Stop automatic renewal for master key users
        const isMasterKeyUser = currentUser.profile?.master_key_auth === true || 
                               currentUser.profile?.sub === "master-key-user";
        if (isMasterKeyUser) {
          userManager.stopSilentRenew();
        }
      } else if (currentUser?.expired) {
        // Check if this is a master key user - they don't support token renewal
        const isMasterKeyUser = currentUser.profile?.master_key_auth === true || 
                                currentUser.profile?.sub === "master-key-user";
        
        if (isMasterKeyUser) {
          console.log("Master key user session expired, clearing session and redirecting to login");
          setUser(null);
          localStorage.removeItem("authToken");
          await userManager.removeUser();
          // Use window.location for immediate redirect since we're in an async context
          window.location.href = "/ui/login";
          return;
        } else {
          // Try to silently renew the token for regular OAuth users
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
      
      // Enable automatic renewal for OAuth users (not master key users)
      if (user.refresh_token) {
        // This user has a refresh token, start silent renewal manually
        userManager.startSilentRenew();
      }
      
      // Get the return URL from state or default to dashboard
      const returnUrl = parsedState?.returnUrl || "/dashboard";
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
      navigate("/login", { replace: true });
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

  const loginWithMasterKey = async (masterKey: string) => {
    try {
      // Authenticate with master key through our backend
      const response = await fetch("/api/admin/auth/master-key", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ master_key: masterKey }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || "Invalid master key");
      }

      const tokenResponse = await response.json();
      
      // Create a User object for master key access
      const user = new User({
        access_token: tokenResponse.token, // Backend returns 'token', not 'access_token'
        id_token: tokenResponse.id_token || "",
        token_type: "Bearer",
        scope: "admin",
        expires_at: Math.floor(Date.now() / 1000) + (tokenResponse.expires_in || 86400), // 24 hours for master key
        profile: {
          sub: "master-key-user",
          iss: "pllm-master",
          aud: "pllm-admin",
          exp: Math.floor(Date.now() / 1000) + (tokenResponse.expires_in || 86400),
          iat: Math.floor(Date.now() / 1000),
          name: "Admin (Master Key)",
          email: "admin@pllm.local",
          role: "admin",
          groups: ["admin"],
          // Mark this as a master key user to skip renewal attempts
          master_key_auth: true,
        } as any,
      });

      // Store the user
      await userManager.storeUser(user);
      setUser(user);
      localStorage.setItem("authToken", user.access_token);
      
      // Stop automatic renewal for master key users
      userManager.stopSilentRenew();

      toast({
        title: "Success",
        description: "Logged in with master key",
      });

      navigate("/dashboard");
    } catch (error) {
      console.error("Master key login error:", error);
      toast({
        title: "Error",
        description: error instanceof Error ? error.message : "Master key login failed",
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
      navigate("/login");
      
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
      navigate("/login");
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
        loginWithMasterKey,
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