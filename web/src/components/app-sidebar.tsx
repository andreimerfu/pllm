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
  ChevronsUpDown,
  FileText,
  Shield,
  ChevronRight,
  GitBranch,
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

// Navigation items configuration with groups
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
      {
        title: "Routes",
        href: "/routes",
        icon: GitBranch,
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
                <span className="truncate text-xs">AI Model Router</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
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
                  <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
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
                            <item.icon />
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
                  <ChevronsUpDown className="ml-auto size-4" />
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
                    <User />
                    Profile
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={toggleTheme}>
                    {isDark ? <Sun /> : <Moon />}
                    {isDark ? "Light Mode" : "Dark Mode"}
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuItem asChild>
                    <a href="/docs" target="_blank" rel="noopener noreferrer">
                      <BookOpen />
                      Documentation
                    </a>
                  </DropdownMenuItem>
                  <DropdownMenuItem asChild>
                    <a
                      href="https://github.com/andreimerfu/pllm"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <Github />
                      GitHub Repository
                    </a>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={logout} className="text-destructive">
                  <LogOut />
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
