import { useMemo, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  deleteUseCase,
  getUseCase,
  listUseCases,
} from "@/lib/api";
import type {
  UseCase,
  UseCaseRef,
  UseCaseRiskTier,
  UseCaseStatus,
  UseCaseWithRefs,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { EmptyState } from "@/components/common/EmptyState";
import { KindIcon } from "@/components/registry/kind-icon";
import { UseCaseGraph } from "@/components/use_cases/use-case-graph";
import {
  ClassificationMix,
  CoverageRing,
  RiskMatrix,
} from "@/components/use_cases/use-case-stats";
import { UseCaseDrawer } from "@/components/use_cases/use-case-drawer";
import { PageShell } from "@/components/common/page-shell";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const RISK_TIERS: { value: UseCaseRiskTier; label: string; description: string }[] = [
  { value: "tier1", label: "Tier 1", description: "Highest impact — customer-facing decisions, financial exposure" },
  { value: "tier2", label: "Tier 2", description: "High — internal decisions, significant operational reliance" },
  { value: "tier3", label: "Tier 3", description: "Medium — productivity / summarization" },
  { value: "tier4", label: "Tier 4", description: "Low — exploratory, sandbox, experimentation" },
];

const STATUSES: { value: UseCaseStatus; label: string }[] = [
  { value: "draft", label: "Draft" },
  { value: "under_review", label: "Under review" },
  { value: "approved", label: "Approved" },
  { value: "retired", label: "Retired" },
];

// Risk-tier palette — ordered from hottest (tier1) to coolest (tier4).
// Color is reserved for meaning; tier badges are the one place we permit it.
const TIER_TONES: Record<UseCaseRiskTier, string> = {
  tier1: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
  tier2: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
  tier3: "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
  tier4: "bg-muted text-muted-foreground ring-border",
};

const STATUS_TONES: Record<UseCaseStatus, string> = {
  draft: "bg-muted text-muted-foreground ring-border",
  under_review: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
  approved: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-emerald-500/30",
  retired: "bg-muted text-muted-foreground/60 ring-border",
};

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function UseCases() {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [tierFilter, setTierFilter] = useState<string>("");
  const [createOpen, setCreateOpen] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);

  // For the portfolio charts we always want the unfiltered view so the
  // matrix/mix reflect the whole inventory, even while the operator
  // narrows the list below.
  const { data: allData } = useQuery({
    queryKey: ["use-cases-all"],
    queryFn: () => listUseCases(),
  });
  const all = allData?.use_cases ?? [];

  const { data, isLoading } = useQuery({
    queryKey: ["use-cases", { search, statusFilter, tierFilter }],
    queryFn: () =>
      listUseCases({
        search: search || undefined,
        status: statusFilter || undefined,
        risk_tier: tierFilter || undefined,
      }),
  });
  const items = data?.use_cases ?? [];

  const portfolio = useMemo(() => {
    const byStatus = { draft: 0, under_review: 0, approved: 0, retired: 0 };
    let enforcing = 0;
    let flagged = 0;
    for (const uc of all) {
      byStatus[uc.status]++;
      if (uc.enforce_scope) enforcing++;
      if (uc.compliance_flag) flagged++;
    }
    return {
      total: all.length,
      byStatus,
      enforcing,
      flagged,
    };
  }, [all]);

  return (
    <PageShell
      title="Use cases"
      description="The business-activity layer. Each use case declares the MCP servers, agents, skills, prompts, and LLMs it depends on — and carries the risk tier, owner, and data classifications you approve against."
      actions={
        <Button onClick={() => setCreateOpen(true)} className="gap-2">
          <Icon icon={icons.plus} className="h-4 w-4" />
          New use case
        </Button>
      }
    >
      {/* ────────────── Hero ────────────── */}
      <div className="rounded-lg border border-border bg-card p-5">
        <div className="grid grid-cols-1 md:grid-cols-[auto_1fr] gap-5 items-center">
          <div className="flex items-center gap-5">
            <CoverageRing
              enforcing={portfolio.enforcing}
              approved={portfolio.byStatus.approved}
            />
            <div>
              <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                Portfolio
              </div>
              <div className="text-2xl font-bold tabular-nums tracking-tight leading-none mt-1">
                {portfolio.total}
                <span className="text-xs font-normal font-mono uppercase tracking-widest text-muted-foreground ml-2">
                  use cases
                </span>
              </div>
              <div className="text-[11px] text-muted-foreground mt-1.5">
                {portfolio.enforcing} of {portfolio.byStatus.approved || "—"}{" "}
                approved in block mode
              </div>
            </div>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            <HeroStat
              icon={icons.edit}
              label="Draft"
              value={portfolio.byStatus.draft}
              tone="muted"
            />
            <HeroStat
              icon={icons.clock}
              label="Under review"
              value={portfolio.byStatus.under_review}
              tone={portfolio.byStatus.under_review > 0 ? "amber" : "muted"}
            />
            <HeroStat
              icon={icons.check}
              label="Approved"
              value={portfolio.byStatus.approved}
              tone={portfolio.byStatus.approved > 0 ? "emerald" : "muted"}
            />
            <HeroStat
              icon={icons.shieldAlert}
              label="Flagged"
              value={portfolio.flagged}
              tone={portfolio.flagged > 0 ? "rose" : "muted"}
            />
          </div>
        </div>
      </div>

      {/* ────────────── Charts ────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4">
        <RiskMatrix
          useCases={all}
          onSelect={(tier, status) => {
            setTierFilter(tier);
            setStatusFilter(status);
          }}
          activeCell={{
            tier: (tierFilter as any) || "",
            status: (statusFilter as any) || "",
          }}
        />
        <ClassificationMix useCases={all} />
      </div>

      {/* ────────────── Filters ────────────── */}
      <div className="flex flex-col md:flex-row md:items-center gap-3">
        <div className="relative flex-1 md:max-w-sm">
          <Icon
            icon={icons.search}
            className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
          />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search use cases…"
            className="pl-9 h-9"
          />
        </div>
        <Select value={statusFilter || "all"} onValueChange={(v) => setStatusFilter(v === "all" ? "" : v)}>
          <SelectTrigger className="h-9 w-[10rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            {STATUSES.map((s) => (
              <SelectItem key={s.value} value={s.value}>
                {s.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={tierFilter || "all"} onValueChange={(v) => setTierFilter(v === "all" ? "" : v)}>
          <SelectTrigger className="h-9 w-[10rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All tiers</SelectItem>
            {RISK_TIERS.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                {t.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* ────────────── Grid ────────────── */}
      {isLoading ? (
        <LoadingGrid />
      ) : items.length === 0 ? (
        <EmptyState
          icon={icons.useCase}
          title="No use cases yet"
          description="Create one to declare the components a business activity depends on — that's what risk approves."
          action={
            <Button onClick={() => setCreateOpen(true)} variant="outline" className="gap-2">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Create your first use case
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {items.map((uc) => (
            <UseCaseCard
              key={uc.id}
              row={uc}
              onClick={() => setDetailId(uc.id)}
            />
          ))}
        </div>
      )}

      <UseCaseDrawer
        open={createOpen}
        onOpenChange={setCreateOpen}
        existing={null}
      />

      {detailId && (
        <DetailDrawer
          id={detailId}
          onClose={() => setDetailId(null)}
        />
      )}
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// Hero stat tile
// ---------------------------------------------------------------------------

function HeroStat({
  icon,
  label,
  value,
  tone = "muted",
}: {
  icon: string;
  label: string;
  value: number;
  tone?: "emerald" | "amber" | "rose" | "muted";
}) {
  return (
    <div className="rounded-md border border-border/70 bg-background/40 px-3 py-2.5 flex items-center gap-2.5">
      <div
        className={cn(
          "h-8 w-8 rounded-md flex items-center justify-center shrink-0",
          tone === "emerald" &&
            "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
          tone === "amber" &&
            "bg-amber-500/10 text-amber-600 dark:text-amber-400",
          tone === "rose" && "bg-rose-500/10 text-rose-600 dark:text-rose-400",
          tone === "muted" && "bg-muted text-muted-foreground",
        )}
      >
        <Icon icon={icon} className="h-4 w-4" />
      </div>
      <div className="min-w-0">
        <div className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground leading-none">
          {label}
        </div>
        <div
          className={cn(
            "text-lg font-bold tabular-nums leading-none mt-1",
            tone === "emerald" && "text-emerald-600 dark:text-emerald-400",
            tone === "amber" && "text-amber-600 dark:text-amber-400",
            tone === "rose" && "text-rose-600 dark:text-rose-400",
          )}
        >
          {value}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

function UseCaseCard({ row, onClick }: { row: UseCase; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "group relative text-left w-full",
        "rounded-xl border border-border/80 bg-card",
        "hover:border-primary/40 hover:shadow-sm transition-all",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
        "p-4 space-y-3",
      )}
    >
      <div className="flex items-start gap-3 min-w-0">
        <div className="h-10 w-10 rounded-lg bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
          <Icon icon={icons.useCase} className="h-5 w-5" />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold text-[15px] leading-tight truncate">
            {row.name}
          </h3>
          <code className="text-[11px] font-mono text-muted-foreground block truncate mt-0.5">
            {row.slug}
          </code>
        </div>
        {row.compliance_flag && (
          <span
            className="shrink-0"
            title={row.compliance_flag_reason || "Compliance issue"}
          >
            <Icon
              icon={icons.shieldAlert}
              className="h-4 w-4 text-rose-500"
            />
          </span>
        )}
      </div>

      {row.description && (
        <p className="text-[13px] leading-relaxed text-muted-foreground line-clamp-2">
          {row.description}
        </p>
      )}

      <div className="flex items-center flex-wrap gap-1.5 pt-1">
        <RiskBadge tier={row.risk_tier} />
        <StatusBadge status={row.status} />
        {row.enforce_scope && <EnforceBadge />}
        {row.data_classifications?.slice(0, 3).map((c) => (
          <span
            key={c}
            className="inline-flex items-center h-[18px] px-1.5 rounded bg-muted text-muted-foreground text-[10px] font-mono uppercase tracking-wider ring-1 ring-border"
          >
            {c}
          </span>
        ))}
        {(row.data_classifications?.length ?? 0) > 3 && (
          <span className="text-[10px] text-muted-foreground font-mono">
            +{(row.data_classifications ?? []).length - 3}
          </span>
        )}
      </div>

      {row.created_at && (
        <p className="text-[11px] text-muted-foreground flex items-center gap-1 pt-1">
          <Icon icon={icons.clock} className="h-3 w-3" />
          created {formatDistanceToNow(new Date(row.created_at), { addSuffix: true })}
        </p>
      )}
    </button>
  );
}

function RiskBadge({ tier }: { tier: UseCaseRiskTier }) {
  return (
    <span
      className={cn(
        "inline-flex items-center h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        TIER_TONES[tier],
      )}
    >
      {tier.replace("tier", "t")}
    </span>
  );
}

function StatusBadge({ status }: { status: UseCaseStatus }) {
  return (
    <span
      className={cn(
        "inline-flex items-center h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        STATUS_TONES[status],
      )}
    >
      {status.replace("_", " ")}
    </span>
  );
}

// EnforceBadge communicates the teeth-mode in a compact form. Only shown
// when enforce_scope is true; absence = flag mode is assumed.
function EnforceBadge() {
  return (
    <span
      className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/40"
      title="Out-of-scope calls are blocked, not just logged"
    >
      <Icon icon={icons.shield} className="h-2.5 w-2.5" />
      enforced
    </span>
  );
}

// ---------------------------------------------------------------------------
// Loading shimmer
// ---------------------------------------------------------------------------

function LoadingGrid() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl border border-border/60 bg-card p-4 space-y-3 animate-pulse"
        >
          <div className="flex gap-3">
            <div className="h-10 w-10 rounded-lg bg-muted" />
            <div className="flex-1 space-y-2">
              <div className="h-3 w-1/2 rounded bg-muted" />
              <div className="h-2 w-3/4 rounded bg-muted/70" />
            </div>
          </div>
          <div className="h-2 w-full rounded bg-muted/70" />
        </div>
      ))}
    </div>
  );
}


// ---------------------------------------------------------------------------
// Detail drawer
// ---------------------------------------------------------------------------

function DetailDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["use-case-detail", id],
    queryFn: () => getUseCase(id),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteUseCase(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["use-cases"] });
      toast({ title: "Use case deleted" });
      setConfirmDelete(false);
      onClose();
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  return (
    <>
      <Sheet open={true} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="sm:max-w-2xl w-full p-0 overflow-hidden flex flex-col">
          {/* Hero */}
          <SheetHeader className="px-6 pt-6 pb-4 border-b space-y-3">
            <div className="flex items-start gap-4">
              <div className="h-14 w-14 rounded-lg bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
                <Icon icon={icons.useCase} className="h-7 w-7" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                  Use case
                </p>
                <SheetTitle className="text-2xl font-semibold leading-tight tracking-tight truncate">
                  {data?.use_case.name ?? "…"}
                </SheetTitle>
                {data?.use_case.slug && (
                  <code className="text-xs font-mono text-muted-foreground block truncate mt-1">
                    {data.use_case.slug}
                  </code>
                )}
              </div>
            </div>
            <SheetDescription asChild>
              <div className="flex flex-wrap items-center gap-2">
                {data && <RiskBadge tier={data.use_case.risk_tier} />}
                {data && <StatusBadge status={data.use_case.status} />}
                {data?.use_case.enforce_scope && <EnforceBadge />}
                {data?.use_case.compliance_flag && (
                  <span className="inline-flex items-center gap-1 text-[11px] text-rose-600 dark:text-rose-400 font-mono">
                    <Icon icon={icons.shieldAlert} className="h-3.5 w-3.5" />
                    {data.use_case.compliance_flag_reason || "compliance flag"}
                  </span>
                )}
                {data?.use_case.created_at && (
                  <span className="text-[11px] font-mono text-muted-foreground">
                    created{" "}
                    {formatDistanceToNow(new Date(data.use_case.created_at), { addSuffix: true })}
                  </span>
                )}
              </div>
            </SheetDescription>
            <div className="flex items-center gap-2 pt-2">
              <Button
                size="sm"
                variant="default"
                onClick={() => setEditOpen(true)}
                className="gap-1.5"
                disabled={!data}
              >
                <Icon icon={icons.edit} className="h-3.5 w-3.5" />
                Edit
              </Button>
              <div className="ml-auto">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setConfirmDelete(true)}
                  className="text-destructive hover:text-destructive hover:bg-destructive/10 gap-1.5"
                >
                  <Icon icon={icons.delete} className="h-3.5 w-3.5" />
                  Delete
                </Button>
              </div>
            </div>
          </SheetHeader>

          {/* Body */}
          <div className="flex-1 overflow-y-auto">
            {isLoading || !data ? (
              <div className="flex items-center justify-center h-48">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
              </div>
            ) : (
              <Tabs defaultValue="overview" className="px-6 pt-4">
                <TabsList className="grid grid-cols-3 w-full max-w-md">
                  <TabsTrigger value="overview">Overview</TabsTrigger>
                  <TabsTrigger value="graph">Graph</TabsTrigger>
                  <TabsTrigger value="components">
                    Components{" "}
                    <span className="text-[10px] font-mono text-muted-foreground ml-1.5">
                      {totalRefs(data)}
                    </span>
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="overview" className="mt-4 pb-8 space-y-5">
                  {data.use_case.description && (
                    <p className="text-sm leading-relaxed">
                      {data.use_case.description}
                    </p>
                  )}
                  <DataClassifications
                    declared={data.use_case.data_classifications ?? []}
                    effective={data.use_case.effective_classifications ?? []}
                  />
                  <DepsCounts data={data} />
                </TabsContent>

                <TabsContent value="graph" className="mt-4 pb-8">
                  <UseCaseGraph data={data} />
                  <p className="text-[11px] text-muted-foreground mt-3 text-center">
                    Dependency tree · click the fit-to-view button to recenter.
                  </p>
                </TabsContent>

                <TabsContent value="components" className="mt-4 pb-8 space-y-5">
                  <ComponentSections data={data} />
                </TabsContent>
              </Tabs>
            )}
          </div>
        </SheetContent>
      </Sheet>

      {data && (
        <UseCaseDrawer open={editOpen} onOpenChange={setEditOpen} existing={data} />
      )}

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this use case?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes{" "}
              <span className="font-mono">{data?.use_case.slug}</span> and all
              its declared component edges. It does not touch the components
              themselves.
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
    </>
  );
}

function totalRefs(d: UseCaseWithRefs): number {
  return (
    (d.servers?.length ?? 0) +
    (d.agents?.length ?? 0) +
    (d.skills?.length ?? 0) +
    (d.prompts?.length ?? 0) +
    (d.llms?.length ?? 0)
  );
}

function DataClassifications({
  declared,
  effective,
}: {
  declared: string[];
  effective: string[];
}) {
  if (declared.length === 0 && effective.length === 0) return null;
  const extra = effective.filter((c) => !declared.includes(c));
  return (
    <section className="space-y-2">
      <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        Data classifications
      </p>
      <div className="flex flex-wrap gap-1.5">
        {declared.map((c) => (
          <span
            key={c}
            className="inline-flex items-center h-6 px-2 rounded bg-foreground/5 ring-1 ring-border text-[11px] font-mono uppercase tracking-wider"
          >
            {c}
          </span>
        ))}
        {extra.map((c) => (
          <span
            key={c}
            className="inline-flex items-center h-6 px-2 rounded bg-amber-500/10 ring-1 ring-amber-500/30 text-[11px] font-mono uppercase tracking-wider text-amber-700 dark:text-amber-300"
            title="Inherited from a component dependency"
          >
            ↓ {c}
          </span>
        ))}
      </div>
      {extra.length > 0 && (
        <p className="text-[11px] text-muted-foreground">
          {extra.length} class{extra.length === 1 ? "" : "es"} inherited from dependencies.
        </p>
      )}
    </section>
  );
}

function DepsCounts({ data }: { data: UseCaseWithRefs }) {
  const tiles: { label: string; count: number; kind: "server" | "agent" | "skill" | "prompt" | "llm" }[] = [
    { label: "Servers", count: data.servers?.length ?? 0, kind: "server" },
    { label: "Agents", count: data.agents?.length ?? 0, kind: "agent" },
    { label: "LLMs", count: data.llms?.length ?? 0, kind: "llm" },
    { label: "Skills", count: data.skills?.length ?? 0, kind: "skill" },
    { label: "Prompts", count: data.prompts?.length ?? 0, kind: "prompt" },
  ];
  return (
    <section className="space-y-2">
      <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        Dependency counts
      </p>
      <div className="grid grid-cols-5 gap-2">
        {tiles.map((t) => (
          <DepTile key={t.label} {...t} />
        ))}
      </div>
    </section>
  );
}

function DepTile({
  label,
  count,
  kind,
}: {
  label: string;
  count: number;
  kind: "server" | "agent" | "skill" | "prompt" | "llm";
}) {
  // LLMs don't have a KindIcon yet — we use a plain tile.
  return (
    <div
      className={cn(
        "rounded-lg border p-2 flex flex-col items-center gap-1",
        count > 0 ? "border-border/80 bg-card" : "border-border/40 bg-muted/30",
      )}
    >
      {kind === "llm" ? (
        <div className="h-7 w-7 rounded-lg bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-1 ring-violet-500/20 flex items-center justify-center">
          <Icon icon={icons.llm} className="h-4 w-4" />
        </div>
      ) : (
        <KindIcon kind={kind} size="sm" />
      )}
      <span className="text-lg font-semibold tabular-nums leading-none">
        {count}
      </span>
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        {label}
      </span>
    </div>
  );
}

function ComponentSections({ data }: { data: UseCaseWithRefs }) {
  const sections: {
    label: string;
    kind: "server" | "agent" | "skill" | "prompt" | "llm";
    refs: UseCaseRef[] | undefined;
  }[] = [
    { label: "MCP Servers", kind: "server", refs: data.servers },
    { label: "Agents", kind: "agent", refs: data.agents },
    { label: "LLMs", kind: "llm", refs: data.llms },
    { label: "Skills", kind: "skill", refs: data.skills },
    { label: "Prompts", kind: "prompt", refs: data.prompts },
  ];
  const anyContent = sections.some((s) => (s.refs?.length ?? 0) > 0);
  if (!anyContent) {
    return (
      <div className="text-center py-12">
        <Icon icon={icons.useCase} className="h-10 w-10 mx-auto text-muted-foreground mb-3" />
        <p className="text-sm text-muted-foreground">
          No components declared yet.
        </p>
        <p className="text-xs text-muted-foreground mt-1">
          Click Edit to add MCP servers, agents, LLMs, skills, and prompts.
        </p>
      </div>
    );
  }
  return (
    <>
      {sections.map((s) =>
        !s.refs || s.refs.length === 0 ? null : (
          <section key={s.label} className="space-y-2">
            <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              {s.label}{" "}
              <span className="text-foreground/70">({s.refs.length})</span>
            </p>
            <div className="space-y-1.5">
              {s.refs.map((r, i) => (
                <div
                  key={i}
                  className="flex items-center gap-3 p-2.5 rounded-md border border-border/60 bg-card"
                >
                  {s.kind === "llm" ? (
                    <div className="h-8 w-8 rounded-lg bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-1 ring-violet-500/20 flex items-center justify-center shrink-0">
                      <Icon icon={icons.llm} className="h-4 w-4" />
                    </div>
                  ) : (
                    <KindIcon kind={s.kind} size="sm" />
                  )}
                  <div className="flex-1 min-w-0">
                    <code className="text-[12px] font-mono truncate block">
                      {r.target_name}
                    </code>
                    <p className="text-[10px] text-muted-foreground">
                      {r.local_name && (
                        <>
                          alias{" "}
                          <code className="font-mono text-foreground/80">
                            {r.local_name}
                          </code>
                          {" · "}
                        </>
                      )}
                      pinned to{" "}
                      <code className="font-mono text-foreground/80">
                        {r.target_version ? `v${r.target_version}` : "latest"}
                      </code>
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ),
      )}
    </>
  );
}

