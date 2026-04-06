import { useState, useEffect } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { detectProvider } from "@/lib/providers";

import { getRoute, getModels, createRoute, updateRoute, deleteRoute, getRouteStats } from "@/lib/api";
import type { Route, RouteModel, ModelsResponse, RouteStatsResponse } from "@/types/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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
  { value: "priority", label: "Priority", icon: "solar:arrow-to-top-left-linear", desc: "Highest priority first" },
  { value: "least-latency", label: "Fastest", icon: "solar:bolt-linear", desc: "Lowest latency" },
  { value: "weighted-round-robin", label: "Weighted", icon: "solar:chart-2-linear", desc: "By weight" },
  { value: "random", label: "Random", icon: "solar:shuffle-linear", desc: "Random pick" },
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

  // Right panel tab
  const [activePanel, setActivePanel] = useState<"settings" | "fallback" | "traffic">("settings");

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

  const panelTabs = [
    { id: "settings" as const, label: "Settings", icon: icons.settings },
    { id: "fallback" as const, label: "Fallback", icon: icons.layers },
    ...(!isNew ? [{ id: "traffic" as const, label: "Traffic", icon: icons.trendingUp }] : []),
  ];

  return (
    <div className="flex flex-col h-[calc(100vh-6rem)]">
      {/* Compact header */}
      <div className="flex items-center justify-between pb-4 flex-shrink-0">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate("/routes")}>
            <Icon icon={icons.arrowLeft} className="h-4 w-4" />
          </Button>
          <div className="flex items-center gap-3">
            <h1 className="text-lg font-bold tracking-tight">{isNew ? "Create Route" : name}</h1>
            {!isNew && (
              <div className="flex items-center gap-1.5">
                <code className="text-[11px] font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                  {slug}
                </code>
                <div className={`w-2 h-2 rounded-full ${enabled ? "bg-emerald-500" : "bg-zinc-400"}`} />
              </div>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {!isNew && !isSystem && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive gap-1.5 h-8">
                  <Icon icon={icons.delete} className="h-3.5 w-3.5" />
                  Delete
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete Route</AlertDialogTitle>
                  <AlertDialogDescription>
                    Are you sure you want to delete &quot;{name}&quot;? This action cannot be undone.
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
            <Button onClick={handleSave} disabled={saveMutation.isPending} size="sm" className="gap-1.5 h-8">
              <Icon icon={icons.check} className="h-3.5 w-3.5" />
              {saveMutation.isPending ? "Saving..." : "Save"}
            </Button>
          )}
        </div>
      </div>

      {/* Main layout: Diagram left, panels right */}
      <div className="flex gap-4 flex-1 min-h-0">
        {/* Left: Flow Diagram — takes ~65% */}
        <div className="flex-[2] min-w-0 flex flex-col">
          <div className="flex-1 min-h-0 border border-border rounded-xl overflow-hidden">
            <RouteFlowDiagram
              models={models}
              strategy={strategy}
              routeName={name || "Route"}
              routeSlug={slug || "slug"}
              availableModels={availableModels}
              onChange={readOnly ? () => {} : setModels}
            />
          </div>

          {/* Model chips beneath diagram */}
          {models.length > 0 && (
            <div className="flex flex-wrap gap-1.5 pt-3">
              {models.map((rm, i) => {
                const info = detectProvider(rm.model_name, "");
                return (
                  <div
                    key={i}
                    className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg border text-xs ${
                      rm.enabled
                        ? "border-border bg-background"
                        : "border-border/50 bg-muted/50 opacity-60"
                    }`}
                  >
                    <Icon icon={info.icon} width={12} height={12} className={info.color} />
                    <span className="font-mono font-medium truncate max-w-[160px]">{rm.model_name}</span>
                    <span className="text-muted-foreground">w:{rm.weight}</span>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Right: Tabbed panel — takes ~35% */}
        <div className="flex-1 min-w-[300px] max-w-[400px] flex flex-col border border-border rounded-xl overflow-hidden bg-background">
          {/* Tab bar */}
          <div className="flex border-b border-border bg-muted/30 flex-shrink-0">
            {panelTabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActivePanel(tab.id)}
                className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-2.5 text-xs font-medium transition-colors ${
                  activePanel === tab.id
                    ? "text-foreground border-b-2 border-primary bg-background"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                <Icon icon={tab.icon} className="h-3.5 w-3.5" />
                {tab.label}
              </button>
            ))}
          </div>

          {/* Panel content */}
          <div className="flex-1 overflow-y-auto p-4">
            {/* ── Settings Panel ── */}
            {activePanel === "settings" && (
              <div className="space-y-4">
                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Name</Label>
                  <Input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Smart Router"
                    disabled={readOnly}
                    className="h-8 text-sm"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Slug</Label>
                  <Input
                    value={slug}
                    onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ""))}
                    placeholder="smart"
                    className="h-8 text-sm font-mono"
                    disabled={readOnly}
                  />
                  <p className="text-[10px] text-muted-foreground">
                    API model name: <code className="bg-muted px-1 rounded">{slug || "slug"}</code>
                  </p>
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Description</Label>
                  <Textarea
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="Route description..."
                    rows={2}
                    disabled={readOnly}
                    className="text-sm resize-none"
                  />
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Strategy</Label>
                  <div className="grid grid-cols-2 gap-1.5">
                    {strategies.map((s) => {
                      const isActive = strategy === s.value;
                      return (
                        <button
                          key={s.value}
                          type="button"
                          disabled={readOnly}
                          onClick={() => setStrategy(s.value)}
                          className={`flex items-center gap-2 px-2.5 py-2 rounded-lg border text-left transition-all ${
                            isActive
                              ? "border-primary bg-primary/5 dark:bg-primary/10"
                              : "border-border hover:border-border hover:bg-muted/50"
                          } ${readOnly ? "opacity-60 cursor-not-allowed" : ""}`}
                        >
                          <div className={`w-6 h-6 rounded-md flex items-center justify-center flex-shrink-0 ${
                            isActive ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"
                          }`}>
                            <Icon icon={s.icon} className="h-3.5 w-3.5" />
                          </div>
                          <div className="min-w-0">
                            <div className={`text-xs font-medium leading-tight ${isActive ? "text-foreground" : "text-muted-foreground"}`}>{s.label}</div>
                            <div className="text-[10px] text-muted-foreground/70 leading-tight">{s.desc}</div>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2 border-t border-border">
                  <Label className="text-xs text-muted-foreground">Enabled</Label>
                  <Switch
                    checked={enabled}
                    onCheckedChange={setEnabled}
                    disabled={readOnly}
                  />
                </div>
              </div>
            )}

            {/* ── Fallback Panel ── */}
            {activePanel === "fallback" && (
              <div className="space-y-3">
                <p className="text-xs text-muted-foreground">
                  Models to try when all route models fail. Tried in order.
                </p>

                {fallbackModels.length === 0 && (
                  <div className="text-center py-6 text-xs text-muted-foreground/60">
                    No fallback models configured
                  </div>
                )}

                <div className="space-y-1.5">
                  {fallbackModels.map((fb, i) => {
                    const fbInfo = detectProvider(fb, "");
                    return (
                      <div key={i} className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg border border-border bg-muted/30">
                        <Icon icon={fbInfo.icon} width={14} height={14} className={fbInfo.color} />
                        <span className="text-xs font-mono flex-1 truncate">{fb}</span>
                        <span className="text-[10px] text-muted-foreground mr-1">#{i + 1}</span>
                        {!readOnly && (
                          <button
                            type="button"
                            className="text-muted-foreground hover:text-destructive transition-colors"
                            onClick={() => setFallbackModels(fallbackModels.filter((_, idx) => idx !== i))}
                          >
                            <Icon icon={icons.close} className="h-3 w-3" />
                          </button>
                        )}
                      </div>
                    );
                  })}
                </div>

                {!readOnly && (
                  <Select
                    onValueChange={(val) => {
                      if (val && !fallbackModels.includes(val)) {
                        setFallbackModels([...fallbackModels, val]);
                      }
                    }}
                  >
                    <SelectTrigger className="h-8 text-xs">
                      <SelectValue placeholder="Add fallback model..." />
                    </SelectTrigger>
                    <SelectContent>
                      {availableModels
                        .filter(m => !fallbackModels.includes(m) && !models.some(rm => rm.model_name === m))
                        .map((m) => {
                          const mInfo = detectProvider(m, "");
                          return (
                            <SelectItem key={m} value={m}>
                              <div className="flex items-center gap-2">
                                <Icon icon={mInfo.icon} width={14} height={14} className={mInfo.color} />
                                {m}
                              </div>
                            </SelectItem>
                          );
                        })}
                    </SelectContent>
                  </Select>
                )}
              </div>
            )}

            {/* ── Traffic Panel ── */}
            {activePanel === "traffic" && !isNew && (
              <div className="space-y-4">
                {/* Period selector */}
                <div className="flex items-center gap-1">
                  {[
                    { label: "1h", value: 1 },
                    { label: "24h", value: 24 },
                    { label: "7d", value: 168 },
                  ].map((p) => (
                    <button
                      key={p.value}
                      type="button"
                      onClick={() => setStatsPeriod(p.value)}
                      className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                        statsPeriod === p.value
                          ? "bg-primary text-primary-foreground font-medium"
                          : "text-muted-foreground hover:text-foreground hover:bg-muted"
                      }`}
                    >
                      {p.label}
                    </button>
                  ))}
                </div>

                {statsLoading ? (
                  <div className="flex items-center justify-center h-24">
                    <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary"></div>
                  </div>
                ) : !routeStats || routeStats.total_requests === 0 ? (
                  <div className="text-center py-8 text-xs text-muted-foreground/60">
                    No traffic recorded yet
                  </div>
                ) : (
                  <>
                    {/* Summary stats */}
                    <div className="grid grid-cols-3 gap-2">
                      {[
                        { label: "Requests", value: routeStats.total_requests.toLocaleString() },
                        { label: "Tokens", value: routeStats.total_tokens.toLocaleString() },
                        { label: "Cost", value: `$${routeStats.total_cost.toFixed(4)}` },
                      ].map((stat) => (
                        <div key={stat.label} className="text-center p-2 rounded-lg bg-muted/40">
                          <div className="text-sm font-bold">{stat.value}</div>
                          <div className="text-[10px] text-muted-foreground">{stat.label}</div>
                        </div>
                      ))}
                    </div>

                    {/* Per-model breakdown */}
                    <div className="space-y-2">
                      {routeStats.models.map((m) => {
                        const info = detectProvider(m.model, "");
                        return (
                          <div key={`${m.model}-${m.provider}`} className="space-y-1.5">
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-1.5 min-w-0">
                                <Icon icon={info.icon} width={12} height={12} className={info.color} />
                                <span className="text-xs font-mono truncate">{m.model}</span>
                              </div>
                              <span className="text-xs font-bold text-foreground">{m.percentage}%</span>
                            </div>
                            <div className="w-full bg-muted rounded-full h-1.5">
                              <div
                                className="bg-primary rounded-full h-1.5 transition-all"
                                style={{ width: `${m.percentage}%` }}
                              />
                            </div>
                            <div className="flex gap-3 text-[10px] text-muted-foreground">
                              <span>{m.requests} req</span>
                              <span>{m.avg_latency}ms</span>
                              <span>${m.cost.toFixed(4)}</span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
