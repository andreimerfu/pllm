import * as React from "react";
import { Link, useLocation } from "react-router-dom";
import { useAuth } from "@/contexts/OIDCAuthContext";
import { useConfig } from "@/contexts/ConfigContext";
import { CanAccess } from "@/components/CanAccess";
import { useState, useEffect } from "react";
import {
  Home,
  MessageSquare,
  Brain,
  Users2,
  Key,
  Users,
  Wallet,
  Settings,
  User,
  Sun,
  Moon,
  BookOpen,
  Github,
  LogOut,
  Activity,
  ChevronUp,
  FileText,
  Shield,
} from "lucide-react";

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

  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";

// Navigation items configuration with submenus
const navigation = [
  {
    title: "Core",
    items: [
      {
        title: "Dashboard",
        href: "/dashboard",
        icon: Home,
        permission: null,
      },
      {
        title: "Chat",
        href: "/chat",
        icon: MessageSquare,
        permission: null,
      },
      {
        title: "Models",
        href: "/models",
        icon: Brain,
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
        icon: Key,
        permission: null,
      },
      {
        title: "Teams",
        href: "/teams",
        icon: Users2,
        permission: "admin.teams.read",
      },
      {
        title: "Users",
        href: "/users",
        icon: Users,
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
        icon: Wallet,
        permission: "admin.budget.read",
      },
      {
        title: "Audit Logs",
        href: "/audit-logs",
        icon: FileText,
        permission: "admin.audit.read",
      },
      {
        title: "Guardrails",
        href: "/guardrails",
        icon: Shield,
        permission: "admin.guardrails.read",
      },
      {
        title: "Settings",
        href: "/settings",
        icon: Settings,
        permission: "admin.settings.read",
      },
    ],
  },
];

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation();
  const { logout, user } = useAuth();
  const { config } = useConfig();
  const [isDark, setIsDark] = useState(false);

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
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2) || "U";

  useEffect(() => {
    // Check for saved theme preference or default to light
    const savedTheme = localStorage.getItem("theme");
    const prefersDark = window.matchMedia(
      "(prefers-color-scheme: dark)",
    ).matches;
    const shouldBeDark = savedTheme === "dark" || (!savedTheme && prefersDark);

    setIsDark(shouldBeDark);
    if (shouldBeDark) {
      document.documentElement.classList.add("dark");
    }
  }, []);

  const toggleTheme = () => {
    const newTheme = !isDark;
    setIsDark(newTheme);

    if (newTheme) {
      document.documentElement.classList.add("dark");
      localStorage.setItem("theme", "dark");
    } else {
      document.documentElement.classList.remove("dark");
      localStorage.setItem("theme", "light");
    }
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
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                <Activity className="size-4" />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">pLLM</span>
                <span className="truncate text-xs opacity-70">
                  AI Model Router
                </span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarMenu>
            {filteredNavigation.map((section) => (
              <SidebarMenuItem key={section.title}>
                <SidebarMenuButton asChild>
                  <div className="font-medium cursor-default">
                    {section.title}
                  </div>
                </SidebarMenuButton>
                {section.items?.length ? (
                  <SidebarMenuSub>
                    {section.items.map((item) => {
                      const isActive = location.pathname === item.href;
                      
                      const NavigationSubItem = (
                        <SidebarMenuSubItem key={item.title}>
                          <SidebarMenuSubButton asChild isActive={isActive}>
                            <Link to={item.href}>
                              <item.icon />
                              <span>{item.title}</span>
                            </Link>
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      );

                      // If item has a permission requirement, wrap with CanAccess
                      if (item.permission) {
                        return (
                          <CanAccess key={item.title} permission={item.permission}>
                            {NavigationSubItem}
                          </CanAccess>
                        );
                      }

                      return NavigationSubItem;
                    })}
                  </SidebarMenuSub>
                ) : null}
              </SidebarMenuItem>
            ))}
          </SidebarMenu>
        </SidebarGroup>
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
                  <Avatar className="h-8 w-8">
                    <AvatarImage src={user?.profile?.picture} />
                    <AvatarFallback className="rounded-lg">
                      {userInitials}
                    </AvatarFallback>
                  </Avatar>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-semibold">{userName}</span>
                    <span className="truncate text-xs">{userEmail}</span>
                  </div>
                  <ChevronUp className="ml-auto size-4" />
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
                      <AvatarImage src={user?.profile?.picture} />
                      <AvatarFallback className="rounded-lg">
                        {userInitials}
                      </AvatarFallback>
                    </Avatar>
                    <div className="grid flex-1 text-left text-sm leading-tight">
                      <span className="truncate font-semibold">{userName}</span>
                      <span className="truncate text-xs">{userEmail}</span>
                    </div>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem>
                  <User className="mr-2 h-4 w-4" />
                  Profile
                </DropdownMenuItem>
                <DropdownMenuItem onClick={toggleTheme}>
                  {isDark ? (
                    <Sun className="mr-2 h-4 w-4" />
                  ) : (
                    <Moon className="mr-2 h-4 w-4" />
                  )}
                  {isDark ? "Light Mode" : "Dark Mode"}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild>
                  <a href="/docs" target="_blank" rel="noopener noreferrer">
                    <BookOpen className="mr-2 h-4 w-4" />
                    Documentation
                  </a>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <a
                    href="https://github.com/andreimerfu/pllm"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <Github className="mr-2 h-4 w-4" />
                    GitHub Repository
                  </a>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={logout} className="text-destructive">
                  <LogOut className="mr-2 h-4 w-4" />
                  Logout
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>

        <SidebarMenu>
          <SidebarMenuItem>
            <div className="flex flex-col items-center gap-2 px-2 py-3 text-xs text-muted-foreground">
              <div className="text-center">
                <p className="font-medium group-data-[collapsible=icon]:hidden">
                  Version 1.0.0
                </p>
                <p className="group-data-[collapsible=icon]:hidden">
                  © 2025 pLLM
                </p>
              </div>
            </div>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
