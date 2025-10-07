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
import { Badge } from "@/components/ui/badge";

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
    <header className="sticky top-0 z-10 flex h-16 shrink-0 items-center gap-2 border-b bg-background px-4">
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

      <div className="ml-auto flex items-center gap-2">
        <Badge variant="outline" className="text-xs">
          v1.0.0
        </Badge>
      </div>
    </header>
  );
}
