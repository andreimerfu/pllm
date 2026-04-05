import { useLocation, Link } from "react-router-dom";
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
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { useColorMode } from '@/contexts/ColorModeContext';

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
  const { resolvedMode, setColorMode } = useColorMode();
  const pathSegments = location.pathname.split("/").filter(Boolean);

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
        <div className="flex items-center gap-1.5 px-2.5 py-1.5 bg-secondary border border-border rounded-[6px] text-muted-foreground text-xs cursor-pointer hover:bg-accent">
          <Icon icon={icons.search} className="w-3.5 h-3.5" />
          <span>Search...</span>
          <kbd className="bg-background px-1.5 py-0.5 rounded text-[10px] font-mono border border-border">⌘K</kbd>
        </div>
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
    </header>
  );
}
