import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { LayoutGrid, Table as TableIcon } from "lucide-react";

import { getModels } from "@/lib/api";
import type { ModelsResponse, ModelWithUsage } from "@/types/api";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { detectProvider } from "@/lib/providers";

import ModelFilters, { ModelFilters as IModelFilters } from "@/components/models/ModelFilters";
import ModelsTable from "@/components/models/ModelsTable";
import ModelsCards from "@/components/models/ModelsCards";

type ViewMode = "cards" | "table";

export default function Models() {
  const [viewMode, setViewMode] = useState<ViewMode>("cards");
  const [filters, setFilters] = useState<IModelFilters>({
    search: "",
    providers: [],
    status: [],
  });

  const { data, isLoading } = useQuery({
    queryKey: ["models"],
    queryFn: getModels,
  });

  const rawModels = (data as ModelsResponse)?.data || [];

  // Transform models and add mock usage data for demonstration
  const modelsWithUsage: ModelWithUsage[] = useMemo(() => {
    return rawModels.map((model) => {
      const providerInfo = detectProvider(model.id, model.owned_by);
      
      // Mock usage data - in real implementation, this would come from API
      const mockUsage = {
        requests_today: Math.floor(Math.random() * 1000),
        requests_total: Math.floor(Math.random() * 10000) + 1000,
        tokens_today: Math.floor(Math.random() * 100000),
        tokens_total: Math.floor(Math.random() * 1000000) + 100000,
        cost_today: Math.random() * 10,
        cost_total: Math.random() * 100 + 10,
        avg_latency: Math.floor(Math.random() * 500) + 100,
        error_rate: Math.random() * 5,
        cache_hit_rate: Math.random() * 100,
        health_score: Math.floor(Math.random() * 30) + 70,
        trend_data: Array.from({ length: 30 }, () => Math.floor(Math.random() * 100)),
        last_used: new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000).toISOString(),
      };

      return {
        ...model,
        provider: providerInfo.name.toLowerCase(),
        is_active: Math.random() > 0.1, // 90% active
        usage_stats: Math.random() > 0.2 ? mockUsage : undefined, // 80% have usage data
      };
    });
  }, [rawModels]);

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
    <div className="space-y-4 lg:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">Models</h1>
          <p className="text-sm lg:text-base text-muted-foreground">
            Configure and manage LLM models
          </p>
        </div>

        {/* View Toggle */}
        <ToggleGroup 
          type="single" 
          value={viewMode} 
          onValueChange={(value: string) => value && setViewMode(value as ViewMode)}
          className="border"
        >
          <ToggleGroupItem value="cards" aria-label="Card view" className="gap-2">
            <LayoutGrid className="h-4 w-4" />
            Cards
          </ToggleGroupItem>
          <ToggleGroupItem value="table" aria-label="Table view" className="gap-2">
            <TableIcon className="h-4 w-4" />
            Table
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      {/* Filters */}
      <ModelFilters
        filters={filters}
        onFiltersChange={handleFiltersChange}
        onClearAll={handleClearFilters}
      />

      {/* Results Summary */}
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Showing {filteredModels.length} of {modelsWithUsage.length} models
        </p>
      </div>

      {/* Content */}
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
        <ModelsTable models={filteredModels} />
      ) : (
        <ModelsCards models={filteredModels} />
      )}
    </div>
  );
}
