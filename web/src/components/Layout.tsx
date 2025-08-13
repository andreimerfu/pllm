import { Link, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import { Icon } from "@iconify/react";
import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";

const navigation = [
  { name: "Dashboard", href: "/dashboard", icon: "lucide:layout-dashboard" },
  { name: "Chat", href: "/chat", icon: "lucide:messages-square" },
  { name: "Models", href: "/models", icon: "lucide:brain" },
  { name: "Teams", href: "/teams", icon: "lucide:users-2" },
  { name: "API Keys", href: "/keys", icon: "lucide:key" },
  { name: "Users", href: "/users", icon: "lucide:users" },
  { name: "Settings", href: "/settings", icon: "lucide:settings" },
];

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { logout } = useAuth();
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
    <div className="min-h-screen bg-background transition-theme">
      {/* Mobile Menu Button */}
      <div className="lg:hidden fixed top-4 left-4 z-50">
        <Button
          variant="outline"
          size="icon"
          onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
          className="glass border-border shadow-lg hover:shadow-xl transition-all duration-200"
        >
          <div className="relative">
            <Icon 
              icon={isMobileMenuOpen ? "lucide:x" : "lucide:menu"} 
              width="16" 
              height="16" 
              className="transition-transform duration-200" 
            />
          </div>
        </Button>
      </div>

      {/* Mobile Overlay */}
      {isMobileMenuOpen && (
        <div
          className="lg:hidden fixed inset-0 z-40 bg-black/50 backdrop-blur-sm animate-in fade-in-0 duration-200"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div className={cn(
        "fixed inset-y-0 left-0 z-50 w-64 bg-card/95 backdrop-blur-xl border-r border-border/50 shadow-2xl",
        "transform transition-all duration-300 ease-out",
        "lg:translate-x-0 lg:shadow-none lg:bg-card lg:backdrop-blur-none",
        isMobileMenuOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
      )}>
        <div className="flex flex-col h-full">
          {/* Logo */}
          <div className="flex items-center justify-between h-16 px-6 border-b border-border/50">
            <div className="flex items-center space-x-3">
              <div className="relative">
                <div className="absolute inset-0 rounded-lg bg-primary/20 blur animate-pulse" />
                <div className="relative p-2 rounded-lg bg-primary/10 border border-primary/20">
                  <Icon icon="lucide:activity" width="24" height="24" className="text-primary" />
                </div>
              </div>
              <div>
                <span className="text-lg lg:text-xl font-bold bg-gradient-to-r from-primary to-primary/70 bg-clip-text text-transparent">
                  pLLM Gateway
                </span>
                <p className="text-xs text-muted-foreground mt-0.5">AI Model Router</p>
              </div>
            </div>
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-4 py-6 space-y-2">
            {navigation.map((item) => {
              const isActive = location.pathname === item.href;
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  onClick={() => setIsMobileMenuOpen(false)}
                  className={cn(
                    "group flex items-center px-3 py-3 text-sm font-medium rounded-xl transition-all duration-200 relative overflow-hidden",
                    isActive
                      ? "bg-primary text-primary-foreground shadow-lg shadow-primary/25"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground hover:shadow-md",
                  )}
                >
                  {isActive && (
                    <div className="absolute inset-0 bg-gradient-to-r from-primary to-primary/80 rounded-xl" />
                  )}
                  <div className={cn(
                    "relative flex items-center justify-center w-8 h-8 rounded-lg mr-3 transition-all duration-200",
                    isActive 
                      ? "bg-white/20 text-white" 
                      : "bg-muted/50 group-hover:bg-muted"
                  )}>
                    <Icon icon={item.icon} width="18" height="18" />
                  </div>
                  <span className="relative font-semibold">{item.name}</span>
                  {isActive && (
                    <div className="absolute right-3 w-1 h-1 rounded-full bg-white/60" />
                  )}
                </Link>
              );
            })}
          </nav>

          {/* Footer */}
          <div className="p-4 border-t border-border/50 space-y-4">
            <div className="flex items-center justify-between">
              <Button
                variant="ghost"
                size="icon"
                onClick={toggleTheme}
                className="h-10 w-10 rounded-xl hover:bg-muted transition-all duration-200 group"
              >
                <div className="relative">
                  <Icon 
                    icon={isDark ? "lucide:sun" : "lucide:moon"} 
                    width="18" 
                    height="18" 
                    className="transition-transform duration-200"
                  />
                </div>
              </Button>
              <div className="flex items-center space-x-2">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={logout}
                  className="h-10 w-10 rounded-xl hover:bg-muted transition-all duration-200 group"
                  title="Logout"
                >
                  <Icon icon="lucide:log-out" width="18" height="18" className="text-muted-foreground group-hover:text-foreground transition-colors duration-200" />
                </Button>
                <a
                  href="/docs"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted transition-all duration-200 group"
                  title="Documentation"
                >
                  <Icon icon="lucide:book-open" width="18" height="18" className="transition-transform duration-200" />
                </a>
                <a
                  href="https://github.com/amerfu/pllm"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted transition-all duration-200 group"
                  title="GitHub Repository"
                >
                  <Icon icon="lucide:github" width="18" height="18" className="transition-transform duration-200" />
                </a>
              </div>
            </div>
            <div className="text-xs text-muted-foreground space-y-1">
              <div className="flex items-center justify-between">
                <span className="font-medium">Version 1.0.0</span>
                <div className="flex items-center space-x-1">
                  <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                  <span className="text-green-600 dark:text-green-400 font-medium">Online</span>
                </div>
              </div>
              <p className="text-center pt-1 border-t border-border/30">© 2025 pLLM Gateway</p>
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="lg:pl-64 transition-all duration-300">
        <main className="p-4 pt-16 lg:p-8 lg:pt-8 min-h-screen">
          <div className="mx-auto max-w-7xl">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
