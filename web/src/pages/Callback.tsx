import { useEffect } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

export default function Callback() {
  useEffect(() => {
    // The OIDCAuthContext handles the callback logic
    // This component just shows a loading state
    console.log("Callback component mounted");
    console.log("Current URL:", window.location.href);
    console.log("URL params:", window.location.search);
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="flex flex-col items-center space-y-4">
        <Icon icon={icons.loader} className="h-8 w-8 animate-spin" />
        <p className="text-lg">Completing login...</p>
        <p className="text-sm text-muted-foreground">Processing authentication...</p>
      </div>
    </div>
  );
}