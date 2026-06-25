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
          <main className="flex flex-1 flex-col bg-background">
            <div className="mx-auto flex min-h-0 w-full max-w-[1340px] flex-1 flex-col px-6 pb-16 pt-6 lg:px-[30px] lg:pt-[26px]">
              {children}
            </div>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </PageChromeProvider>
  );
}
