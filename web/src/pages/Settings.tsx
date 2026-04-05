import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { useColorMode } from '@/contexts/ColorModeContext';

export default function Settings() {
  const { colorMode, setColorMode } = useColorMode();

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-semibold">Settings</h1>
        <p className="text-[13px] text-muted-foreground mt-1">
          Configure your dashboard preferences
        </p>
      </div>

      <div className="space-y-4">
        {/* Appearance */}
        <div className="bg-card border rounded-lg p-4">
          <div className="flex items-center gap-2 mb-3">
            <Icon icon={icons.settings} className="w-4 h-4 text-muted-foreground" />
            <div className="text-sm font-semibold">Appearance</div>
          </div>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-[13px] font-medium">Color Mode</div>
              <div className="text-[12px] text-muted-foreground">Switch between light and dark</div>
            </div>
            <div className="flex bg-secondary border border-border rounded-[6px] overflow-hidden">
              {(['light', 'dark', 'system'] as const).map((mode) => (
                <button
                  key={mode}
                  onClick={() => setColorMode(mode)}
                  className={cn(
                    "px-3 py-1.5 text-xs capitalize",
                    colorMode === mode ? "bg-primary/10 text-primary" : "text-muted-foreground"
                  )}
                >
                  {mode}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Gateway */}
        <div className="bg-card border rounded-lg p-4">
          <div className="flex items-center gap-2 mb-3">
            <Icon icon={icons.routes} className="w-4 h-4 text-muted-foreground" />
            <div className="text-sm font-semibold">Gateway</div>
          </div>
          <div className="space-y-3">
            <div className="flex items-center justify-between py-2 border-b border-border/50 last:border-0">
              <div>
                <div className="text-[13px] font-medium">API Endpoint</div>
                <div className="text-[12px] text-muted-foreground">OpenAI-compatible base URL</div>
              </div>
              <code className="text-[12px] bg-muted px-2 py-1 rounded text-muted-foreground">/v1</code>
            </div>
            <div className="flex items-center justify-between py-2">
              <div>
                <div className="text-[13px] font-medium">Admin API</div>
                <div className="text-[12px] text-muted-foreground">Admin management endpoints</div>
              </div>
              <code className="text-[12px] bg-muted px-2 py-1 rounded text-muted-foreground">/api/admin</code>
            </div>
          </div>
        </div>

        {/* Authentication */}
        <div className="bg-card border rounded-lg p-4">
          <div className="flex items-center gap-2 mb-3">
            <Icon icon={icons.guardrails} className="w-4 h-4 text-muted-foreground" />
            <div className="text-sm font-semibold">Authentication</div>
          </div>
          <div className="space-y-3">
            <div className="flex items-center justify-between py-2 border-b border-border/50">
              <div>
                <div className="text-[13px] font-medium">Session</div>
                <div className="text-[12px] text-muted-foreground">Authenticated via JWT or master key</div>
              </div>
              <span className="text-[12px] text-green-500 font-medium">Active</span>
            </div>
            <div className="flex items-center justify-between py-2">
              <div>
                <div className="text-[13px] font-medium">API Keys</div>
                <div className="text-[12px] text-muted-foreground">Manage keys from the Keys page</div>
              </div>
              <Icon icon={icons.keys} className="w-4 h-4 text-muted-foreground" />
            </div>
          </div>
        </div>

        {/* Danger Zone */}
        <div className="bg-card border border-destructive/50 rounded-lg p-4">
          <div className="text-sm font-semibold text-destructive mb-3">Danger Zone</div>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-[13px] font-medium">Reset Preferences</div>
              <div className="text-[12px] text-muted-foreground">Clear all locally stored settings and return to defaults</div>
            </div>
            <button
              onClick={() => {
                localStorage.clear();
                window.location.reload();
              }}
              className="px-3 py-1.5 text-xs border border-destructive/50 text-destructive rounded-[6px] hover:bg-destructive/10 transition-colors"
            >
              Reset
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
