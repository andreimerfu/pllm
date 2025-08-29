import { UserManagerSettings, WebStorageStateStore } from "oidc-client-ts";

const oidcConfig: UserManagerSettings = {
  // Dex public issuer URL (for browser OAuth flows)
  authority: import.meta.env.VITE_DEX_PUBLIC_AUTHORITY || import.meta.env.VITE_DEX_AUTHORITY || "http://dex.local/dex",
  
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
    issuer: import.meta.env.VITE_DEX_PUBLIC_AUTHORITY || "http://dex.local/dex",
    authorization_endpoint: (import.meta.env.VITE_DEX_PUBLIC_AUTHORITY || "http://dex.local/dex") + "/auth",
    token_endpoint: `${window.location.origin}/api/admin/auth/token`, // Use our backend token endpoint
    userinfo_endpoint: `${window.location.origin}/api/admin/auth/userinfo`, // Use our backend userinfo endpoint
    jwks_uri: (import.meta.env.VITE_DEX_PUBLIC_AUTHORITY || "http://dex.local/dex") + "/keys",
    // Note: Dex doesn't provide a logout endpoint, we handle logout client-side
  },
};

export default oidcConfig;