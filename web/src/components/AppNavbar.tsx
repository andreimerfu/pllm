import { useState, useEffect } from "react";
import { useLocation, Link, useNavigate } from "react-router-dom";
import { SidebarTrigger } from "@/components/ui/sidebar";
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from "@/components/ui/command";
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { useColorMode } from '@/contexts/ColorModeContext';
import { useConfig } from '@/contexts/ConfigContext';
import { useAuth } from '@/contexts/OIDCAuthContext';
import { usePageChrome } from '@/components/common/page-chrome';
import { cn } from '@/lib/utils';

// Route metadata keeps the breadcrumb group/page mapping and the ⌘K
// palette in sync with app-sidebar.tsx. When the sidebar gains an item
// add it here too. Groups mirror the sidebar IA (AI Registry first).
type RouteMeta = { label: string; group: string; icon: string };

const routes: Record<string, RouteMeta> = {
  "/dashboard":     { label: "Dashboard",    group: "Overview",          icon: icons.dashboard },
  "/registry":      { label: "Catalog",      group: "AI Registry",       icon: icons.registry },
  "/agents":        { label: "Agents",       group: "AI Registry",       icon: icons.agents },
  "/mcp-servers":   { label: "MCP Servers",  group: "AI Registry",       icon: icons.mcp },
  "/deployments":   { label: "Deployments",  group: "AI Registry",       icon: icons.rocket },
  "/federation":    { label: "Federation",   group: "AI Registry",       icon: icons.federation },
  "/models":        { label: "Models",       group: "Gateway",           icon: icons.models },
  "/routes":        { label: "Routes",       group: "Gateway",           icon: icons.routes },
  "/providers":     { label: "Providers",    group: "Gateway",           icon: icons.globe },
  "/guardrails":    { label: "Guardrails",   group: "Gateway",           icon: icons.guardrails },
  "/playground":    { label: "Playground",   group: "Gateway",           icon: icons.terminal },
  "/chat":          { label: "Playground",   group: "Gateway",           icon: icons.terminal },
  "/use-cases":     { label: "Use Cases",    group: "Governance",        icon: icons.useCase },
  "/compliance":    { label: "Compliance",   group: "Governance",        icon: icons.shield },
  "/policies":      { label: "Policies",     group: "Governance",        icon: icons.policies },
  "/keys":          { label: "API Keys",     group: "Access & Activity", icon: icons.keys },
  "/teams":         { label: "Teams",        group: "Access & Activity", icon: icons.teams },
  "/users":         { label: "Users",        group: "Access & Activity", icon: icons.users },
  "/budget":        { label: "Budget",       group: "Access & Activity", icon: icons.budget },
  "/audit-logs":    { label: "Audit Logs",   group: "Access & Activity", icon: icons.auditLogs },
  "/notifications": { label: "Notifications", group: "System",           icon: icons.notifications },
  "/settings":      { label: "Settings",     group: "System",            icon: icons.settings },
};

function matchBaseRoute(pathname: string): string | null {
  const prefixes = Object.keys(routes).sort((a, b) => b.length - a.length);
  for (const p of prefixes) {
    if (pathname === p || pathname.startsWith(p + "/")) return p;
  }
  return null;
}

function prettyDynamic(segment: string): string {
  try {
    const decoded = decodeURIComponent(segment);
    if (decoded.length > 40) return decoded.substring(0, 40) + "…";
    return decoded;
  } catch {
    return segment;
  }
}

export function AppNavbar() {
  const location = useLocation();
  const navigate = useNavigate();
  const { resolvedMode, setColorMode } = useColorMode();
  const { config } = useConfig();
  const { user } = useAuth();
  const { actions, entityCrumb } = usePageChrome();
  const [commandOpen, setCommandOpen] = useState(false);

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setCommandOpen((prev) => !prev);
      }
    };
    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, []);

  const basePath = matchBaseRoute(location.pathname);
  const meta = basePath ? routes[basePath] : null;

  // Skills/Prompts are catalog facets; reflect the active facet in the crumb.
  const kindParam = new URLSearchParams(location.search).get("kind");
  const facetLabel =
    basePath === "/registry" && kindParam === "skill"
      ? "Skills"
      : basePath === "/registry" && kindParam === "prompt"
        ? "Prompts"
        : null;

  // Build the crumb trail. Group ▸ Page ▸ (Entity). Drilled-in entity
  // comes from PageEntityCrumb (drawers without routing) or the URL tail.
  const crumbs: Array<{ label: string; to?: string; mono?: boolean }> = [];
  if (meta) {
    crumbs.push({ label: meta.group });
    crumbs.push({ label: facetLabel ?? meta.label, to: basePath ?? undefined });
    const extra = basePath ? location.pathname.slice(basePath.length).replace(/^\//, "") : "";
    if (entityCrumb) {
      crumbs.push({ label: entityCrumb, mono: true });
    } else if (extra) {
      const segs = extra.split("/").filter(Boolean).filter((s) => !["edit", "new"].includes(s));
      const tail = segs[segs.length - 1];
      if (tail) crumbs.push({ label: prettyDynamic(tail), mono: true });
    }
  } else {
    crumbs.push({ label: "pLLM" });
  }

  const env = config?.environment?.trim();
  const envPaletteMap: Record<string, string> = {
    prod: "bg-bad/10 text-bad ring-bad/30",
    production: "bg-bad/10 text-bad ring-bad/30",
    staging: "bg-warn/10 text-warn ring-warn/30",
    dev: "bg-good/10 text-good ring-good/30",
    development: "bg-good/10 text-good ring-good/30",
    local: "bg-surface-2 text-text-dim ring-border",
  };
  const envPalette = env ? envPaletteMap[env.toLowerCase()] ?? envPaletteMap.local : null;

  const userName =
    user?.profile?.name ||
    user?.profile?.preferred_username ||
    user?.profile?.email ||
    "User";
  const userInitials =
    userName
      .split(/\s+/)
      .map((n: string) => n.match(/[A-Za-z]/)?.[0] ?? "")
      .join("")
      .toUpperCase()
      .slice(0, 2) || "U";

  return (
    <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center gap-3.5 border-b border-border bg-card px-5">
      <SidebarTrigger className="h-[34px] w-[34px] rounded-[9px] border border-border bg-card text-muted-foreground hover:bg-surface-2 hover:text-foreground" />

      <nav className="flex min-w-0 items-center gap-2 text-[13.5px]" aria-label="Breadcrumb">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <div key={`${c.label}-${i}`} className="flex min-w-0 items-center gap-2">
              {i > 0 && (
                <Icon icon={icons.chevronRight} className="h-3.5 w-3.5 shrink-0 text-text-dim" />
              )}
              {last ? (
                <span
                  className={cn(
                    "max-w-[320px] truncate font-semibold text-foreground",
                    c.mono && "font-mono text-[12.5px] font-medium",
                  )}
                >
                  {c.label}
                </span>
              ) : c.to ? (
                <Link to={c.to} className="max-w-[220px] truncate text-text-dim hover:text-foreground">
                  {c.label}
                </Link>
              ) : (
                <span className="truncate text-text-dim">{c.label}</span>
              )}
            </div>
          );
        })}
      </nav>

      {/* Back-compat slot. Pages should move CTAs into their own header
          via <PageShell actions={...}> — this slot will be removed when
          every page is migrated. Rendered small to discourage new use. */}
      {actions && (
        <div className="ml-3 flex items-center gap-2 [&_button]:h-8 [&_button]:text-[12px]">
          {actions}
        </div>
      )}

      <div className="ml-auto flex items-center gap-2.5">
        {env && envPalette && (
          <span
            className={cn(
              "hidden items-center rounded-full px-2 py-0.5 font-mono text-[10px] uppercase tracking-widest ring-1 sm:inline-flex",
              envPalette,
            )}
            title={`Environment: ${env}`}
          >
            {env}
          </span>
        )}
        <button
          onClick={() => setCommandOpen(true)}
          className="flex h-[34px] min-w-[220px] items-center gap-2.5 rounded-[9px] border border-border bg-surface-2 px-3 text-[13px] text-text-dim transition-colors hover:border-border-strong"
        >
          <Icon icon={icons.search} className="h-[15px] w-[15px]" />
          <span className="flex-1 text-left">Search assets, models, policies…</span>
          <kbd className="rounded-md border border-border bg-card px-1.5 py-px font-mono text-[10.5px] text-text-dim">
            ⌘K
          </kbd>
        </button>
        <button
          onClick={() => setColorMode(resolvedMode === 'dark' ? 'light' : 'dark')}
          className="flex h-[34px] items-center gap-2 rounded-[9px] border border-border bg-card px-2.5 text-[12.5px] font-medium text-muted-foreground transition-colors hover:bg-surface-2 hover:text-foreground"
          aria-label="Toggle theme"
        >
          <Icon icon={resolvedMode === 'dark' ? icons.sun : icons.moon} className="h-[17px] w-[17px]" />
          <span className="hidden sm:inline">{resolvedMode === 'dark' ? "Dark" : "Light"}</span>
        </button>
        <div className="h-[22px] w-px bg-border" />
        <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--accent-soft)] text-[12px] font-bold text-primary">
          {userInitials}
        </span>
      </div>

      <CommandDialog open={commandOpen} onOpenChange={setCommandOpen}>
        <CommandInput placeholder="Search pages…" />
        <CommandList>
          <CommandEmpty>No results found.</CommandEmpty>
          {Array.from(new Set(Object.values(routes).map((r) => r.group))).map((group) => (
            <CommandGroup key={group} heading={group}>
              {Object.entries(routes)
                .filter(([path, r]) => r.group === group && path !== "/chat")
                .map(([path, r]) => (
                  <CommandItem
                    key={path}
                    onSelect={() => {
                      navigate(path);
                      setCommandOpen(false);
                    }}
                  >
                    <Icon icon={r.icon} className="mr-2 h-4 w-4" />
                    {r.label}
                  </CommandItem>
                ))}
            </CommandGroup>
          ))}
        </CommandList>
      </CommandDialog>
    </header>
  );
}
