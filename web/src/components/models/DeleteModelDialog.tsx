import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { useToast } from "@/hooks/use-toast";
import { deleteModel } from "@/lib/api";

interface DeleteModelDialogProps {
  modelId: string;
  modelName: string;
  trigger?: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export default function DeleteModelDialog({ modelId, modelName, trigger, open: controlledOpen, onOpenChange }: DeleteModelDialogProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = onOpenChange ?? setInternalOpen;
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const mutation = useMutation({
    mutationFn: () => deleteModel(modelId),
    onSuccess: () => {
      toast({ title: "Model deleted", description: `Model "${modelName}" has been deleted.` });
      queryClient.invalidateQueries({ queryKey: ["models"] });
      queryClient.invalidateQueries({ queryKey: ["admin-models"] });
      setOpen(false);
    },
    onError: (error: any) => {
      const message = error.response?.data?.error || error.message || "Failed to delete model";
      toast({ title: "Error", description: message, variant: "destructive" });
    },
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {trigger && (
        <DialogTrigger asChild>
          {trigger}
        </DialogTrigger>
      )}
      {!trigger && controlledOpen === undefined && (
        <DialogTrigger asChild>
          <Button variant="destructive" size="sm" className="gap-2">
            <Trash2 className="h-3 w-3" />
            Delete
          </Button>
        </DialogTrigger>
      )}
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle>Delete Model</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete <span className="font-semibold">{modelName}</span>?
            This action cannot be undone. The model will be removed from the database and the active
            registry.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={() => mutation.mutate()}
            disabled={mutation.isPending}
          >
            {mutation.isPending ? "Deleting..." : "Delete Model"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
