import { Pencil } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import type { AdminModel } from "@/types/api";

interface EditModelButtonProps {
  model: AdminModel;
  trigger?: React.ReactNode;
}

export default function EditModelButton({ model, trigger }: EditModelButtonProps) {
  const navigate = useNavigate();

  if (model.source !== "user") return null;

  return (
    <span onClick={() => navigate(`/models/edit/${model.id}`)}>
      {trigger || (
        <Button variant="outline" size="sm" className="gap-2">
          <Pencil className="h-3 w-3" />
          Edit
        </Button>
      )}
    </span>
  );
}
