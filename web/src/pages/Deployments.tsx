import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import {
  listDeployments,
  refreshDeploymentStatus,
  deleteDeployment,
} from "@/lib/api";
import type { Deployment, DeploymentStatus } from "@/types/api";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { useToast } from "@/hooks/use-toast";

const statusColors: Record<DeploymentStatus, string> = {
  running: "bg-green-500/15 text-green-700 dark:text-green-400 border-green-500/30",
  deploying: "bg-blue-500/15 text-blue-700 dark:text-blue-400 border-blue-500/30",
  pending: "bg-muted text-muted-foreground",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400 border-red-500/30",
  terminating: "bg-amber-500/15 text-amber-700 dark:text-amber-400 border-amber-500/30",
  stopped: "bg-muted text-muted-foreground",
};

export default function Deployments() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState<Deployment | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["deployments"],
    queryFn: listDeployments,
    refetchInterval: 5_000,
  });
  const deployments = data?.deployments ?? [];

  const refreshMutation = useMutation({
    mutationFn: (id: string) => refreshDeploymentStatus(id),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      toast({ title: `Status: ${d.status}`, description: d.status_reason });
    },
    onError: (e: any) => {
      toast({
        title: "Refresh failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteDeployment(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      setConfirmDelete(null);
      toast({ title: "Deployment removed" });
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  return (
    <div className="space-y-4 lg:space-y-6">
      <div>
        <h1 className="text-2xl lg:text-3xl font-bold">Deployments</h1>
        <p className="text-sm lg:text-base text-muted-foreground">
          Registry servers running as dedicated Kubernetes workloads. Each deployment is
          auto-registered as an MCP Gateway backend.
        </p>
      </div>

      {deployments.length === 0 ? (
        <EmptyState
          icon={icons.deployment}
          title="No active deployments"
          description="Open the Registry, click a server, and hit Deploy to spin up a Kubernetes workload that the MCP Gateway will route to."
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {deployments.map((d) => (
            <Card key={d.id}>
              <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2 gap-3">
                <div className="space-y-1 min-w-0">
                  <CardTitle className="text-base font-semibold truncate">
                    {d.resource_name}
                  </CardTitle>
                  <code className="text-xs font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded block truncate">
                    v{d.resource_version}
                  </code>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Badge className={statusColors[d.status]} variant="outline">
                    {d.status}
                  </Badge>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-8 w-8">
                        <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => refreshMutation.mutate(d.id)}>
                        <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                        Refresh status
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        className="text-destructive"
                        onClick={() => setConfirmDelete(d)}
                      >
                        <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
                        Undeploy
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <Badge variant="secondary" className="capitalize">{d.platform}</Badge>
                  <code className="text-xs font-mono text-muted-foreground">
                    {d.namespace}/{d.workload_name}
                  </code>
                </div>
                {d.endpoint && (
                  <div className="text-xs font-mono text-muted-foreground truncate">
                    → {d.endpoint}
                  </div>
                )}
                {d.status_reason && (
                  <p className="text-xs text-muted-foreground line-clamp-2">
                    {d.status_reason}
                  </p>
                )}
                {d.gateway_backend_id && (
                  <p className="text-[11px] text-muted-foreground">
                    MCP backend registered
                  </p>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <AlertDialog open={!!confirmDelete} onOpenChange={(open) => !open && setConfirmDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Undeploy workload?</AlertDialogTitle>
            <AlertDialogDescription>
              This will delete the Kubernetes Deployment/Service for{" "}
              <span className="font-mono">{confirmDelete?.workload_name}</span> and remove its MCP
              Gateway backend. Active sessions using it will fail.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => confirmDelete && deleteMutation.mutate(confirmDelete.id)}
            >
              Undeploy
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
