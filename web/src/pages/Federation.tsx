import { useMemo, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import {
  createFederationPeer,
  deleteFederationPeer,
  listFederationPeers,
  syncFederationPeer,
  updateFederationPeer,
} from "@/lib/api";
import type {
  FederationPeer,
  FederationPeerUpsertRequest,
  FederationSyncMode,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { PageShell, StatTile } from "@/components/common/page-shell";
import { EmptyState } from "@/components/common/EmptyState";
import { useToast } from "@/hooks/use-toast";

// Federation page — manage peer-to-peer registry sync.
//
// Each peer is one upstream pllm registry this instance pulls from. The
// runner pulls on `sync_interval_minutes`; operators can also click
// "Sync now" to force an immediate run. Synced artifacts appear in the
// catalog with their `source_peer_id` badge and are locked from local
// edits — only the source registry can modify them.

const SYNC_MODES: { value: FederationSyncMode; label: string; hint: string }[] = [
  { value: "all", label: "All", hint: "Pull every active artifact" },
  { value: "whitelist", label: "Whitelist", hint: "Only listed names per kind" },
  { value: "tag_filter", label: "Tag filter", hint: "Match metadata substring" },
];

function modeLabel(m: FederationSyncMode): string {
  return SYNC_MODES.find((x) => x.value === m)?.label ?? m;
}

function statusTone(status?: string): "emerald" | "amber" | "rose" | "default" {
  switch (status) {
    case "success":
      return "emerald";
    case "partial":
      return "amber";
    case "failed":
      return "rose";
    default:
      return "default";
  }
}

function emptyForm(): FederationPeerUpsertRequest {
  return {
    peer_id: "",
    name: "",
    endpoint: "",
    federation_token: "",
    expected_client_id: "",
    sync_mode: "all",
    whitelist_servers: [],
    whitelist_agents: [],
    whitelist_skills: [],
    whitelist_prompts: [],
    tag_filters: [],
    sync_interval_minutes: 60,
    enabled: true,
  };
}

export default function Federation() {
  const qc = useQueryClient();
  const { toast } = useToast();

  const peersQ = useQuery({
    queryKey: ["federation", "peers"],
    queryFn: listFederationPeers,
    refetchInterval: 15_000,
  });

  const peers = peersQ.data?.peers ?? [];

  const stats = useMemo(() => {
    const enabled = peers.filter((p) => p.enabled).length;
    const lastOk = peers.filter((p) => p.last_sync_status === "success").length;
    const failing = peers.filter((p) => p.last_sync_status === "failed").length;
    return { total: peers.length, enabled, lastOk, failing };
  }, [peers]);

  const [editing, setEditing] = useState<FederationPeer | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<FederationPeer | null>(null);

  const syncMu = useMutation({
    mutationFn: (id: string) => syncFederationPeer(id),
    onSuccess: (res) => {
      const r = res.result;
      toast({
        title: "Sync complete",
        description: `${r.servers.saved} servers · ${r.agents.saved} agents · ${r.skills.saved} skills · ${r.prompts.saved} prompts${r.orphaned ? ` · ${r.orphaned} orphaned` : ""}`,
      });
      qc.invalidateQueries({ queryKey: ["federation", "peers"] });
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : "Sync failed";
      toast({ title: "Sync failed", description: msg, variant: "destructive" });
    },
  });

  const delMu = useMutation({
    mutationFn: (id: string) => deleteFederationPeer(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["federation", "peers"] });
      setPendingDelete(null);
      toast({ title: "Peer deleted" });
    },
  });

  return (
    <PageShell
      title="Federation"
      description="Peer pllm registries this instance pulls from. Synced artifacts are read-only on this side."
      actions={
        <Button onClick={() => setShowCreate(true)}>
          <Icon icon={icons.add} className="mr-2 size-4" />
          Add peer
        </Button>
      }
      stats={
        <>
          <StatTile label="Total" value={stats.total} icon={<Icon icon={icons.federation} />} />
          <StatTile label="Enabled" value={stats.enabled} tone="emerald" />
          <StatTile label="Last sync OK" value={stats.lastOk} tone="emerald" />
          <StatTile
            label="Failing"
            value={stats.failing}
            tone={stats.failing > 0 ? "rose" : "default"}
          />
        </>
      }
    >
      {peers.length === 0 && !peersQ.isLoading ? (
        <EmptyState
          icon={icons.federation}
          title="No federation peers yet"
          description="Add an upstream pllm registry to pull servers, agents, skills, and prompts on a schedule."
          action={
            <Button onClick={() => setShowCreate(true)}>
              <Icon icon={icons.add} className="mr-2 size-4" />
              Add peer
            </Button>
          }
        />
      ) : (
        <div className="space-y-2">
          {peers.map((p) => (
            <PeerRow
              key={p.id}
              peer={p}
              onEdit={() => setEditing(p)}
              onSync={() => syncMu.mutate(p.id)}
              onDelete={() => setPendingDelete(p)}
              syncing={syncMu.isPending && syncMu.variables === p.id}
            />
          ))}
        </div>
      )}

      <PeerDialog
        open={showCreate}
        peer={null}
        onOpenChange={(v) => setShowCreate(v)}
        onSaved={() => {
          setShowCreate(false);
          qc.invalidateQueries({ queryKey: ["federation", "peers"] });
        }}
      />

      <PeerDialog
        open={editing !== null}
        peer={editing}
        onOpenChange={(v) => !v && setEditing(null)}
        onSaved={() => {
          setEditing(null);
          qc.invalidateQueries({ queryKey: ["federation", "peers"] });
        }}
      />

      <AlertDialog
        open={pendingDelete !== null}
        onOpenChange={(v) => !v && setPendingDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete federation peer?</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingDelete?.name} ({pendingDelete?.peer_id}) — already synced
              artifacts stay in the catalog and remain marked with their source
              peer id. Re-adding the same peer_id later will re-adopt them.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => pendingDelete && delMu.mutate(pendingDelete.id)}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}

function PeerRow({
  peer,
  onEdit,
  onSync,
  onDelete,
  syncing,
}: {
  peer: FederationPeer;
  onEdit: () => void;
  onSync: () => void;
  onDelete: () => void;
  syncing: boolean;
}) {
  const tone = statusTone(peer.last_sync_status);
  return (
    <div className="flex items-center gap-4 rounded-lg border border-border/70 bg-card p-4">
      <div
        className={`size-2.5 shrink-0 rounded-full ${
          peer.enabled
            ? tone === "rose"
              ? "bg-rose-500"
              : tone === "amber"
                ? "bg-amber-500"
                : "bg-emerald-500"
            : "bg-muted-foreground/40"
        }`}
        aria-hidden
      />
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex items-center gap-2">
          <span className="font-semibold truncate">{peer.name}</span>
          <Badge variant="secondary" className="font-mono text-[10px]">
            {peer.peer_id}
          </Badge>
          <Badge variant="outline" className="text-[10px]">
            {modeLabel(peer.sync_mode)}
          </Badge>
          {!peer.enabled && (
            <Badge variant="outline" className="text-[10px] text-muted-foreground">
              disabled
            </Badge>
          )}
        </div>
        <div className="text-[12px] text-muted-foreground truncate">
          {peer.endpoint}
        </div>
        <div className="text-[12px] text-muted-foreground">
          {peer.last_sync_at ? (
            <>
              Last sync{" "}
              {formatDistanceToNow(new Date(peer.last_sync_at), { addSuffix: true })}
              {peer.last_sync_summary ? ` · ${peer.last_sync_summary}` : ""}
            </>
          ) : (
            <>Never synced — click Sync to pull now</>
          )}
          {peer.last_sync_error && (
            <span className="ml-2 text-rose-500">· {peer.last_sync_error}</span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <Button
          variant="outline"
          size="sm"
          onClick={onSync}
          disabled={!peer.enabled || syncing}
        >
          <Icon
            icon={icons.syncSquare}
            className={`mr-1.5 size-4 ${syncing ? "animate-spin" : ""}`}
          />
          {syncing ? "Syncing…" : "Sync now"}
        </Button>
        <Button variant="ghost" size="sm" onClick={onEdit}>
          <Icon icon={icons.edit} className="mr-1.5 size-4" />
          Edit
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={onDelete}
          className="text-rose-500 hover:text-rose-600"
        >
          <Icon icon={icons.delete} className="size-4" />
        </Button>
      </div>
    </div>
  );
}

function PeerDialog({
  open,
  peer,
  onOpenChange,
  onSaved,
}: {
  open: boolean;
  peer: FederationPeer | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}) {
  const { toast } = useToast();
  const [form, setForm] = useState<FederationPeerUpsertRequest>(() =>
    peer
      ? {
          peer_id: peer.peer_id,
          name: peer.name,
          endpoint: peer.endpoint,
          federation_token: "",
          expected_client_id: peer.expected_client_id ?? "",
          sync_mode: peer.sync_mode,
          whitelist_servers: peer.whitelist_servers ?? [],
          whitelist_agents: peer.whitelist_agents ?? [],
          whitelist_skills: peer.whitelist_skills ?? [],
          whitelist_prompts: peer.whitelist_prompts ?? [],
          tag_filters: peer.tag_filters ?? [],
          sync_interval_minutes: peer.sync_interval_minutes,
          enabled: peer.enabled,
        }
      : emptyForm(),
  );

  // Reset when the bound peer changes.
  useMemo(() => {
    setForm(
      peer
        ? {
            peer_id: peer.peer_id,
            name: peer.name,
            endpoint: peer.endpoint,
            federation_token: "",
            expected_client_id: peer.expected_client_id ?? "",
            sync_mode: peer.sync_mode,
            whitelist_servers: peer.whitelist_servers ?? [],
            whitelist_agents: peer.whitelist_agents ?? [],
            whitelist_skills: peer.whitelist_skills ?? [],
            whitelist_prompts: peer.whitelist_prompts ?? [],
            tag_filters: peer.tag_filters ?? [],
            sync_interval_minutes: peer.sync_interval_minutes,
            enabled: peer.enabled,
          }
        : emptyForm(),
    );
  }, [peer]);

  const mu = useMutation({
    mutationFn: async () => {
      if (peer) {
        return updateFederationPeer(peer.id, form);
      }
      return createFederationPeer(form);
    },
    onSuccess: () => {
      toast({ title: peer ? "Peer updated" : "Peer created" });
      onSaved();
    },
    onError: (e: unknown) => {
      const msg = e instanceof Error ? e.message : "Failed to save peer";
      toast({ title: "Save failed", description: msg, variant: "destructive" });
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{peer ? "Edit federation peer" : "Add federation peer"}</DialogTitle>
          <DialogDescription>
            {peer
              ? "Update connection details, sync schedule, and filters for this peer."
              : "Configure an upstream pllm registry to pull from."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <Field
              label="Peer ID"
              hint="Stable slug used to tag synced rows"
              required
            >
              <Input
                value={form.peer_id}
                onChange={(e) => setForm({ ...form, peer_id: e.target.value })}
                placeholder="lob-a"
                disabled={peer !== null}
              />
            </Field>
            <Field label="Display name" required>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="LOB A pllm"
              />
            </Field>
          </div>

          <Field label="Endpoint" hint="Base URL of the peer pllm" required>
            <Input
              value={form.endpoint}
              onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
              placeholder="https://pllm-lob-a.example.com"
            />
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <Field
              label="Federation token"
              hint={peer ? "Leave blank to keep existing" : "Bearer token the peer accepts"}
            >
              <Input
                type="password"
                value={form.federation_token ?? ""}
                onChange={(e) => setForm({ ...form, federation_token: e.target.value })}
                placeholder={peer ? "•••" : "shared-secret"}
              />
            </Field>
            <Field label="Client ID" hint="Optional — sent as X-PLLM-Federation-Client">
              <Input
                value={form.expected_client_id ?? ""}
                onChange={(e) =>
                  setForm({ ...form, expected_client_id: e.target.value })
                }
                placeholder="this-pllm"
              />
            </Field>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Field label="Sync mode">
              <Select
                value={form.sync_mode}
                onValueChange={(v) =>
                  setForm({ ...form, sync_mode: v as FederationSyncMode })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SYNC_MODES.map((m) => (
                    <SelectItem value={m.value} key={m.value}>
                      <div className="flex flex-col">
                        <span>{m.label}</span>
                        <span className="text-[11px] text-muted-foreground">
                          {m.hint}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label="Interval (minutes)">
              <Input
                type="number"
                min={1}
                value={form.sync_interval_minutes ?? 60}
                onChange={(e) =>
                  setForm({
                    ...form,
                    sync_interval_minutes: parseInt(e.target.value, 10) || 60,
                  })
                }
              />
            </Field>
          </div>

          {form.sync_mode === "whitelist" && (
            <div className="grid grid-cols-2 gap-3">
              <ListField
                label="Servers (one per line)"
                value={form.whitelist_servers ?? []}
                onChange={(v) => setForm({ ...form, whitelist_servers: v })}
              />
              <ListField
                label="Agents (one per line)"
                value={form.whitelist_agents ?? []}
                onChange={(v) => setForm({ ...form, whitelist_agents: v })}
              />
              <ListField
                label="Skills (one per line)"
                value={form.whitelist_skills ?? []}
                onChange={(v) => setForm({ ...form, whitelist_skills: v })}
              />
              <ListField
                label="Prompts (one per line)"
                value={form.whitelist_prompts ?? []}
                onChange={(v) => setForm({ ...form, whitelist_prompts: v })}
              />
            </div>
          )}

          {form.sync_mode === "tag_filter" && (
            <ListField
              label="Tag substrings (one per line)"
              hint="Each row matched as case-insensitive substring against the artifact's metadata JSON"
              value={form.tag_filters ?? []}
              onChange={(v) => setForm({ ...form, tag_filters: v })}
            />
          )}

          <div className="flex items-center gap-2">
            <input
              id="fed-enabled"
              type="checkbox"
              checked={form.enabled ?? true}
              onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              className="size-4"
            />
            <Label htmlFor="fed-enabled" className="text-sm font-normal">
              Enabled — runner will pull on schedule
            </Label>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={() => mu.mutate()} disabled={mu.isPending}>
            {mu.isPending ? "Saving…" : peer ? "Save changes" : "Add peer"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Field({
  label,
  hint,
  required,
  children,
}: {
  label: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <Label className="text-[12px]">
        {label}
        {required && <span className="text-rose-500 ml-0.5">*</span>}
      </Label>
      {children}
      {hint && <p className="text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  );
}

function ListField({
  label,
  hint,
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  value: string[];
  onChange: (v: string[]) => void;
}) {
  const text = value.join("\n");
  return (
    <div className="space-y-1">
      <Label className="text-[12px]">{label}</Label>
      <Textarea
        rows={3}
        value={text}
        onChange={(e) =>
          onChange(
            e.target.value
              .split("\n")
              .map((s) => s.trim())
              .filter(Boolean),
          )
        }
      />
      {hint && <p className="text-[11px] text-muted-foreground">{hint}</p>}
    </div>
  );
}
