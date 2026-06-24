import * as React from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "@/contexts/OIDCAuthContext";
import { useConfig } from "@/contexts/ConfigContext";
import { CanAccess } from "@/components/CanAccess";
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { useColorMode } from '@/contexts/ColorModeContext';

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
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
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
};

type NavGroup = {
  title: string;
  items: NavItem[];
  // enterprise marks a whole group as EE-flavored so the label can carry
  // a subtle "EE" badge even when individual items are visible.
  enterprise?: boolean;
};

// Navigation grouped by narrative rather than admin-rights:
//   LLM Gateway   — OpenAI-compatible request surface
//   Agents & MCP  — the MCP/registry/deployments layer
//   Governance    — MRM / compliance (EE, gated by features.mrm)
//   Access & Activity — identities, limits, audit
//   System        — operator-only
const navigation: NavGroup[] = [
  {
    title: "LLM Gateway",
    items: [
      { title: "Models", href: "/models", icon: icons.models, permission: null },
      { title: "Routes", href: "/routes", icon: icons.routes, permission: null },
      { title: "Providers", href: "/providers", icon: icons.globe, permission: null },
      { title: "Guardrails", href: "/guardrails", icon: icons.guardrails, permission: "admin.guardrails.read" },
    ],
  },
  {
    title: "Agents & MCP",
    items: [
      { title: "Registry", href: "/registry", icon: icons.registry, permission: null },
      { title: "MCP Servers", href: "/mcp-servers", icon: icons.mcp, permission: null },
      { title: "Deployments", href: "/deployments", icon: icons.deployment, permission: null },
      { title: "Agents", href: "/agents", icon: icons.agents, permission: null },
      { title: "Federation", href: "/federation", icon: icons.federation, permission: null, feature: "federation" },
    ],
  },
  {
    title: "Governance",
    enterprise: true,
    items: [
      { title: "Use Cases", href: "/use-cases", icon: icons.useCase, permission: null, feature: "mrm" },
      { title: "Compliance", href: "/compliance", icon: icons.shield, permission: null, feature: "mrm" },
      { title: "Policies", href: "/policies", icon: icons.policies, permission: null, feature: "mrm" },
      { title: "Notifications", href: "/notifications", icon: icons.notifications, permission: null, feature: "mrm" },
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

  const toggleTheme = () => {
    setColorMode(resolvedMode === 'dark' ? 'light' : 'dark');
  };

  const dashboardActive = location.pathname === "/dashboard";
  const playgroundActive =
    location.pathname === "/playground" || location.pathname === "/chat";

  // Active-row style: muted bg tint + 2px primary left bar, instead of
  // the default solid teal background. Keeps the sidebar calm and
  // reserves saturated color for the CTA on each page.
  const activeRowClass = cn(
    "relative",
    "data-[active=true]:bg-primary/5 data-[active=true]:text-foreground data-[active=true]:font-medium",
    "data-[active=true]:before:content-[''] data-[active=true]:before:absolute",
    "data-[active=true]:before:left-0 data-[active=true]:before:inset-y-1",
    "data-[active=true]:before:w-[2px] data-[active=true]:before:rounded-full",
    "data-[active=true]:before:bg-primary",
    "data-[active=true]:[&_svg]:text-primary",
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
          <img src={`${import.meta.env.BASE_URL}robot.png`} alt="pLLM" className="w-8 h-8 rounded-lg flex-shrink-0" />
          <div className="group-data-[collapsible=icon]:hidden">
            <div className="text-sm font-bold text-sidebar-foreground">pLLM</div>
            <div className="text-[11px] text-muted-foreground">Gateway Console</div>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent>
        {/* Pinned top-level — cross-cutting entries that don't belong to a
            narrative group. Dashboard = overview, Playground = try things. */}
        <SidebarGroup>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                asChild
                isActive={dashboardActive}
                tooltip="Dashboard"
                className={activeRowClass}
              >
                <Link to="/dashboard">
                  <Icon icon={icons.dashboard} className="h-4 w-4" />
                  <span>Dashboard</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton
                asChild
                isActive={playgroundActive}
                tooltip="Playground"
                className={activeRowClass}
              >
                <Link to="/playground">
                  <Icon icon={icons.chat} className="h-4 w-4" />
                  <span>Playground</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>

        {filteredNavigation.map((section) => (
          <Collapsible
            key={section.title}
            asChild
            defaultOpen={true}
            className="group/collapsible"
          >
            <SidebarGroup>
              <SidebarGroupLabel asChild>
                <CollapsibleTrigger>
                  <span className="flex-1 text-left">{section.title}</span>
                  {section.enterprise && (
                    <span
                      className="mr-1 h-1.5 w-1.5 rounded-full bg-amber-500/70"
                      title="pLLM Enterprise"
                      aria-label="Enterprise feature"
                    />
                  )}
                  <Icon icon={icons.chevronRight} className="transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                </CollapsibleTrigger>
              </SidebarGroupLabel>
              <CollapsibleContent>
                <SidebarMenu>
                  {section.items.map((item) => {
                    const isActive = location.pathname === item.href;
                    const locked = Boolean(item.feature) && !features?.[item.feature as string];

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
                            <Icon icon={item.icon} className="h-4 w-4" />
                            <span className="flex-1">{item.title}</span>
                            {locked && (
                              <Icon
                                icon={icons.lock}
                                className="h-3 w-3 text-muted-foreground"
                                aria-hidden
                              />
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
              </CollapsibleContent>
            </SidebarGroup>
          </Collapsible>
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
                  <Icon icon={icons.chevronsUpDown} className="ml-auto size-4" />
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
