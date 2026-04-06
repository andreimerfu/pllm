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

const navCommands = [
  { label: "Dashboard", path: "/dashboard", icon: icons.dashboard },
  { label: "Chat", path: "/chat", icon: icons.chat },
  { label: "Models", path: "/models", icon: icons.models },
  { label: "Routes", path: "/routes", icon: icons.routes },
  { label: "API Keys", path: "/keys", icon: icons.keys },
  { label: "Teams", path: "/teams", icon: icons.teams },
  { label: "Users", path: "/users", icon: icons.users },
  { label: "Budget", path: "/budget", icon: icons.budget },
  { label: "Audit Logs", path: "/audit-logs", icon: icons.auditLogs },
  { label: "Guardrails", path: "/guardrails", icon: icons.guardrails },
  { label: "Settings", path: "/settings", icon: icons.settings },
];

const routeNames: Record<string, string> = {
  "/dashboard": "Dashboard",
  "/chat": "Chat",
  "/models": "Models",
  "/users": "Users",
  "/teams": "Teams",
  "/keys": "API Keys",
  "/budget": "Budget",
  "/audit-logs": "Audit Logs",
  "/guardrails": "Guardrails",
  "/settings": "Settings",
};

// Function to get display name for dynamic segments
const getDynamicSegmentLabel = (segment: string, parentPath: string): string => {
  // For model IDs, decode and shorten if needed
  if (parentPath.includes('/models')) {
    try {
      const decoded = decodeURIComponent(segment);
      // Shorten long model names for breadcrumb
      return decoded.length > 40 ? decoded.substring(0, 40) + '...' : decoded;
    } catch {
      return segment;
    }
  }

  // For guardrail config
  if (parentPath.includes('/guardrails/config')) {
    return segment === 'new' ? 'New Guardrail' : `Config ${segment}`;
  }

  // Default: capitalize and decode
  try {
    const decoded = decodeURIComponent(segment);
    return decoded.charAt(0).toUpperCase() + decoded.slice(1);
  } catch {
    return segment.charAt(0).toUpperCase() + segment.slice(1);
  }
};

export function AppNavbar() {
  const location = useLocation();
  const navigate = useNavigate();
  const { resolvedMode, setColorMode } = useColorMode();
  const [commandOpen, setCommandOpen] = useState(false);
  const pathSegments = location.pathname.split("/").filter(Boolean);

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

  // Generate breadcrumb items
  const breadcrumbItems: Array<{ path: string; label: string }> = [];
  let currentPath = "";

  pathSegments.forEach((segment) => {
    currentPath += `/${segment}`;

    // Check if this is a known route
    const knownRoute = routeNames[currentPath];

    if (knownRoute) {
      breadcrumbItems.push({ path: currentPath, label: knownRoute });
    } else {
      // This is a dynamic segment
      const parentPath = breadcrumbItems.length > 0 ? breadcrumbItems[breadcrumbItems.length - 1].path : '';
      const label = getDynamicSegmentLabel(segment, parentPath);
      breadcrumbItems.push({ path: currentPath, label });
    }
  });

  return (
    <header className="sticky top-0 z-10 flex h-[52px] shrink-0 items-center gap-2 border-b border-border bg-background px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="h-4" />

      <Breadcrumb>
        <BreadcrumbList>
          {breadcrumbItems.map((item, index) => (
            <div key={item.path} className="flex items-center gap-1.5">
              {index > 0 && <BreadcrumbSeparator />}
              <BreadcrumbItem>
                {index === breadcrumbItems.length - 1 ? (
                  <BreadcrumbPage className="max-w-[200px] truncate">{item.label}</BreadcrumbPage>
                ) : (
                  <BreadcrumbLink asChild>
                    <Link to={item.path} className="max-w-[200px] truncate">{item.label}</Link>
                  </BreadcrumbLink>
                )}
              </BreadcrumbItem>
            </div>
          ))}
        </BreadcrumbList>
      </Breadcrumb>

      <div className="ml-auto flex items-center gap-3">
        {/* Search trigger */}
        <button
          onClick={() => setCommandOpen(true)}
          className="flex items-center gap-1.5 px-2.5 py-1.5 bg-secondary border border-border rounded-[6px] text-muted-foreground text-xs cursor-pointer hover:bg-accent"
        >
          <Icon icon={icons.search} className="w-3.5 h-3.5" />
          <span>Search...</span>
          <kbd className="bg-background px-1.5 py-0.5 rounded text-[10px] font-mono border border-border">⌘K</kbd>
        </button>
        {/* Theme toggle */}
        <button
          onClick={() => setColorMode(resolvedMode === 'dark' ? 'light' : 'dark')}
          className="w-8 h-8 flex items-center justify-center rounded-[6px] text-muted-foreground hover:bg-accent"
        >
          <Icon icon={resolvedMode === 'dark' ? icons.sun : icons.moon} className="w-4 h-4" />
        </button>
        {/* Version badge */}
        <span className="text-[11px] text-muted-foreground bg-secondary border border-border px-2 py-0.5 rounded-full font-mono">v2.0.0</span>
      </div>
      <CommandDialog open={commandOpen} onOpenChange={setCommandOpen}>
        <CommandInput placeholder="Search pages..." />
        <CommandList>
          <CommandEmpty>No results found.</CommandEmpty>
          <CommandGroup heading="Pages">
            {navCommands.map((cmd) => (
              <CommandItem
                key={cmd.path}
                onSelect={() => {
                  navigate(cmd.path);
                  setCommandOpen(false);
                }}
              >
                <Icon icon={cmd.icon} className="mr-2 h-4 w-4" />
                {cmd.label}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </header>
  );
}
