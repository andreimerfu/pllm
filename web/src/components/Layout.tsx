import { SidebarProvider, SidebarInset } from "@/components/ui/sidebar";
import { AppNavbar } from "@/components/AppNavbar";
import { AppSidebar } from "@/components/app-sidebar";

export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <SidebarProvider defaultOpen={true}>
      <AppSidebar />
      <SidebarInset>
        <AppNavbar />
        <main className="flex flex-1 flex-col gap-4 bg-background p-6 lg:p-8">
          <div className="w-full flex-1 flex flex-col min-h-0">
            {children}
          </div>
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
