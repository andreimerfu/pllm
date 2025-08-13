import { UserManagerSettings, WebStorageStateStore } from "oidc-client-ts";

const oidcConfig: UserManagerSettings = {
  // Dex issuer URL
  authority: import.meta.env.VITE_DEX_AUTHORITY || "http://localhost:5556/dex",
  
  // OAuth2 client configuration
  client_id: import.meta.env.VITE_DEX_CLIENT_ID || "pllm-web",
  client_secret: import.meta.env.VITE_DEX_CLIENT_SECRET || "pllm-web-secret",
  
  // Redirect URIs
  redirect_uri: `${window.location.origin}/ui/callback`,
  post_logout_redirect_uri: `${window.location.origin}/ui`,
  silent_redirect_uri: `${window.location.origin}/ui/silent-renew`,
  
  // Response type for authorization code flow
  response_type: "code",
  
  // Scopes to request
  scope: "openid profile email groups offline_access",
  
  // Load user info after login
  loadUserInfo: true,
  
  // Automatic silent renew
  automaticSilentRenew: true,
  
  // Store tokens in localStorage (can be changed to sessionStorage)
  userStore: new WebStorageStateStore({ store: window.localStorage }),
  
  // Additional metadata
  metadata: {
    issuer: "http://localhost:5556/dex",
    authorization_endpoint: "http://localhost:5556/dex/auth",
    token_endpoint: "http://localhost:5556/dex/token",
    userinfo_endpoint: "http://localhost:5556/dex/userinfo",
    jwks_uri: "http://localhost:5556/dex/keys",
    // Note: Dex doesn't provide a logout endpoint, we handle logout client-side
  },
};

export default oidcConfig;