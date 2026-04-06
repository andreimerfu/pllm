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

type NavItem = {
  title: string;
  href: string;
  icon: string;
  permission: string | null;
};

type NavGroup = {
  title: string;
  items: NavItem[];
};

// Navigation items configuration with groups
const navigation: NavGroup[] = [
  {
    title: "Core",
    items: [
      {
        title: "Dashboard",
        href: "/dashboard",
        icon: icons.dashboard,
        permission: null,
      },
      {
        title: "Chat",
        href: "/chat",
        icon: icons.chat,
        permission: null,
      },
      {
        title: "Models",
        href: "/models",
        icon: icons.models,
        permission: null,
      },
      {
        title: "Routes",
        href: "/routes",
        icon: icons.routes,
        permission: null,
      },
    ],
  },
  {
    title: "Management",
    items: [
      {
        title: "API Keys",
        href: "/keys",
        icon: icons.keys,
        permission: null,
      },
      {
        title: "Teams",
        href: "/teams",
        icon: icons.teams,
        permission: "admin.teams.read",
      },
      {
        title: "Users",
        href: "/users",
        icon: icons.users,
        permission: "admin.users.read",
      },
    ],
  },
  {
    title: "Administration",
    items: [
      {
        title: "Budget",
        href: "/budget",
        icon: icons.budget,
        permission: "admin.budget.read",
      },
      {
        title: "Audit Logs",
        href: "/audit-logs",
        icon: icons.auditLogs,
        permission: "admin.audit.read",
      },
      {
        title: "Guardrails",
        href: "/guardrails",
        icon: icons.guardrails,
        permission: "admin.guardrails.read",
      },
      {
        title: "Settings",
        href: "/settings",
        icon: icons.settings,
        permission: "admin.settings.read",
      },
    ],
  },
];

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation();
  const { logout, user } = useAuth();
  const { config } = useConfig();
  const { resolvedMode, setColorMode } = useColorMode();

  // Extract user info from JWT
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
      .split(" ")
      .map((n: string) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2) || "U";

  const toggleTheme = () => {
    setColorMode(resolvedMode === 'dark' ? 'light' : 'dark');
  };

  // Filter navigation groups based on dex configuration
  const filteredNavigation = navigation.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      // Hide users section if dex is not enabled
      if (item.title === "Users" && config && !config.dex_enabled) {
        return false;
      }
      return true;
    }),
  })).filter((group) => group.items.length > 0); // Remove empty groups

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <div className="flex items-center gap-2.5 px-4 py-3 border-b border-sidebar-border">
          <img src="/robot.png" alt="pLLM" className="w-8 h-8 rounded-lg" />
          <div>
            <div className="text-sm font-bold text-sidebar-foreground">pLLM</div>
            <div className="text-[11px] text-muted-foreground">Gateway Console</div>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent>
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
                  {section.title}
                  <Icon icon={icons.chevronRight} className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                </CollapsibleTrigger>
              </SidebarGroupLabel>
              <CollapsibleContent>
                <SidebarMenu>
                  {section.items.map((item) => {
                    const isActive = location.pathname === item.href;

                    const NavigationItem = (
                      <SidebarMenuItem key={item.title}>
                        <SidebarMenuButton asChild isActive={isActive} tooltip={item.title}>
                          <Link to={item.href}>
                            <Icon icon={item.icon} className="h-4 w-4" />
                            <span>{item.title}</span>
                          </Link>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );

                    // If item has a permission requirement, wrap with CanAccess
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
