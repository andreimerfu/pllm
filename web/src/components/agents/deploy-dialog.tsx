import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { deployAgent, listRegistryAgents, listUseCases } from "@/lib/api";
import type { RegistryAgent, UseCase } from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useToast } from "@/hooks/use-toast";

type Props = {
  open: boolean;
  onClose: () => void;
};

export function DeployDialog({ open, onClose }: Props) {
  const { toast } = useToast();
  const qc = useQueryClient();

  const [agentKey, setAgentKey] = useState<string>(""); // "<name>::<version>"
  const [useCaseId, setUseCaseId] = useState<string>("");
  const [replicas, setReplicas] = useState<number>(1);

  const { data: agentsRes, isLoading: agentsLoading } = useQuery({
    queryKey: ["registry-agents", "for-deploy"],
    queryFn: () => listRegistryAgents({ latest: true }),
    enabled: open,
  });
  // RegistryListResponse<RegistryAgent> exposes results under `agents`
  const agents: RegistryAgent[] = agentsRes?.agents ?? [];

  const { data: ucRes, isLoading: ucLoading } = useQuery({
    queryKey: ["use-cases", "for-deploy"],
    queryFn: () => listUseCases(),
    enabled: open,
  });
  const useCases: UseCase[] = ucRes?.use_cases ?? [];

  const selectedAgent: RegistryAgent | undefined = (() => {
    if (!agentKey) return undefined;
    const [name, version] = agentKey.split("::");
    return agents.find((a) => a.name === name && a.version === version);
  })();

  const deployMut = useMutation({
    mutationFn: () => {
      if (!selectedAgent) throw new Error("pick an agent");
      if (!useCaseId) throw new Error("pick a use case");
      return deployAgent({
        agent_id: selectedAgent.id,
        use_case_id: useCaseId,
        replicas,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: "Agent deployed", description: "Pod is starting up." });
      reset();
      onClose();
    },
    onError: (e: any) => {
      toast({
        title: "Deploy failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  function reset() {
    setAgentKey("");
    setUseCaseId("");
    setReplicas(1);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) {
          reset();
          onClose();
        }
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Deploy agent</DialogTitle>
          <DialogDescription>
            Pick a published agent and the use case it runs under. pllm
            provisions a scoped key and applies the workload.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-xs font-mono uppercase text-muted-foreground">
              Agent
            </label>
            <select
              className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              value={agentKey}
              onChange={(e) => setAgentKey(e.target.value)}
              disabled={agentsLoading}
            >
              <option value="">
                {agentsLoading
                  ? "Loading…"
                  : agents.length === 0
                    ? "No published agents"
                    : "Select an agent"}
              </option>
              {agents.map((a) => (
                <option
                  key={`${a.name}::${a.version}`}
                  value={`${a.name}::${a.version}`}
                >
                  {a.name} v{a.version}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-mono uppercase text-muted-foreground">
              Use case
            </label>
            <select
              className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              value={useCaseId}
              onChange={(e) => setUseCaseId(e.target.value)}
              disabled={ucLoading}
            >
              <option value="">
                {ucLoading
                  ? "Loading…"
                  : useCases.length === 0
                    ? "No use cases"
                    : "Select a use case"}
              </option>
              {useCases.map((uc) => (
                <option key={uc.id} value={uc.id}>
                  {uc.name}
                  {uc.slug ? ` (${uc.slug})` : ""}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-mono uppercase text-muted-foreground">
              Replicas
            </label>
            <input
              type="number"
              min={1}
              max={5}
              value={replicas}
              onChange={(e) =>
                setReplicas(
                  Math.max(1, Math.min(5, Number(e.target.value) || 1)),
                )
              }
              className="w-24 rounded-md border bg-background px-3 py-2 text-sm"
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => {
              reset();
              onClose();
            }}
          >
            Cancel
          </Button>
          <Button
            onClick={() => deployMut.mutate()}
            disabled={!selectedAgent || !useCaseId || deployMut.isPending}
          >
            {deployMut.isPending ? "Deploying…" : "Deploy"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
