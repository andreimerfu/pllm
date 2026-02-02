import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, GitBranch, Trash2, Pencil, MoreHorizontal } from "lucide-react";

import { getRoutes, deleteRoute } from "@/lib/api";
import type { RoutesResponse, Route } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { useToast } from "@/hooks/use-toast";

const strategyColors: Record<string, string> = {
  priority: "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300",
  "least-latency": "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300",
  "weighted-round-robin": "bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300",
  random: "bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300",
};

export default function Routes() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [deleteTarget, setDeleteTarget] = useState<Route | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["routes"],
    queryFn: getRoutes,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteRoute(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["routes"] });
      toast({ title: "Route deleted successfully" });
      setDeleteTarget(null);
    },
    onError: (error: any) => {
      toast({
        title: "Failed to delete route",
        description: error.response?.data?.error || error.message,
        variant: "destructive",
      });
    },
  });

  const routes = (data as RoutesResponse)?.routes || [];

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div className="space-y-4 lg:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">Routes</h1>
          <p className="text-sm lg:text-base text-muted-foreground">
            Virtual endpoints that distribute traffic across multiple models
          </p>
        </div>
        <Button onClick={() => navigate("/routes/new")} className="gap-2">
          <Plus className="h-4 w-4" />
          Create Route
        </Button>
      </div>

      {/* Content */}
      {routes.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-64 border border-dashed rounded-lg">
          <GitBranch className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium mb-1">No routes configured</h3>
          <p className="text-sm text-muted-foreground mb-4">
            Create a route to distribute traffic across multiple models
          </p>
          <Button onClick={() => navigate("/routes/new")} variant="outline" className="gap-2">
            <Plus className="h-4 w-4" />
            Create Route
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {routes.map((route) => (
            <Card
              key={route.id}
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate(`/routes/${route.id}`)}
            >
              <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                <div className="space-y-1">
                  <CardTitle className="text-base font-semibold">{route.name}</CardTitle>
                  <Badge variant="outline" className="font-mono text-xs">
                    {route.slug}
                  </Badge>
                </div>
                <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                  <Badge variant={route.enabled ? "default" : "secondary"}>
                    {route.enabled ? "Enabled" : "Disabled"}
                  </Badge>
                  {route.source === "user" && (
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => navigate(`/routes/${route.id}`)}>
                          <Pencil className="mr-2 h-4 w-4" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          className="text-destructive"
                          onClick={() => setDeleteTarget(route)}
                        >
                          <Trash2 className="mr-2 h-4 w-4" />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                {route.description && (
                  <p className="text-sm text-muted-foreground mb-3 line-clamp-2">
                    {route.description}
                  </p>
                )}
                <div className="flex flex-wrap items-center gap-2 mb-3">
                  <Badge className={strategyColors[route.strategy] || "bg-gray-100 text-gray-800"}>
                    {route.strategy}
                  </Badge>
                  <Badge variant="outline">
                    {route.source}
                  </Badge>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {route.models.map((rm, i) => (
                    <Badge key={i} variant="secondary" className="text-xs font-mono">
                      {rm.model_name}
                    </Badge>
                  ))}
                </div>
                {route.models.length === 0 && (
                  <p className="text-xs text-muted-foreground">No models configured</p>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Delete confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open: boolean) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Route</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the route "{deleteTarget?.name}" (slug: {deleteTarget?.slug})?
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
