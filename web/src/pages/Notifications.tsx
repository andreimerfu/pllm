import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { listWebhooks, deleteWebhook, testWebhook } from "@/lib/api";
import type { Webhook } from "@/types/api";

import { PageShell, StatStrip, StatStripItem } from "@/components/common/page-shell";
import { WebhookDrawer } from "@/components/compliance/webhook-drawer";
import { visualFor } from "@/components/compliance/destination-visuals";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
import { GovernanceHelp } from "@/components/governance/help-popover";

// ---------------------------------------------------------------------------
// Notifications page — outbound HMAC-signed event delivery.
//
// Subscribe an external endpoint (SIEM, GRC, chat) to scope events and
// use-case lifecycle changes. Each subscription gets its own HMAC secret
// and last-delivery health pill. Used to live as a section on /compliance;
// promoted to a top-level page so admin CRUD doesn't crowd the dashboard.
// ---------------------------------------------------------------------------

export default function Notifications() {
  const { data, isLoading } = useQuery({
    queryKey: ["webhooks"],
    queryFn: listWebhooks,
    refetchInterval: 10_000,
  });
  const webhooks = data?.webhooks ?? [];

  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Webhook | null>(null);
  const [filter, setFilter] = useState("");

  const stats = useMemo(() => deriveStats(webhooks), [webhooks]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return webhooks;
    return webhooks.filter(
      (h) =>
        h.name.toLowerCase().includes(q) ||
        h.url.toLowerCase().includes(q) ||
        (h.events ?? []).some((e) => e.toLowerCase().includes(q)),
    );
  }, [webhooks, filter]);

  return (
    <PageShell
      title="Notifications"
      description="HMAC-signed event deliveries to your SIEM, GRC, or chat. Subscribe an HTTPS endpoint to scope events and use-case lifecycle changes."
      actions={
        <>
          <GovernanceHelp accent="audit" />
          <Button
            onClick={() => {
              setEditing(null);
              setCreateOpen(true);
            }}
            size="sm"
            className="gap-1.5"
          >
            <Icon icon={icons.plus} className="h-3.5 w-3.5" />
            New subscription
          </Button>
        </>
      }
      stats={
        <StatStrip className="col-span-full">
          <StatStripItem label="Subscriptions" value={stats.total} />
          <StatStripItem
            label="Enabled"
            value={stats.enabled}
            hint={stats.disabled > 0 ? `${stats.disabled} paused` : undefined}
          />
          <StatStripItem
            label="Healthy"
            value={stats.healthy}
            tone={stats.healthy > 0 ? "emerald" : "default"}
            hint="last delivery 2xx"
          />
          <StatStripItem
            label="Failing"
            value={stats.failing}
            tone={stats.failing > 0 ? "rose" : "default"}
            hint={stats.failing === 0 ? "all clear" : "non-2xx response"}
          />
          <StatStripItem
            label="Last delivery"
            value={stats.lastDeliveryLabel}
            hint={stats.lastDeliveryName ?? undefined}
          />
          <DestinationBreakdown webhooks={webhooks} />
        </StatStrip>
      }
      toolbar={
        webhooks.length > 0 ? (
          <div className="flex items-center gap-2 w-full">
            <div className="relative w-72 max-w-full">
              <Icon
                icon={icons.search}
                className="h-4 w-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder="Filter by name, URL, or event…"
                className="h-9 pl-8"
              />
            </div>
            <span className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground ml-auto">
              auto-refresh · 10s
            </span>
          </div>
        ) : undefined
      }
    >
      {isLoading ? (
        <div className="rounded-lg border border-border bg-card p-12 text-center">
          <div className="animate-spin mx-auto h-6 w-6 rounded-full border-b-2 border-primary" />
          <p className="text-sm text-muted-foreground mt-3">
            Loading subscriptions…
          </p>
        </div>
      ) : webhooks.length === 0 ? (
        <EmptyState onCreate={() => setCreateOpen(true)} />
      ) : filtered.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-10 text-center">
          <p className="text-sm text-muted-foreground">
            Nothing matches{" "}
            <code className="font-mono px-1 rounded bg-muted">{filter}</code>.
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-[180px]">Destination</TableHead>
                <TableHead>Subscription</TableHead>
                <TableHead>Events</TableHead>
                <TableHead className="w-[140px]">Last delivery</TableHead>
                <TableHead className="w-[160px] text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((h) => (
                <WebhookRow key={h.id} hook={h} onEdit={() => setEditing(h)} />
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <WebhookDrawer
        open={createOpen}
        onOpenChange={setCreateOpen}
        existing={null}
      />
      <WebhookDrawer
        open={!!editing}
        onOpenChange={(open) => !open && setEditing(null)}
        existing={editing}
      />
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// Stats derivation
// ---------------------------------------------------------------------------

function deriveStats(webhooks: Webhook[]) {
  const total = webhooks.length;
  const enabled = webhooks.filter((h) => h.enabled).length;
  const disabled = total - enabled;
  const triedAndEnabled = webhooks.filter(
    (h) => h.enabled && h.last_delivery_at,
  );
  const healthy = triedAndEnabled.filter((h) => {
    const s = h.last_delivery_status ?? 0;
    return s >= 200 && s < 300;
  }).length;
  const failing = triedAndEnabled.length - healthy;

  // Last delivery across the fleet — surfaces "is anything firing?"
  let last: Webhook | null = null;
  for (const h of webhooks) {
    if (!h.last_delivery_at) continue;
    if (!last || new Date(h.last_delivery_at) > new Date(last.last_delivery_at!)) {
      last = h;
    }
  }
  const lastDeliveryLabel = last
    ? formatDistanceToNow(new Date(last.last_delivery_at!), { addSuffix: true })
    : "never";
  const lastDeliveryName = last?.name ?? null;

  return {
    total,
    enabled,
    disabled,
    healthy,
    failing,
    lastDeliveryLabel,
    lastDeliveryName,
  };
}

// ---------------------------------------------------------------------------
// Destination breakdown — small inline column on the stat strip showing
// how many subscriptions exist per vendor. Helps an operator answer
// "are we wired into Splunk?" without opening every row.
// ---------------------------------------------------------------------------

function DestinationBreakdown({ webhooks }: { webhooks: Webhook[] }) {
  const counts: Record<string, number> = {};
  for (const h of webhooks) {
    const t = h.destination_type || "generic";
    counts[t] = (counts[t] ?? 0) + 1;
  }
  const entries = Object.entries(counts).sort((a, b) => b[1] - a[1]);
  if (entries.length === 0) {
    return (
      <div className="flex flex-col gap-0.5 px-4 py-2.5 min-w-[7.5rem]">
        <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground truncate">
          By destination
        </span>
        <span className="text-[12px] text-muted-foreground">—</span>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-1 px-4 py-2.5 min-w-[7.5rem]">
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground truncate">
        By destination
      </span>
      <div className="flex flex-wrap gap-1 mt-0.5">
        {entries.map(([type, count]) => {
          const v = visualFor(type as Webhook["destination_type"]);
          return (
            <span
              key={type}
              className="inline-flex items-center gap-1 h-[20px] px-1.5 rounded bg-muted/60 ring-1 ring-border text-[10px] font-mono"
              title={`${count} ${v.label}`}
            >
              <Icon icon={v.icon} className="h-3 w-3" />
              <span className="tabular-nums text-foreground/85">{count}</span>
            </span>
          );
        })}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------

function EmptyState({ onCreate }: { onCreate: () => void }) {
  return (
    <div className="rounded-lg border border-dashed border-border bg-card/30 p-12 text-center">
      <div className="mx-auto h-12 w-12 rounded-full bg-muted flex items-center justify-center mb-3">
        <Icon
          icon={icons.notifications}
          className="h-6 w-6 text-muted-foreground"
        />
      </div>
      <h3 className="text-base font-semibold tracking-tight">
        No subscriptions yet
      </h3>
      <p className="text-sm text-muted-foreground mt-1 max-w-md mx-auto">
        Wire pLLM events into your SIEM, GRC system, or a chat channel. Each
        subscription is HMAC-signed and retried on transient failures.
      </p>

      <div className="mt-5 mx-auto max-w-md text-left">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-2">
          What you can subscribe to
        </div>
        <ul className="space-y-1.5">
          <EventHint
            tag="scope.denied"
            text="A bound key called an LLM/tool its use case doesn't declare — blocked at runtime."
          />
          <EventHint
            tag="scope.flagged"
            text="Same as denied, but the use case runs in flag mode (logged, not blocked)."
          />
          <EventHint
            tag="use_case.compliance_flag"
            text="The reconciler raised a flag — a dependency leaked a data class the use case doesn't declare."
          />
          <EventHint
            tag="registry.server.saved"
            text="A registry server was published or updated — pair with risky-server policies."
          />
        </ul>
      </div>

      <Button onClick={onCreate} size="sm" className="mt-6 gap-1.5">
        <Icon icon={icons.plus} className="h-3.5 w-3.5" />
        Create your first subscription
      </Button>
    </div>
  );
}

function EventHint({ tag, text }: { tag: string; text: string }) {
  return (
    <li className="flex items-start gap-2.5 text-[12px]">
      <code className="shrink-0 font-mono text-[10.5px] tracking-tight bg-muted px-1.5 py-0.5 rounded ring-1 ring-border text-foreground/80 mt-0.5">
        {tag}
      </code>
      <span className="text-muted-foreground leading-relaxed">{text}</span>
    </li>
  );
}

// ---------------------------------------------------------------------------
// Webhook row — name + endpoint + events + last delivery + actions.
// ---------------------------------------------------------------------------

function WebhookRow({ hook, onEdit }: { hook: Webhook; onEdit: () => void }) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const testMutation = useMutation({
    mutationFn: () => testWebhook(hook.id),
    onSuccess: () => {
      toast({
        title: "Test delivery fired",
        description: "Status will update shortly.",
      });
      setTimeout(() => qc.invalidateQueries({ queryKey: ["webhooks"] }), 2000);
    },
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) => {
      toast({
        title: "Test failed to queue",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: () => deleteWebhook(hook.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["webhooks"] });
      setConfirmDelete(false);
      toast({ title: "Subscription deleted" });
    },
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const status = hook.last_delivery_status ?? 0;
  const healthy = status >= 200 && status < 300;
  const hasTried = !!hook.last_delivery_at;
  const visual = visualFor(hook.destination_type);

  return (
    <>
      <TableRow>
        <TableCell>
          <div className="flex items-center gap-2 min-w-0">
            <div
              className={cn(
                "h-7 w-7 rounded-md flex items-center justify-center shrink-0 ring-1 ring-border",
                visual.colored ? "bg-background" : "bg-muted",
                visual.tint,
              )}
            >
              <Icon icon={visual.icon} className="h-4 w-4" />
            </div>
            <span className="text-[12px] font-medium truncate">
              {visual.label}
            </span>
          </div>
        </TableCell>
        <TableCell>
          <div className="flex flex-col min-w-0">
            <div className="flex items-center gap-1.5">
              <span
                className={cn(
                  "h-1.5 w-1.5 rounded-full shrink-0",
                  hook.enabled ? "bg-emerald-500" : "bg-muted-foreground/40",
                )}
                title={hook.enabled ? "Enabled" : "Paused"}
              />
              <span className="font-semibold text-sm truncate">
                {hook.name}
              </span>
            </div>
            <code
              className="text-[11px] font-mono text-muted-foreground/90 mt-0.5 truncate block max-w-[36ch]"
              title={hook.url}
            >
              {hook.url}
            </code>
          </div>
        </TableCell>
        <TableCell>
          <div className="flex flex-wrap gap-1 max-w-[28ch]">
            {(hook.events ?? []).length === 0 ? (
              <span className="inline-flex items-center h-[18px] px-1.5 rounded bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-1 ring-violet-500/30 text-[10px] font-mono uppercase tracking-wider">
                all events
              </span>
            ) : (
              hook.events!.map((e) => (
                <span
                  key={e}
                  className="inline-flex items-center h-[18px] px-1.5 rounded bg-muted text-[10px] font-mono text-muted-foreground ring-1 ring-border"
                >
                  {e}
                </span>
              ))
            )}
          </div>
        </TableCell>
        <TableCell>
          {!hook.enabled ? (
            <StatusPill tone="muted">paused</StatusPill>
          ) : !hasTried ? (
            <StatusPill tone="muted">untested</StatusPill>
          ) : healthy ? (
            <StatusPill
              tone="emerald"
              title={`${status} — ${formatDistanceToNow(new Date(hook.last_delivery_at!), { addSuffix: true })}`}
            >
              {formatDistanceToNow(new Date(hook.last_delivery_at!), {
                addSuffix: true,
              })}
            </StatusPill>
          ) : (
            <StatusPill
              tone="rose"
              title={hook.last_delivery_error ?? `HTTP ${status}`}
            >
              HTTP {status || "err"}
            </StatusPill>
          )}
        </TableCell>
        <TableCell className="text-right">
          <div className="inline-flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => testMutation.mutate()}
              disabled={testMutation.isPending}
              className="gap-1.5"
            >
              <Icon icon={icons.rocket} className="h-3.5 w-3.5" />
              Test
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={onEdit}
              className="gap-1.5"
            >
              <Icon icon={icons.edit} className="h-3.5 w-3.5" />
              Edit
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setConfirmDelete(true)}
              className="text-destructive hover:text-destructive"
            >
              <Icon icon={icons.delete} className="h-3.5 w-3.5" />
            </Button>
          </div>
        </TableCell>
      </TableRow>

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this subscription?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-mono">{hook.name}</span> ({hook.url}) will
              stop receiving events immediately. In-flight deliveries finish
              their retry budget.
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

function StatusPill({
  tone,
  children,
  title,
}: {
  tone: "emerald" | "rose" | "muted";
  children: React.ReactNode;
  title?: string;
}) {
  const cls =
    tone === "emerald"
      ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-emerald-500/30"
      : tone === "rose"
        ? "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30"
        : "bg-muted text-muted-foreground ring-border";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider whitespace-nowrap",
        cls,
      )}
      title={title}
    >
      {tone !== "muted" && (
        <span
          className={cn(
            "h-1.5 w-1.5 rounded-full",
            tone === "emerald" ? "bg-emerald-500" : "bg-rose-500",
          )}
        />
      )}
      {children}
    </span>
  );
}
