import { Plus } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";

export default function AddModelButton() {
  const navigate = useNavigate();

  return (
    <Button className="gap-2" onClick={() => navigate("/models/new")}>
      <Plus className="h-4 w-4" />
      Add Model
    </Button>
  );
}
