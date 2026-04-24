import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { getConfig } from "@/lib/api";

interface ConfigContextType {
  config: {
    master_key_configured: boolean;
    dex_enabled: boolean;
    database_connected: boolean;
    environment?: string;
  } | null;
  // Optional capability flags. "mrm" gates Use Cases + Compliance;
  // "webhooks" gates the outbound-events section. Missing keys are false.
  features: Record<string, boolean>;
  isLoading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

const ConfigContext = createContext<ConfigContextType | undefined>(undefined);

export function ConfigProvider({ children }: { children: ReactNode }) {
  const [config, setConfig] = useState<ConfigContextType['config']>(null);
  const [features, setFeatures] = useState<Record<string, boolean>>({});
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchConfig = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const response = await getConfig() as any;
      setConfig(response.config);
      setFeatures(response.features ?? {});
    } catch (err) {
      console.error("Failed to fetch config:", err);
      setError(err instanceof Error ? err.message : "Failed to fetch configuration");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchConfig();
  }, []);

  return (
    <ConfigContext.Provider
      value={{
        config,
        features,
        isLoading,
        error,
        refetch: fetchConfig,
      }}
    >
      {children}
    </ConfigContext.Provider>
  );
}

export function useConfig() {
  const context = useContext(ConfigContext);
  if (context === undefined) {
    throw new Error("useConfig must be used within a ConfigProvider");
  }
  return context;
}