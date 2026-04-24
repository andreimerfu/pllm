import { useState, useEffect } from "react";
import { useLocation, Link, useNavigate } from "react-router-dom";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
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
import { usePageChrome } from '@/components/common/page-chrome';
import { cn } from '@/lib/utils';

// Route metadata keeps the breadcrumb group/page mapping and the ⌘K
// palette in sync with app-sidebar.tsx. When the sidebar gains an item
// add it here too.
type RouteMeta = { label: string; group: string; icon: string };

const routes: Record<string, RouteMeta> = {
  "/dashboard":     { label: "Dashboard",    group: "Overview",         icon: icons.dashboard },
  "/chat":          { label: "Playground",   group: "LLM Gateway",      icon: icons.chat },
  "/models":        { label: "Models",       group: "LLM Gateway",      icon: icons.models },
  "/routes":        { label: "Routes",       group: "LLM Gateway",      icon: icons.routes },
  "/providers":     { label: "Providers",    group: "LLM Gateway",      icon: icons.globe },
  "/guardrails":    { label: "Guardrails",   group: "LLM Gateway",      icon: icons.guardrails },
  "/registry":      { label: "Registry",     group: "Agents & MCP",     icon: icons.registry },
  "/mcp-servers":   { label: "MCP Servers",  group: "Agents & MCP",     icon: icons.mcp },
  "/deployments":   { label: "Deployments",  group: "Agents & MCP",     icon: icons.deployment },
  "/use-cases":     { label: "Use Cases",    group: "Governance",       icon: icons.useCase },
  "/compliance":    { label: "Compliance",   group: "Governance",       icon: icons.shield },
  "/keys":          { label: "API Keys",     group: "Access & Activity", icon: icons.keys },
  "/teams":         { label: "Teams",        group: "Access & Activity", icon: icons.teams },
  "/users":         { label: "Users",        group: "Access & Activity", icon: icons.users },
  "/budget":        { label: "Budget",       group: "Access & Activity", icon: icons.budget },
  "/audit-logs":    { label: "Audit Logs",   group: "Access & Activity", icon: icons.auditLogs },
  "/settings":      { label: "Settings",     group: "System",           icon: icons.settings },
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

  // Build the crumb trail. Group ▸ Page ▸ (Entity). Drilled-in entity
  // comes from PageEntityCrumb (drawers without routing) or the URL tail.
  const crumbs: Array<{ label: string; to?: string; mono?: boolean }> = [];
  if (meta) {
    crumbs.push({ label: meta.group });
    crumbs.push({ label: meta.label, to: basePath ?? undefined });
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
    prod: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
    production: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
    staging: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
    dev: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-emerald-500/30",
    development: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-emerald-500/30",
    local: "bg-slate-500/10 text-slate-700 dark:text-slate-300 ring-slate-500/30",
  };
  const envPalette = env ? envPaletteMap[env.toLowerCase()] ?? envPaletteMap.local : null;

  return (
    <header className="sticky top-0 z-10 flex h-11 shrink-0 items-center gap-2 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/70 px-4">
      <SidebarTrigger className="-ml-1 h-7 w-7" />
      <Separator orientation="vertical" className="h-4" />

      <Breadcrumb className="min-w-0">
        <BreadcrumbList className="flex-nowrap">
          {crumbs.map((c, i) => {
            const last = i === crumbs.length - 1;
            return (
              <div key={`${c.label}-${i}`} className="flex items-center gap-1.5 min-w-0">
                {i > 0 && <BreadcrumbSeparator />}
                <BreadcrumbItem>
                  {last ? (
                    <BreadcrumbPage
                      className={cn(
                        "max-w-[320px] truncate text-[13px]",
                        c.mono && "font-mono text-[12px]",
                      )}
                    >
                      {c.label}
                    </BreadcrumbPage>
                  ) : c.to ? (
                    <BreadcrumbLink asChild>
                      <Link to={c.to} className="max-w-[220px] truncate text-[13px]">
                        {c.label}
                      </Link>
                    </BreadcrumbLink>
                  ) : (
                    <span className="text-muted-foreground text-[13px]">
                      {c.label}
                    </span>
                  )}
                </BreadcrumbItem>
              </div>
            );
          })}
        </BreadcrumbList>
      </Breadcrumb>

      {/* Back-compat slot. Pages should move CTAs into their own header
          via <PageShell actions={...}> — this slot will be removed when
          every page is migrated. Rendered small to discourage new use. */}
      {actions && (
        <div className="ml-3 flex items-center gap-2 [&_button]:h-7 [&_button]:text-[12px]">
          {actions}
        </div>
      )}

      <div className="ml-auto flex items-center gap-2">
        {env && envPalette && (
          <span
            className={cn(
              "hidden sm:inline-flex items-center h-6 px-2 rounded-full font-mono text-[10px] uppercase tracking-widest ring-1",
              envPalette,
            )}
            title={`Environment: ${env}`}
          >
            {env}
          </span>
        )}
        <button
          onClick={() => setCommandOpen(true)}
          className="flex items-center gap-1.5 px-2 py-1 bg-secondary/60 border border-border rounded-md text-muted-foreground text-[12px] cursor-pointer hover:bg-accent"
        >
          <Icon icon={icons.search} className="w-3.5 h-3.5" />
          <span>Search</span>
          <kbd className="bg-background px-1 py-0 rounded text-[10px] font-mono border border-border">⌘K</kbd>
        </button>
        <button
          onClick={() => setColorMode(resolvedMode === 'dark' ? 'light' : 'dark')}
          className="w-7 h-7 flex items-center justify-center rounded-md text-muted-foreground hover:bg-accent"
          aria-label="Toggle theme"
        >
          <Icon icon={resolvedMode === 'dark' ? icons.sun : icons.moon} className="w-4 h-4" />
        </button>
      </div>

      <CommandDialog open={commandOpen} onOpenChange={setCommandOpen}>
        <CommandInput placeholder="Search pages…" />
        <CommandList>
          <CommandEmpty>No results found.</CommandEmpty>
          {Array.from(new Set(Object.values(routes).map((r) => r.group))).map((group) => (
            <CommandGroup key={group} heading={group}>
              {Object.entries(routes)
                .filter(([, r]) => r.group === group)
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
