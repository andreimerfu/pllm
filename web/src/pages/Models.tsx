import { useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueries } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { getModels, getDashboardMetrics, getAdminModels, getModelsHealth, getModelTrends } from "@/lib/api";
import type { ModelsResponse, ModelWithUsage, AdminModel, AdminModelsResponse, ModelsHealthResponse } from "@/types/api";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { detectProvider } from "@/lib/providers";
import { icons } from "@/lib/icons";

import ModelFilters, { ModelFilters as IModelFilters } from "@/components/models/ModelFilters";
import ModelsTable from "@/components/models/ModelsTable";
import ModelsCards from "@/components/models/ModelsCards";
import AddModelDialog from "@/components/models/AddModelDialog";
import DeleteModelDialog from "@/components/models/DeleteModelDialog";
import { PageShell, StatTile } from "@/components/common/page-shell";

type ViewMode = "cards" | "table";

export default function Models() {
  const navigate = useNavigate();
  const [viewMode, setViewMode] = useState<ViewMode>("table");
  const [filters, setFilters] = useState<IModelFilters>({
    search: "",
    providers: [],
    status: [],
  });
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["models"],
    queryFn: getModels,
  });

  const { data: dashboardData } = useQuery({
    queryKey: ["dashboard-metrics"],
    queryFn: () => getDashboardMetrics(),
    refetchInterval: 60000,
  });

  const { data: adminModelsData } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
    refetchInterval: 60000,
  });

  const { data: healthData } = useQuery({
    queryKey: ["models-health"],
    queryFn: getModelsHealth,
    refetchInterval: 60000,
  });

  // Build lookup from model_name to admin model for edit/delete actions
  const adminModelsByName = useMemo(() => {
    const data = adminModelsData as AdminModelsResponse | undefined;
    const map = new Map<string, AdminModel>();
    data?.models?.forEach((m) => {
      map.set(m.model_name, m);
    });
    return map;
  }, [adminModelsData]);

  const handleEdit = (model: ModelWithUsage) => {
    const admin = adminModelsByName.get(model.id);
    if (admin) {
      navigate(`/models/edit/${encodeURIComponent(admin.id)}`);
    }
  };

  const handleDelete = (model: ModelWithUsage) => {
    const admin = adminModelsByName.get(model.id);
    if (admin) {
      setDeleteTarget({ id: admin.id, name: admin.model_name });
    }
  };

  const rawModels = (data as ModelsResponse)?.data || [];

  // Real per-model 24h request series for the sparkline column. One light
  // query per model (cached + parallel by react-query) against the
  // /dashboard/models/{id}/trends endpoint. No fabricated data — a model
  // with no recorded traffic simply gets an empty series (renders "—").
  const trendQueries = useQueries({
    queries: rawModels.map((m) => ({
      queryKey: ["model-trends-24h", m.id],
      queryFn: () => getModelTrends(m.id, { hours: 24, interval: "hourly" }),
      refetchInterval: 60000,
      staleTime: 60000,
      retry: false,
    })),
  });
  const trendByModel = useMemo(() => {
    const map = new Map<string, number[]>();
    rawModels.forEach((m, i) => {
      const raw = trendQueries[i]?.data as any;
      const points = (raw?.data ?? raw ?? []) as any[];
      map.set(
        m.id,
        Array.isArray(points)
          ? points.map((p) => Number(p?.requests ?? p?.request_count ?? p?.count ?? 0))
          : [],
      );
    });
    return map;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rawModels, trendQueries.map((q) => q.dataUpdatedAt).join(",")]);

  // Transform models and add real usage data from API
  const modelsWithUsage: ModelWithUsage[] = useMemo(() => {
    const dashboard = (dashboardData as any) || {};
    const topModels = dashboard.top_models || [];
    const hd = healthData as ModelsHealthResponse | undefined;

    return rawModels.map((model) => {
      const providerInfo = detectProvider(model.id, model.owned_by);

      // Find usage data for this model from top_models
      const modelUsage = topModels.find((tm: any) => tm.model === model.id);

      // Derive health_score from real health check data when available
      const modelHealth = hd?.models?.[model.id];
      let healthScore: number;
      if (modelHealth && modelHealth.total_count > 0) {
        const ratio = modelHealth.healthy_count / modelHealth.total_count;
        healthScore = ratio >= 1 ? 100 : ratio > 0 ? 70 : 0;
      } else {
        healthScore = modelUsage?.health_score ?? 100;
      }

      const usageData = {
        requests_today: modelUsage?.requests || 0,
        requests_total: modelUsage?.requests || 0,
        tokens_today: modelUsage?.tokens || 0,
        tokens_total: modelUsage?.tokens || 0,
        cost_today: modelUsage?.cost || 0,
        cost_total: modelUsage?.cost || 0,
        avg_latency: modelUsage?.avg_latency || 0,
        error_rate: modelUsage?.success_rate ? (100 - modelUsage.success_rate) : 0,
        cache_hit_rate: modelUsage?.cache_hit_rate || 0,
        health_score: healthScore,
        p95_latency: modelUsage?.p95_latency || 0,
        p99_latency: modelUsage?.p99_latency || 0,
        trend_data: trendByModel.get(model.id) ?? [],
        last_used: modelUsage ? new Date().toISOString() : null,
      };

      // Use the backend's enabled field from admin models; default to true
      const adminModel = adminModelsByName.get(model.id);
      const isActive = adminModel?.enabled ?? true;

      return {
        ...model,
        provider: providerInfo.name.toLowerCase(),
        is_active: isActive,
        usage_stats: usageData,
      };
    });
  }, [rawModels, dashboardData, healthData, adminModelsByName, trendByModel]);

  // Filter models based on current filters
  const filteredModels = useMemo(() => {
    return modelsWithUsage.filter((model) => {
      // Search filter
      if (filters.search) {
        const searchLower = filters.search.toLowerCase();
        const matchesSearch = 
          model.id.toLowerCase().includes(searchLower) ||
          model.owned_by.toLowerCase().includes(searchLower) ||
          (model.provider && model.provider.toLowerCase().includes(searchLower));
        
        if (!matchesSearch) return false;
      }

      // Provider filter
      if (filters.providers.length > 0) {
        const modelProvider = detectProvider(model.id, model.owned_by).name.toLowerCase();
        if (!filters.providers.some(p => p.toLowerCase() === modelProvider)) {
          return false;
        }
      }

      // Status filter
      if (filters.status.length > 0) {
        const modelStatus = model.is_active ? "active" : "inactive";
        if (!filters.status.includes(modelStatus)) {
          return false;
        }
      }

      return true;
    });
  }, [modelsWithUsage, filters]);

  // KPI row — derived entirely from data already fetched. A card is only
  // rendered when it has real backing data (latency + 24h requests can be
  // empty when no health/usage has been recorded yet).
  const kpis = useMemo(() => {
    const total = modelsWithUsage.length;
    const active = modelsWithUsage.filter((m) => m.is_active !== false).length;
    const paused = total - active;

    // Distinct providers across the fleet.
    const providerNames = new Set<string>();
    modelsWithUsage.forEach((m) => {
      providerNames.add(detectProvider(m.id, m.owned_by).name);
    });

    // Average measured latency across models that actually have a value.
    const hd = healthData as ModelsHealthResponse | undefined;
    const latencies = Object.values(hd?.models ?? {})
      .map((h) => h.avg_latency_ms)
      .filter((v) => v > 0);
    const avgLatency = latencies.length
      ? Math.round(latencies.reduce((a, b) => a + b, 0) / latencies.length)
      : null;

    // Requests in the dashboard's default 24h window.
    const dashboard = (dashboardData as { top_models?: Array<{ requests: number }> }) || {};
    const topModels = dashboard.top_models || [];
    const requests24h = topModels.reduce((sum, tm) => sum + (tm.requests || 0), 0);
    const hasRequests = topModels.length > 0;

    return {
      total,
      active,
      paused,
      providerNames: Array.from(providerNames),
      avgLatency,
      requests24h,
      hasRequests,
    };
  }, [modelsWithUsage, healthData, dashboardData]);

  const handleFiltersChange = (newFilters: IModelFilters) => {
    setFilters(newFilters);
  };

  const handleClearFilters = () => {
    setFilters({
      search: "",
      providers: [],
      status: [],
    });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <PageShell
      title="Models"
      description="Every model your gateway can route to, across providers — with live health, latency, and pricing."
      actions={<AddModelDialog />}
      stats={
        <>
          <StatTile
            label="Models"
            value={kpis.total}
            icon={<Icon icon={icons.models} />}
            iconColor="model"
            sub="Across all providers"
          />
          <StatTile
            label="Active"
            value={kpis.active}
            tone={kpis.active > 0 ? "good" : "default"}
            icon={<Icon icon={icons.check} />}
            iconColor="good"
            sub={`${kpis.paused} paused`}
          />
          <StatTile
            label="Providers"
            value={kpis.providerNames.length}
            icon={<Icon icon={icons.layers} />}
            iconColor="mcp"
            sub={kpis.providerNames.length > 0 ? kpis.providerNames.join(", ") : "None yet"}
          />
          {kpis.avgLatency != null && (
            <StatTile
              label="Avg latency"
              value={`${kpis.avgLatency}ms`}
              icon={<Icon icon={icons.clock} />}
              iconColor="agent"
              sub="Measured across health checks"
            />
          )}
          {kpis.hasRequests && (
            <StatTile
              label="Requests (24h)"
              value={kpis.requests24h.toLocaleString()}
              icon={<Icon icon={icons.trendingUp} />}
              iconColor="prompt"
              sub="Last 24 hours"
            />
          )}
        </>
      }
      toolbar={
        <div className="flex items-center justify-between w-full gap-3">
          <ModelFilters
            filters={filters}
            onFiltersChange={handleFiltersChange}
            onClearAll={handleClearFilters}
          />
          <ToggleGroup
            type="single"
            value={viewMode}
            onValueChange={(value: string) => value && setViewMode(value as ViewMode)}
          >
            <ToggleGroupItem value="cards" aria-label="Card view" className="gap-2">
              <Icon icon="solar:widget-2-linear" className="h-4 w-4" />
              Cards
            </ToggleGroupItem>
            <ToggleGroupItem value="table" aria-label="Table view" className="gap-2">
              <Icon icon="solar:table-linear" className="h-4 w-4" />
              Table
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
      }
    >
      <div className="flex items-center justify-between">
        <p className="text-[12.5px] text-muted-foreground">
          Showing {filteredModels.length} of {modelsWithUsage.length} models
        </p>
      </div>

      {filteredModels.length === 0 ? (
        <div className="flex items-center justify-center h-64 border border-dashed rounded-lg">
          <div className="text-center">
            <div className="text-muted-foreground mb-2">No models found</div>
            <Button variant="outline" onClick={handleClearFilters}>
              Clear filters
            </Button>
          </div>
        </div>
      ) : viewMode === "table" ? (
        <ModelsTable models={filteredModels} onEdit={handleEdit} onDelete={handleDelete} />
      ) : (
        <ModelsCards models={filteredModels} onEdit={handleEdit} onDelete={handleDelete} />
      )}

      {deleteTarget && (
        <DeleteModelDialog
          modelId={deleteTarget.id}
          modelName={deleteTarget.name}
          open={!!deleteTarget}
          onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
        />
      )}
    </PageShell>
  );
}
