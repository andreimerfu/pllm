import { useState } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { getProviderLogo } from "@/lib/provider-logos";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/common/PageHeader";
import { EmptyState } from "@/components/common/EmptyState";
import { LoadingState } from "@/components/common/LoadingState";
import { useToast } from "@/hooks/use-toast";
import {
  useProviderProfiles,
  useCreateProviderProfile,
  useUpdateProviderProfile,
  useDeleteProviderProfile,
} from "@/hooks/useProviders";
import { formatRelativeTime } from "@/lib/date-utils";
import type { ProviderProfile } from "@/types/api";

type FieldDef = {
  key: string;
  label: string;
  placeholder: string;
  required: boolean;
  secret?: boolean;
  authMode?: string;
};

// Provider type definitions with credential field schemas
const PROVIDER_TYPES: Array<{
  value: string;
  label: string;
  icon: string;
  color: string;
  bgColor: string;
  borderColor: string;
  authToggle?: boolean;
  fields: FieldDef[];
}> = [
  {
    value: "openai",
    label: "OpenAI",
    icon: "logos:openai-icon",
    color: "text-emerald-600 dark:text-emerald-400",
    bgColor: "bg-emerald-50 dark:bg-emerald-950/30",
    borderColor: "border-emerald-200 dark:border-emerald-800",
    fields: [
      { key: "api_key", label: "API Key", placeholder: "sk-... or ${OPENAI_API_KEY}", required: false, secret: true },
      { key: "base_url", label: "Base URL", placeholder: "https://api.openai.com/v1 (optional)", required: false },
    ],
  },
  {
    value: "anthropic",
    label: "Anthropic",
    icon: "simple-icons:anthropic",
    color: "text-orange-600 dark:text-orange-400",
    bgColor: "bg-orange-50 dark:bg-orange-950/30",
    borderColor: "border-orange-200 dark:border-orange-800",
    authToggle: true,
    fields: [
      { key: "api_key", label: "API Key", placeholder: "sk-ant-... or ${ANTHROPIC_API_KEY}", required: false, secret: true, authMode: "api_key" },
      { key: "oauth_token", label: "OAuth Token", placeholder: "Bearer token from Claude Max", required: false, secret: true, authMode: "oauth_token" },
      { key: "base_url", label: "Base URL", placeholder: "https://api.anthropic.com (optional)", required: false },
    ],
  },
  {
    value: "azure",
    label: "Azure OpenAI",
    icon: "logos:microsoft-azure",
    color: "text-blue-700 dark:text-blue-300",
    bgColor: "bg-blue-50 dark:bg-blue-950/30",
    borderColor: "border-blue-200 dark:border-blue-800",
    fields: [
      { key: "api_key", label: "API Key", placeholder: "${AZURE_API_KEY}", required: false, secret: true },
      { key: "azure_endpoint", label: "Azure Endpoint", placeholder: "https://your-resource.openai.azure.com", required: true },
      { key: "azure_deployment", label: "Deployment Name", placeholder: "my-gpt4-deployment", required: false },
      { key: "api_version", label: "API Version", placeholder: "2024-02-15-preview", required: false },
    ],
  },
  {
    value: "bedrock",
    label: "AWS Bedrock",
    icon: "logos:aws",
    color: "text-yellow-600 dark:text-yellow-400",
    bgColor: "bg-yellow-50 dark:bg-yellow-950/30",
    borderColor: "border-yellow-200 dark:border-yellow-800",
    fields: [
      { key: "aws_access_key_id", label: "AWS Access Key ID", placeholder: "AKIA...", required: true, secret: true },
      { key: "aws_secret_access_key", label: "AWS Secret Key", placeholder: "Your secret key", required: true, secret: true },
      { key: "aws_region", label: "Region", placeholder: "us-east-1", required: false },
    ],
  },
  {
    value: "vertex",
    label: "Google Vertex AI",
    icon: "logos:google-cloud",
    color: "text-red-600 dark:text-red-400",
    bgColor: "bg-red-50 dark:bg-red-950/30",
    borderColor: "border-red-200 dark:border-red-800",
    fields: [
      { key: "api_key", label: "API Key", placeholder: "${GOOGLE_API_KEY}", required: false, secret: true },
      { key: "vertex_project", label: "Project ID", placeholder: "my-gcp-project", required: false },
      { key: "vertex_location", label: "Location", placeholder: "us-central1", required: false },
    ],
  },
  {
    value: "openrouter",
    label: "OpenRouter",
    icon: "solar:global-linear",
    color: "text-purple-600 dark:text-purple-400",
    bgColor: "bg-purple-50 dark:bg-purple-950/30",
    borderColor: "border-purple-200 dark:border-purple-800",
    fields: [
      { key: "api_key", label: "API Key", placeholder: "sk-or-... or ${OPENROUTER_API_KEY}", required: true, secret: true },
      { key: "base_url", label: "Base URL", placeholder: "https://openrouter.ai/api/v1 (optional)", required: false },
    ],
  },
];

export default function Providers() {
  const { toast } = useToast();
  const { data: profiles, isLoading } = useProviderProfiles();
  const createMutation = useCreateProviderProfile();
  const updateMutation = useUpdateProviderProfile();
  const deleteMutation = useDeleteProviderProfile();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<ProviderProfile | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ProviderProfile | null>(null);

  // Form state
  const [selectedType, setSelectedType] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [configValues, setConfigValues] = useState<Record<string, string>>({});
  const [anthropicAuthMode, setAnthropicAuthMode] = useState<"api_key" | "oauth_token">("api_key");

  const providerList: ProviderProfile[] = Array.isArray(profiles) ? profiles : [];

  function openCreateDialog() {
    setEditingProfile(null);
    setSelectedType(null);
    setName("");
    setConfigValues({});
    setAnthropicAuthMode("api_key");
    setDialogOpen(true);
  }

  function openEditDialog(profile: ProviderProfile) {
    setEditingProfile(profile);
    setSelectedType(profile.type);
    setName(profile.name);
    setConfigValues(profile.config ? { ...profile.config } as Record<string, string> : {});
    if (profile.config?.oauth_token && !profile.config?.api_key) {
      setAnthropicAuthMode("oauth_token");
    } else {
      setAnthropicAuthMode("api_key");
    }
    setDialogOpen(true);
  }

  function closeDialog() {
    setDialogOpen(false);
    setEditingProfile(null);
    setSelectedType(null);
  }

  async function handleSave() {
    if (!selectedType || !name.trim()) return;

    // Build config, filtering out empty values
    const config: Record<string, string> = {};
    for (const [k, v] of Object.entries(configValues)) {
      if (v && v.trim()) config[k] = v.trim();
    }

    try {
      if (editingProfile) {
        await updateMutation.mutateAsync({
          id: editingProfile.id,
          data: { name: name.trim(), type: selectedType, config },
        });
        toast({ title: "Provider updated", description: `"${name.trim()}" has been saved.` });
      } else {
        await createMutation.mutateAsync({ name: name.trim(), type: selectedType, config });
        toast({ title: "Provider created", description: `"${name.trim()}" has been saved.` });
      }
      closeDialog();
    } catch (error: any) {
      toast({
        title: "Error",
        description: error?.message || "Failed to save provider profile",
        variant: "destructive",
      });
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast({ title: "Provider deleted", description: `"${deleteTarget.name}" has been removed.` });
      setDeleteTarget(null);
    } catch (error: any) {
      toast({
        title: "Error",
        description: error?.message || "Failed to delete provider",
        variant: "destructive",
      });
    }
  }

  const selectedProviderDef = PROVIDER_TYPES.find((p) => p.value === selectedType);

  // Get visible fields based on provider type and auth mode
  function getVisibleFields(): FieldDef[] {
    if (!selectedProviderDef) return [];
    return selectedProviderDef.fields.filter((f) => {
      if (!f.authMode) return true;
      if (selectedType === "anthropic") return f.authMode === anthropicAuthMode;
      return true;
    });
  }

  if (isLoading) {
    return <LoadingState text="Loading providers..." />;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Providers"
        description="Manage saved LLM provider credentials"
        actions={
          <Button onClick={openCreateDialog} className="bg-teal-600 hover:bg-teal-700 text-white">
            <Icon icon={icons.add} className="mr-2 h-4 w-4" />
            Add Provider
          </Button>
        }
      />

      {providerList.length === 0 ? (
        <EmptyState
          icon={icons.globe}
          title="No providers saved"
          description="Save your LLM provider credentials for easy reuse when adding models."
          action={
            <Button onClick={openCreateDialog} className="bg-teal-600 hover:bg-teal-700 text-white">
              <Icon icon={icons.add} className="mr-2 h-4 w-4" />
              Add First Provider
            </Button>
          }
        />
      ) : (
        <div className="bg-card border rounded-lg">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Models</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {providerList.map((profile) => (
                <TableRow key={profile.id}>
                  <TableCell className="font-medium">{profile.name}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Icon icon={getProviderLogo(profile.type)} className="h-4 w-4" />
                      <span className="capitalize">{profile.type}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary" className="font-mono text-xs">
                      {profile.model_count ?? 0}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {formatRelativeTime(profile.created_at)}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button variant="ghost" size="icon" onClick={() => openEditDialog(profile)}>
                        <Icon icon={icons.edit} className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive"
                        onClick={() => setDeleteTarget(profile)}
                      >
                        <Icon icon={icons.delete} className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={(open) => { if (!open) closeDialog(); }}>
        <DialogContent className="sm:max-w-[560px] max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingProfile ? "Edit Provider" : "Add Provider"}</DialogTitle>
            <DialogDescription>
              {editingProfile
                ? "Update the provider credentials and settings."
                : "Save LLM provider credentials for reuse across models."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-6 py-2">
            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor="provider-name">Name</Label>
              <Input
                id="provider-name"
                placeholder="e.g. Production OpenAI, Dev Anthropic"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>

            {/* Provider type selector */}
            <div className="space-y-2">
              <Label>Provider Type</Label>
              <div className="grid grid-cols-3 gap-2">
                {PROVIDER_TYPES.map((pt) => (
                  <Card
                    key={pt.value}
                    className={`cursor-pointer p-3 flex flex-col items-center gap-1.5 transition-all hover:shadow-sm ${
                      selectedType === pt.value
                        ? `ring-2 ring-teal-500 ${pt.bgColor} ${pt.borderColor}`
                        : "hover:bg-muted/50"
                    }`}
                    onClick={() => {
                      setSelectedType(pt.value);
                      setConfigValues({});
                    }}
                  >
                    <Icon icon={pt.icon} className={`h-6 w-6 ${pt.color}`} />
                    <span className="text-xs font-medium text-center">{pt.label}</span>
                  </Card>
                ))}
              </div>
            </div>

            {/* Credential fields — shown after type is selected */}
            {selectedProviderDef && (
              <div className="space-y-4">
                <div className="flex items-center gap-2">
                  <div className="h-px flex-1 bg-border" />
                  <span className="text-xs text-muted-foreground uppercase tracking-wider">Credentials</span>
                  <div className="h-px flex-1 bg-border" />
                </div>

                {/* Anthropic auth mode toggle */}
                {selectedType === "anthropic" && selectedProviderDef.authToggle && (
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      size="sm"
                      variant={anthropicAuthMode === "api_key" ? "default" : "outline"}
                      onClick={() => setAnthropicAuthMode("api_key")}
                      className={anthropicAuthMode === "api_key" ? "bg-teal-600 hover:bg-teal-700" : ""}
                    >
                      API Key
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant={anthropicAuthMode === "oauth_token" ? "default" : "outline"}
                      onClick={() => setAnthropicAuthMode("oauth_token")}
                      className={anthropicAuthMode === "oauth_token" ? "bg-teal-600 hover:bg-teal-700" : ""}
                    >
                      OAuth Token
                    </Button>
                  </div>
                )}

                {getVisibleFields().map((field) => (
                  <div key={field.key} className="space-y-1.5">
                    <Label htmlFor={`field-${field.key}`}>
                      {field.label}
                      {field.required && <span className="text-destructive ml-1">*</span>}
                    </Label>
                    <Input
                      id={`field-${field.key}`}
                      type={field.secret ? "password" : "text"}
                      placeholder={field.placeholder}
                      className={field.secret ? "font-mono" : ""}
                      value={configValues[field.key] || ""}
                      onChange={(e) =>
                        setConfigValues((prev) => ({ ...prev, [field.key]: e.target.value }))
                      }
                    />
                  </div>
                ))}
              </div>
            )}

            {/* Save button */}
            <Button
              onClick={handleSave}
              disabled={
                !selectedType ||
                !name.trim() ||
                createMutation.isPending ||
                updateMutation.isPending
              }
              className="w-full bg-teal-600 hover:bg-teal-700 text-white"
            >
              {createMutation.isPending || updateMutation.isPending ? (
                <>
                  <Icon icon={icons.loader} className="mr-2 h-4 w-4 animate-spin" />
                  Saving...
                </>
              ) : editingProfile ? (
                "Update Provider"
              ) : (
                "Save Provider"
              )}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete provider "{deleteTarget?.name}"?</AlertDialogTitle>
            <AlertDialogDescription>
              {(deleteTarget?.model_count ?? 0) > 0 ? (
                <>
                  This provider is referenced by{" "}
                  <span className="font-semibold">{deleteTarget?.model_count} model(s)</span>.
                  Remove or reassign those models before deleting.
                </>
              ) : (
                "This action cannot be undone. The saved credentials will be permanently removed."
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={(deleteTarget?.model_count ?? 0) > 0 || deleteMutation.isPending}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
