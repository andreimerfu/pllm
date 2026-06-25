import { useState } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { getAllProviders } from "@/lib/providers";

export interface ModelFilters {
  search: string;
  providers: string[];
  status: string[];
}

interface ModelFiltersProps {
  filters: ModelFilters;
  onFiltersChange: (filters: ModelFilters) => void;
  onClearAll: () => void;
}

const STATUS_OPTIONS = [
  { value: "active", label: "Active" },
  { value: "inactive", label: "Inactive" },
  { value: "error", label: "Error" },
];

export default function ModelFilters({ filters, onFiltersChange, onClearAll }: ModelFiltersProps) {
  const [searchInput, setSearchInput] = useState(filters.search);
  const providers = getAllProviders();

  const handleSearchChange = (value: string) => {
    setSearchInput(value);
    onFiltersChange({ ...filters, search: value });
  };

  const handleProviderToggle = (provider: string) => {
    const newProviders = filters.providers.includes(provider)
      ? filters.providers.filter(p => p !== provider)
      : [...filters.providers, provider];
    onFiltersChange({ ...filters, providers: newProviders });
  };

  const handleStatusToggle = (status: string) => {
    const newStatus = filters.status.includes(status)
      ? filters.status.filter(s => s !== status)
      : [...filters.status, status];
    onFiltersChange({ ...filters, status: newStatus });
  };

  const hasActiveFilters = filters.search || filters.providers.length > 0 || filters.status.length > 0;

  const providerLabel =
    filters.providers.length === 0
      ? "All providers"
      : filters.providers.length === 1
        ? filters.providers[0]
        : `${filters.providers.length} providers`;

  const statusLabel =
    filters.status.length === 0
      ? "All statuses"
      : filters.status.length === 1
        ? STATUS_OPTIONS.find((s) => s.value === filters.status[0])?.label ?? filters.status[0]
        : `${filters.status.length} statuses`;

  return (
    <div className="flex flex-1 flex-wrap items-center gap-2">
      {/* Search */}
      <div className="relative min-w-[200px] flex-1">
        <Icon
          icon={icons.search}
          className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          placeholder="Search models..."
          value={searchInput}
          onChange={(e) => handleSearchChange(e.target.value)}
          className="pl-9"
        />
      </div>

      {/* Provider Filter */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" className="gap-2 capitalize">
            {providerLabel}
            <Icon icon={icons.chevronDown} className="h-4 w-4 text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          <DropdownMenuLabel>Filter by provider</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {providers.map((provider) => (
            <DropdownMenuCheckboxItem
              key={provider}
              checked={filters.providers.includes(provider)}
              onCheckedChange={() => handleProviderToggle(provider)}
              className="capitalize"
            >
              {provider}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Status Filter */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" className="gap-2">
            {statusLabel}
            <Icon icon={icons.chevronDown} className="h-4 w-4 text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          <DropdownMenuLabel>Filter by status</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {STATUS_OPTIONS.map((status) => (
            <DropdownMenuCheckboxItem
              key={status.value}
              checked={filters.status.includes(status.value)}
              onCheckedChange={() => handleStatusToggle(status.value)}
            >
              {status.label}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Clear All */}
      {hasActiveFilters && (
        <Button variant="ghost" onClick={onClearAll} className="gap-2 text-muted-foreground">
          <Icon icon={icons.close} className="h-4 w-4" />
          Clear
        </Button>
      )}
    </div>
  );
}
