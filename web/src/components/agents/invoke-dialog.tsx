import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { invokeAgent } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";

type Props = {
  slug: string | null;
  onClose: () => void;
};

export function InvokeDialog({ slug, onClose }: Props) {
  const { toast } = useToast();
  const [input, setInput] = useState("");
  const [output, setOutput] = useState<string | null>(null);

  const invokeMut = useMutation({
    mutationFn: () => invokeAgent(slug!, input),
    onSuccess: (res: any) => {
      setOutput(
        typeof res.output === "string"
          ? res.output
          : JSON.stringify(res.output, null, 2),
      );
    },
    onError: (e: any) => {
      toast({
        title: "Invoke failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  function handleOpenChange(open: boolean) {
    if (!open) {
      setInput("");
      setOutput(null);
      onClose();
    }
  }

  return (
    <Dialog open={!!slug} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Invoke agent</DialogTitle>
          <DialogDescription className="font-mono text-xs">
            {slug}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <label className="text-xs font-mono uppercase text-muted-foreground">
              Input
            </label>
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              rows={4}
              placeholder="Enter text to send to the agent"
            />
          </div>
          {output !== null && (
            <div>
              <label className="text-xs font-mono uppercase text-muted-foreground">
                Output
              </label>
              <pre className="max-h-64 overflow-auto rounded-md border bg-muted/30 p-3 text-xs">
                {output}
              </pre>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
          <Button
            onClick={() => invokeMut.mutate()}
            disabled={!input.trim() || invokeMut.isPending}
          >
            {invokeMut.isPending ? "Invoking…" : "Invoke"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
