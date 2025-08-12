import { Link, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import { Icon } from "@iconify/react";
import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";

const navigation = [
  { name: "Dashboard", href: "/dashboard", icon: "lucide:layout-dashboard" },
  { name: "Models", href: "/models", icon: "lucide:brain" },
  { name: "Users", href: "/users", icon: "lucide:users" },
  { name: "Settings", href: "/settings", icon: "lucide:settings" },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const [isDark, setIsDark] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

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

  return (
    <div className="min-h-screen bg-background">
      {/* Mobile Menu Button */}
      <div className="lg:hidden fixed top-4 left-4 z-50">
        <Button
          variant="outline"
          size="icon"
          onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
          className="bg-background border-border"
        >
          {isMobileMenuOpen ? (
            <Icon icon="lucide:x" width="16" height="16" />
          ) : (
            <Icon icon="lucide:menu" width="16" height="16" />
          )}
        </Button>
      </div>

      {/* Mobile Overlay */}
      {isMobileMenuOpen && (
        <div
          className="lg:hidden fixed inset-0 z-40 bg-black/50"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div className={cn(
        "fixed inset-y-0 left-0 z-50 w-64 bg-card border-r transform transition-transform duration-200 ease-in-out",
        "lg:translate-x-0",
        isMobileMenuOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
      )}>
        <div className="flex flex-col h-full">
          {/* Logo */}
          <div className="flex items-center justify-between h-16 px-6 border-b">
            <div className="flex items-center space-x-3">
              <Icon icon="lucide:activity" width="32" height="32" className="text-primary" />
              <span className="text-lg lg:text-xl font-bold">pLLM Gateway</span>
            </div>
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-4 py-4 space-y-1">
            {navigation.map((item) => {
              const isActive = location.pathname === item.href;
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  onClick={() => setIsMobileMenuOpen(false)}
                  className={cn(
                    "flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors",
                    isActive
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  <Icon icon={item.icon} width="20" height="20" className="mr-3" />
                  {item.name}
                </Link>
              );
            })}
          </nav>

          {/* Footer */}
          <div className="p-4 border-t space-y-4">
            <div className="flex items-center justify-between">
              <Button
                variant="ghost"
                size="icon"
                onClick={toggleTheme}
                className="h-9 w-9"
              >
                {isDark ? (
                  <Icon icon="lucide:sun" width="16" height="16" />
                ) : (
                  <Icon icon="lucide:moon" width="16" height="16" />
                )}
              </Button>
              <a
                href="https://github.com/amerfu/pllm"
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                <Icon icon="lucide:github" width="20" height="20" />
              </a>
            </div>
            <div className="text-xs text-muted-foreground">
              <p>Version 1.0.0</p>
              <p>© 2025 pLLM Gateway</p>
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="lg:pl-64">
        <main className="p-4 pt-16 lg:p-8 lg:pt-8">{children}</main>
      </div>
    </div>
  );
}
