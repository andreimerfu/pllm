import { UserManagerSettings, WebStorageStateStore } from "oidc-client-ts";

// Dynamic Dex URL based on environment or current host
const getDexAuthority = () => {
  // First try environment variables
  if (import.meta.env.VITE_DEX_PUBLIC_AUTHORITY) {
    return import.meta.env.VITE_DEX_PUBLIC_AUTHORITY;
  }
  if (import.meta.env.VITE_DEX_AUTHORITY) {
    return import.meta.env.VITE_DEX_AUTHORITY;
  }
  
  // Fallback: derive from current location
  const protocol = window.location.protocol;
  const hostname = window.location.hostname;
  
  // In Docker Compose: use localhost:5556
  // In Kubernetes: use dex service or ingress
  if (hostname === 'localhost' || hostname === '127.0.0.1') {
    return `${protocol}//localhost:5556/dex`;
  } else {
    // Production/Kubernetes: assume dex is at /dex path on same host
    return `${protocol}//${hostname}/dex`;
  }
};

const dexAuthority = getDexAuthority();

const oidcConfig: UserManagerSettings = {
  // Dex public issuer URL (for browser OAuth flows)
  authority: dexAuthority,
  
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
    issuer: dexAuthority,
    authorization_endpoint: dexAuthority + "/auth",
    token_endpoint: `${window.location.origin}/api/admin/auth/token`, // Use our backend token endpoint
    userinfo_endpoint: `${window.location.origin}/api/admin/auth/userinfo`, // Use our backend userinfo endpoint
    jwks_uri: dexAuthority + "/keys",
    // Note: Dex doesn't provide a logout endpoint, we handle logout client-side
  },
};

export default oidcConfig;