import * as React from "react";
import { Link, useLocation } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/OIDCAuthContext";
import { useConfig } from "@/contexts/ConfigContext";
import { CanAccess } from "@/components/CanAccess";
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { useColorMode } from '@/contexts/ColorModeContext';
import {
  listRegistryServers,
  listRegistryAgents,
  listRegistrySkills,
  listRegistryPrompts,
  listDeployments,
  listFederationPeers,
} from "@/lib/api";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarGroup,
  SidebarGroupLabel,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuGroup,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

type NavItem = {
  title: string;
  href: string;
  icon: string;
  permission: string | null;
  // feature gates the item behind a flag from /api/config. When the flag
  // is false the item renders in a dimmed/locked state and the click
  // routes to a "Governance is part of pLLM Enterprise" info panel
  // instead of the page itself. Leave undefined for always-on items.
  feature?: string;
  // kind deep-links into the Catalog (/registry) pre-filtered by registry
  // kind. Skills/Prompts have no standalone page — they're catalog facets.
  // Active-matching keys off the ?kind= query when this is set.
  kind?: string;
  // countKey selects which live count feeds the mono pill badge. Counts come
  // from real list endpoints (registry / deployments / federation) — never
  // fabricated; the badge is omitted while loading or on a 403.
  countKey?: "agents" | "servers" | "skills" | "prompts" | "deployments" | "federation";
};

type NavGroup = {
  title: string;
  items: NavItem[];
  // enterprise marks a whole group as EE-flavored so the label can carry
  // a subtle "EE" badge even when individual items are visible.
  enterprise?: boolean;
};

// Navigation grouped to lead with the registry — pLLM's new center of
// gravity as a control plane for agentic systems:
//   AI Registry   — catalog of agents/MCP/skills/prompts + deploy/federate
//   Gateway       — the OpenAI-compatible request surface
//   Governance    — MRM / compliance (EE, gated by features.mrm)
//   Access & Activity — identities, limits, audit
//   System        — notifications + operator settings
const navigation: NavGroup[] = [
  {
    title: "AI Registry",
    items: [
      { title: "Catalog", href: "/registry", icon: icons.registry, permission: null },
      { title: "Agents", href: "/agents", icon: icons.agents, permission: null, countKey: "agents" },
      { title: "MCP Servers", href: "/mcp-servers", icon: icons.mcp, permission: null, countKey: "servers" },
      { title: "Skills", href: "/registry?kind=skill", icon: icons.skill, permission: null, kind: "skill", countKey: "skills" },
      { title: "Prompts", href: "/registry?kind=prompt", icon: icons.prompt, permission: null, kind: "prompt", countKey: "prompts" },
      { title: "Deployments", href: "/deployments", icon: icons.rocket, permission: null, countKey: "deployments" },
      { title: "Federation", href: "/federation", icon: icons.federation, permission: null, feature: "federation", countKey: "federation" },
    ],
  },
  {
    title: "Gateway",
    items: [
      { title: "Models", href: "/models", icon: icons.models, permission: null },
      { title: "Routes", href: "/routes", icon: icons.routes, permission: null },
      { title: "Providers", href: "/providers", icon: icons.globe, permission: null },
      { title: "Guardrails", href: "/guardrails", icon: icons.guardrails, permission: "admin.guardrails.read" },
      { title: "Playground", href: "/playground", icon: icons.terminal, permission: null },
    ],
  },
  {
    title: "Governance",
    enterprise: true,
    items: [
      { title: "Use Cases", href: "/use-cases", icon: icons.useCase, permission: null, feature: "mrm" },
      { title: "Compliance", href: "/compliance", icon: icons.shieldCheck, permission: null, feature: "mrm" },
      { title: "Policies", href: "/policies", icon: icons.policies, permission: null, feature: "mrm" },
    ],
  },
  {
    title: "Access & Activity",
    items: [
      { title: "API Keys", href: "/keys", icon: icons.keys, permission: null },
      { title: "Teams", href: "/teams", icon: icons.teams, permission: "admin.teams.read" },
      { title: "Users", href: "/users", icon: icons.users, permission: "admin.users.read" },
      { title: "Budget", href: "/budget", icon: icons.budget, permission: "admin.budget.read" },
      { title: "Audit Logs", href: "/audit-logs", icon: icons.auditLogs, permission: "admin.audit.read" },
    ],
  },
  {
    title: "System",
    items: [
      { title: "Notifications", href: "/notifications", icon: icons.notifications, permission: null, feature: "mrm" },
      { title: "Settings", href: "/settings", icon: icons.settings, permission: "admin.settings.read" },
    ],
  },
];

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation();
  const { logout, user } = useAuth();
  const { config, features } = useConfig();
  const { resolvedMode, setColorMode } = useColorMode();

  const userEmail =
    user?.profile?.email ||
    user?.profile?.preferred_username ||
    "user@pllm.local";
  const userName =
    user?.profile?.name ||
    user?.profile?.preferred_username ||
    userEmail.split("@")[0];
  const userInitials =
    userName
      .split(/\s+/)
      .map((n: string) => n.match(/[A-Za-z]/)?.[0] ?? "")
      .join("")
      .toUpperCase()
      .slice(0, 2) || "U";

  // Role badge — only rendered when a real role claim is present (never
  // fabricated). Looks at common OIDC shapes: `role`, `roles[]`, `groups[]`.
  const profile = user?.profile as Record<string, unknown> | undefined;
  const userRole = ((): string | null => {
    const r =
      (typeof profile?.role === "string" && profile.role) ||
      (Array.isArray(profile?.roles) && (profile?.roles as string[])[0]) ||
      (Array.isArray(profile?.groups) && (profile?.groups as string[])[0]) ||
      null;
    return r ? String(r).toUpperCase() : null;
  })();

  const toggleTheme = () => {
    setColorMode(resolvedMode === 'dark' ? 'light' : 'dark');
  };

  // Live nav counts — real catalog / deployment / federation totals. Cached
  // and shared across the session; a 403 (non-admin) just leaves the badge
  // off. Never fabricated.
  const registryCounts = useQuery({
    queryKey: ["sidebar-registry-counts"],
    queryFn: async () => {
      const [s, a, sk, p] = await Promise.all([
        listRegistryServers({ latest: true }),
        listRegistryAgents({ latest: true }),
        listRegistrySkills({ latest: true }),
        listRegistryPrompts({ latest: true }),
      ]);
      return {
        servers: s.total ?? s.servers?.length ?? 0,
        agents: a.total ?? a.agents?.length ?? 0,
        skills: sk.total ?? sk.skills?.length ?? 0,
        prompts: p.total ?? p.prompts?.length ?? 0,
      };
    },
    staleTime: 60_000,
    retry: false,
  });
  const deploymentsCount = useQuery({
    queryKey: ["sidebar-deployments-count"],
    queryFn: async () => (await listDeployments()).deployments?.length ?? 0,
    staleTime: 30_000,
    retry: false,
  });
  const peersCount = useQuery({
    queryKey: ["sidebar-peers-count"],
    queryFn: async () => (await listFederationPeers()).peers?.length ?? 0,
    staleTime: 60_000,
    retry: false,
  });

  const liveCounts: Record<NonNullable<NavItem["countKey"]>, number | undefined> = {
    agents: registryCounts.data?.agents,
    servers: registryCounts.data?.servers,
    skills: registryCounts.data?.skills,
    prompts: registryCounts.data?.prompts,
    deployments: deploymentsCount.data,
    federation: peersCount.data,
  };

  const dashboardActive = location.pathname === "/dashboard";

  // Skills/Prompts are catalog facets (/registry?kind=…). Match active state
  // off the query so they highlight independently of the bare Catalog entry.
  const kindParam = new URLSearchParams(location.search).get("kind");
  const itemActive = (item: NavItem): boolean => {
    if (item.href.startsWith("/registry")) {
      if (item.kind) {
        return location.pathname === "/registry" && kindParam === item.kind;
      }
      // Catalog itself — active on /registry unless a facet deep-link owns it.
      return (
        location.pathname === "/registry" &&
        kindParam !== "skill" &&
        kindParam !== "prompt"
      );
    }
    return location.pathname === item.href;
  };

  // Active-row style: soft accent fill + accent text (the "pLLM Console"
  // pill). Saturated color stays reserved for the CTA on each page.
  const activeRowClass = cn(
    "rounded-[9px] gap-[11px] px-2.5 text-[13.5px] font-medium tracking-[-0.01em]",
    "text-muted-foreground hover:text-foreground",
    "data-[active=true]:bg-[var(--accent-soft)] data-[active=true]:text-primary data-[active=true]:font-semibold",
    "data-[active=true]:[&_svg]:text-primary",
    "data-[active=true]:hover:bg-[var(--accent-soft)] data-[active=true]:hover:text-primary",
  );

  // Filter navigation groups based on dex configuration. Feature-gated
  // items are NOT removed — we dim them on-screen so users know the
  // capability exists (and where to upgrade). Group visibility only
  // changes if the whole group ends up empty after permission filtering.
  const filteredNavigation = navigation.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      if (item.title === "Users" && config && !config.dex_enabled) return false;
      return true;
    }),
  })).filter((group) => group.items.length > 0);

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <div className="flex items-center gap-2.5 px-2 group-data-[collapsible=icon]:px-0 group-data-[collapsible=icon]:justify-center py-3 border-b border-sidebar-border overflow-hidden">
          <span className="flex h-[34px] w-[34px] flex-shrink-0 items-center justify-center rounded-[9px] bg-gradient-to-br from-emerald-400 to-emerald-700 text-white shadow-[0_3px_10px_rgba(14,159,110,0.32)]">
            <Icon icon={icons.layers} className="h-[18px] w-[18px]" />
          </span>
          <div className="min-w-0 group-data-[collapsible=icon]:hidden">
            <div className="text-[15px] font-bold leading-tight tracking-[-0.01em] text-sidebar-foreground">pLLM</div>
            <div className="text-[11px] text-text-dim">AI Registry &amp; Gateway</div>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent className="gap-0 pb-3.5">
        {/* Pinned top-level — Dashboard is the cross-cutting overview that
            doesn't belong to a single narrative group. */}
        <SidebarGroup className="px-3.5 pb-0 pt-2">
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                asChild
                isActive={dashboardActive}
                tooltip="Dashboard"
                className={activeRowClass}
              >
                <Link to="/dashboard">
                  <Icon icon={icons.dashboard} className="h-[18px] w-[18px]" />
                  <span>Dashboard</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>

        {filteredNavigation.map((section) => (
          <SidebarGroup key={section.title} className="px-3.5 py-0">
            <SidebarGroupLabel className="h-auto px-2.5 pb-[7px] pt-[15px] text-[10.5px] font-semibold uppercase tracking-[0.08em] text-text-dim">
              {section.title}
            </SidebarGroupLabel>
            <SidebarMenu>
              {section.items.map((item) => {
                const isActive = itemActive(item);
                const locked = Boolean(item.feature) && !features?.[item.feature as string];
                const count = item.countKey ? liveCounts[item.countKey] : undefined;

                const NavigationItem = (
                  <SidebarMenuItem key={item.title}>
                    <SidebarMenuButton
                      asChild
                      isActive={isActive}
                      tooltip={locked ? `${item.title} — pLLM Enterprise` : item.title}
                      className={cn(activeRowClass, locked && "opacity-60")}
                    >
                      <Link
                        to={locked ? "/unauthorized?feature=mrm" : item.href}
                        aria-disabled={locked}
                      >
                        <Icon icon={item.icon} className="h-[18px] w-[18px]" />
                        <span className="flex-1">{item.title}</span>
                        {locked ? (
                          <Icon
                            icon={icons.lock}
                            className="h-3 w-3 text-muted-foreground"
                            aria-hidden
                          />
                        ) : (
                          count != null && (
                            <span className="ml-auto rounded-full bg-surface-2 px-[7px] py-px font-mono text-[10.5px] font-semibold tabular-nums text-text-dim group-data-[collapsible=icon]:hidden">
                              {count}
                            </span>
                          )
                        )}
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );

                if (item.permission) {
                  return (
                    <CanAccess key={item.title} permission={item.permission}>
                      {NavigationItem}
                    </CanAccess>
                  );
                }

                return NavigationItem;
              })}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  size="lg"
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                >
                  <Avatar className="h-8 w-8 rounded-lg">
                    <AvatarImage src={user?.profile?.picture} alt={userName} />
                    <AvatarFallback className="rounded-lg">
                      {userInitials}
                    </AvatarFallback>
                  </Avatar>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">{userName}</span>
                    <span className="truncate text-xs">{userEmail}</span>
                  </div>
                  {userRole && (
                    <span className="ml-auto rounded-md bg-[var(--accent-soft)] px-1.5 py-0.5 font-mono text-[9.5px] font-bold tracking-[0.05em] text-primary group-data-[collapsible=icon]:hidden">
                      {userRole}
                    </span>
                  )}
                  <Icon icon={icons.chevronsUpDown} className="size-4 shrink-0" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                className="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg"
                side="right"
                align="end"
                sideOffset={4}
              >
                <DropdownMenuLabel className="p-0 font-normal">
                  <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                    <Avatar className="h-8 w-8 rounded-lg">
                      <AvatarImage src={user?.profile?.picture} alt={userName} />
                      <AvatarFallback className="rounded-lg">
                        {userInitials}
                      </AvatarFallback>
                    </Avatar>
                    <div className="grid flex-1 text-left text-sm leading-tight">
                      <span className="truncate font-medium">{userName}</span>
                      <span className="truncate text-xs">{userEmail}</span>
                    </div>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuItem>
                    <Icon icon={icons.user} />
                    Profile
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={toggleTheme}>
                    <Icon icon={resolvedMode === 'dark' ? icons.sun : icons.moon} />
                    {resolvedMode === 'dark' ? "Light Mode" : "Dark Mode"}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuItem asChild>
                    <a href="/docs" target="_blank" rel="noopener noreferrer">
                      <Icon icon={icons.book} />
                      Documentation
                    </a>
                  </DropdownMenuItem>
                  <DropdownMenuItem asChild>
                    <a
                      href="https://github.com/andreimerfu/pllm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <Icon icon={icons.github} />
                      GitHub Repository
                    </a>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={logout} className="text-destructive">
                  <Icon icon={icons.logout} />
                  Logout
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
