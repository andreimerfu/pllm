import { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Save, Trash2, BarChart3 } from "lucide-react";

import { getRoute, getModels, createRoute, updateRoute, deleteRoute, getRouteStats } from "@/lib/api";
import type { Route, RouteModel, ModelsResponse, RouteStatsResponse } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { useToast } from "@/hooks/use-toast";
import { RouteFlowDiagram } from "@/components/routes/RouteFlowDiagram";

const strategies = [
  { value: "priority", label: "Priority", description: "Select model with highest priority" },
  { value: "least-latency", label: "Least Latency", description: "Select model with lowest latency" },
  { value: "weighted-round-robin", label: "Weighted Round Robin", description: "Distribute traffic by weight" },
  { value: "random", label: "Random", description: "Randomly select a model" },
];

export default function RouteDetail() {
  const { routeId } = useParams<{ routeId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const isNew = !routeId;

  // Form state
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [strategy, setStrategy] = useState("priority");
  const [models, setModels] = useState<RouteModel[]>([]);
  const [fallbackModels, setFallbackModels] = useState<string[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [isSystem, setIsSystem] = useState(false);

  // Fetch existing route
  const { data: routeData, isLoading: routeLoading } = useQuery({
    queryKey: ["route", routeId],
    queryFn: () => getRoute(routeId!),
    enabled: !!routeId,
  });

  // Fetch available models
  const { data: modelsData } = useQuery({
    queryKey: ["models"],
    queryFn: getModels,
  });

  const availableModels = ((modelsData as ModelsResponse)?.data || []).map(m => m.id);

  // Traffic distribution stats (only for existing routes)
  const [statsPeriod, setStatsPeriod] = useState(24);
  const { data: statsData, isLoading: statsLoading } = useQuery({
    queryKey: ["routeStats", routeId, statsPeriod],
    queryFn: () => getRouteStats(routeId!, statsPeriod),
    enabled: !!routeId,
    refetchInterval: 30000,
  });
  const routeStats = statsData as RouteStatsResponse | undefined;

  // Populate form from existing route
  useEffect(() => {
    const route = routeData as Route | undefined;
    if (route) {
      setName(route.name);
      setSlug(route.slug);
      setDescription(route.description || "");
      setStrategy(route.strategy);
      setModels(route.models || []);
      setFallbackModels(route.fallback_models || []);
      setEnabled(route.enabled);
      setIsSystem(route.source === "system");
    }
  }, [routeData]);

  // Auto-generate slug from name for new routes
  useEffect(() => {
    if (isNew && name) {
      const generated = name
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-|-$/g, "");
      setSlug(generated);
    }
  }, [name, isNew]);

  // Save mutation
  const saveMutation = useMutation({
    mutationFn: (data: any) => {
      if (isNew) {
        return createRoute(data);
      }
      return updateRoute(routeId!, data);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["routes"] });
      queryClient.invalidateQueries({ queryKey: ["route", routeId] });
      toast({ title: isNew ? "Route created successfully" : "Route updated successfully" });
      navigate("/routes");
    },
    onError: (error: any) => {
      toast({
        title: "Failed to save route",
        description: error.response?.data?.error || error.message,
        variant: "destructive",
      });
    },
  });

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: () => deleteRoute(routeId!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["routes"] });
      toast({ title: "Route deleted successfully" });
      navigate("/routes");
    },
    onError: (error: any) => {
      toast({
        title: "Failed to delete route",
        description: error.response?.data?.error || error.message,
        variant: "destructive",
      });
    },
  });

  const handleSave = () => {
    if (!name.trim()) {
      toast({ title: "Name is required", variant: "destructive" });
      return;
    }
    if (!slug.trim()) {
      toast({ title: "Slug is required", variant: "destructive" });
      return;
    }
    if (models.length === 0) {
      toast({ title: "At least one model is required", variant: "destructive" });
      return;
    }

    saveMutation.mutate({
      name: name.trim(),
      slug: slug.trim(),
      description: description.trim(),
      strategy,
      models: models.map(m => ({
        model_name: m.model_name,
        weight: m.weight,
        priority: m.priority,
        enabled: m.enabled,
      })),
      fallback_models: fallbackModels.filter(Boolean),
      enabled,
    });
  };

  if (!isNew && routeLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const readOnly = isSystem;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => navigate("/routes")}>
            <ArrowLeft className="h-5 w-5" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold">{isNew ? "Create Route" : name}</h1>
            {!isNew && (
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="outline" className="font-mono">{slug}</Badge>
                <Badge variant={isSystem ? "secondary" : "default"}>
                  {isSystem ? "System" : "User"}
                </Badge>
              </div>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {!isNew && !isSystem && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm" className="gap-2">
                  <Trash2 className="h-4 w-4" />
                  Delete
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete Route</AlertDialogTitle>
                  <AlertDialogDescription>
                    Are you sure you want to delete "{name}"? This action cannot be undone.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    onClick={() => deleteMutation.mutate()}
                  >
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
          {!readOnly && (
            <Button onClick={handleSave} disabled={saveMutation.isPending} className="gap-2">
              <Save className="h-4 w-4" />
              {saveMutation.isPending ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </div>

      {/* Route Settings + Fallback — horizontal row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Route Settings</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Smart Router"
                  disabled={readOnly}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="slug">Slug</Label>
                <Input
                  id="slug"
                  value={slug}
                  onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ""))}
                  placeholder="smart"
                  className="font-mono"
                  disabled={readOnly}
                />
                <p className="text-xs text-muted-foreground">
                  Clients use this as the model name: <code>model: "{slug || "slug"}"</code>
                </p>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Route description..."
                rows={2}
                disabled={readOnly}
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Strategy</Label>
                <Select value={strategy} onValueChange={setStrategy} disabled={readOnly}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {strategies.map((s) => (
                      <SelectItem key={s.value} value={s.value}>
                        <div>
                          <div className="font-medium">{s.label}</div>
                          <div className="text-xs text-muted-foreground">{s.description}</div>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="flex items-center justify-between sm:justify-start sm:gap-4 pt-6">
                <Label htmlFor="enabled">Enabled</Label>
                <Switch
                  id="enabled"
                  checked={enabled}
                  onCheckedChange={setEnabled}
                  disabled={readOnly}
                />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Fallback Models */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Fallback Models</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground mb-3">
              Models to try when all route models fail. These are tried in order.
            </p>
            <div className="space-y-2">
              {fallbackModels.map((fb, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Badge variant="secondary" className="font-mono flex-1 justify-start">
                    {fb}
                  </Badge>
                  {!readOnly && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => setFallbackModels(fallbackModels.filter((_, idx) => idx !== i))}
                    >
                      <span className="text-xs text-destructive">x</span>
                    </Button>
                  )}
                </div>
              ))}
              {!readOnly && (
                <Select
                  onValueChange={(val) => {
                    if (val && !fallbackModels.includes(val)) {
                      setFallbackModels([...fallbackModels, val]);
                    }
                  }}
                >
                  <SelectTrigger className="text-xs">
                    <SelectValue placeholder="Add fallback model..." />
                  </SelectTrigger>
                  <SelectContent>
                    {availableModels
                      .filter(m => !fallbackModels.includes(m) && !models.some(rm => rm.model_name === m))
                      .map((m) => (
                        <SelectItem key={m} value={m}>{m}</SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Traffic Distribution — only for existing routes */}
      {!isNew && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
            <CardTitle className="text-base flex items-center gap-2">
              <BarChart3 className="h-4 w-4" />
              Traffic Distribution
            </CardTitle>
            <div className="flex items-center gap-1">
              {[
                { label: "1h", value: 1 },
                { label: "24h", value: 24 },
                { label: "7d", value: 168 },
              ].map((p) => (
                <Button
                  key={p.value}
                  variant={statsPeriod === p.value ? "default" : "outline"}
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => setStatsPeriod(p.value)}
                >
                  {p.label}
                </Button>
              ))}
            </div>
          </CardHeader>
          <CardContent>
            {statsLoading ? (
              <div className="flex items-center justify-center h-24">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
              </div>
            ) : !routeStats || routeStats.total_requests === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-6">
                No traffic recorded for this route yet.
              </p>
            ) : (
              <div className="space-y-4">
                {/* Summary */}
                <div className="grid grid-cols-3 gap-4 text-center">
                  <div>
                    <div className="text-2xl font-bold">{routeStats.total_requests.toLocaleString()}</div>
                    <div className="text-xs text-muted-foreground">Requests</div>
                  </div>
                  <div>
                    <div className="text-2xl font-bold">{routeStats.total_tokens.toLocaleString()}</div>
                    <div className="text-xs text-muted-foreground">Tokens</div>
                  </div>
                  <div>
                    <div className="text-2xl font-bold">${routeStats.total_cost.toFixed(4)}</div>
                    <div className="text-xs text-muted-foreground">Cost</div>
                  </div>
                </div>

                {/* Per-model bars + table */}
                <div className="space-y-2">
                  {routeStats.models.map((m) => (
                    <div key={`${m.model}-${m.provider}`} className="space-y-1">
                      <div className="flex items-center justify-between text-sm">
                        <div className="flex items-center gap-2">
                          <span className="font-mono font-medium">{m.model}</span>
                          <Badge variant="outline" className="text-xs">{m.provider}</Badge>
                        </div>
                        <div className="flex items-center gap-3 text-xs text-muted-foreground">
                          <span>{m.requests.toLocaleString()} req</span>
                          <span>{m.avg_latency}ms</span>
                          <span>${m.cost.toFixed(4)}</span>
                          <span className="font-medium text-foreground">{m.percentage}%</span>
                        </div>
                      </div>
                      <div className="w-full bg-muted rounded-full h-2">
                        <div
                          className="bg-primary rounded-full h-2 transition-all"
                          style={{ width: `${m.percentage}%` }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Flow Diagram — full width */}
      <Card>
        <CardContent className="pt-6">
          <RouteFlowDiagram
            models={models}
            strategy={strategy}
            routeName={name || "Route"}
            routeSlug={slug || "slug"}
            availableModels={availableModels}
            onChange={readOnly ? () => {} : setModels}
          />

          {/* Model list summary */}
          {models.length > 0 && (
            <div className="mt-4 space-y-2">
              <div className="text-sm font-medium">Models ({models.length})</div>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                {models.map((rm, i) => (
                  <div
                    key={i}
                    className="flex items-center justify-between p-2 rounded-lg border text-sm"
                  >
                    <div className="flex items-center gap-2">
                      <Badge variant={rm.enabled ? "default" : "secondary"} className="text-xs">
                        {rm.enabled ? "ON" : "OFF"}
                      </Badge>
                      <span className="font-mono">{rm.model_name}</span>
                    </div>
                    <div className="flex items-center gap-3 text-xs text-muted-foreground">
                      <span>Weight: {rm.weight}</span>
                      <span>Priority: {rm.priority}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
