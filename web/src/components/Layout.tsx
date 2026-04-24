import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { AppNavbar } from "@/components/AppNavbar";
import { AppSidebar } from "@/components/app-sidebar";
import { PageChromeProvider } from "@/components/common/page-chrome";

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <PageChromeProvider>
      <SidebarProvider defaultOpen={true}>
        <AppSidebar />
        <SidebarInset>
          <AppNavbar />
          <main className="flex flex-1 flex-col gap-4 bg-background px-6 pt-6 pb-0 lg:px-8 lg:pt-8">
            <div className="w-full flex-1 flex flex-col min-h-0">
              {children}
            </div>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </PageChromeProvider>
  );
}
