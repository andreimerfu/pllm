import { useEffect } from "react";
import { UserManager } from "oidc-client-ts";
import oidcConfig from "@/config/auth";

export default function SilentRenew() {
  useEffect(() => {
    const userManager = new UserManager(oidcConfig);
    userManager.signinSilentCallback().catch((error) => {
      console.error("Silent renew error:", error);
    });
  }, []);

  return <div>Renewing session...</div>;
}