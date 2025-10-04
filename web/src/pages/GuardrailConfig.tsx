import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { 
  Shield, 
  Save, 
  ArrowLeft, 
  TestTube,
  AlertTriangle,
  CheckCircle,
  Settings2,
  Globe,
  Clock,
  Lock,
  Zap
} from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Slider } from "@/components/ui/slider";
import { toast } from "@/hooks/use-toast";

// Types and validation schemas
const guardrailConfigSchema = z.object({
  name: z.string().min(1, "Name is required"),
  description: z.string().optional(),
  provider: z.string().min(1, "Provider is required"),
  enabled: z.boolean().default(true),
  default_on: z.boolean().default(false),
  execution_modes: z.array(z.string()).min(1, "At least one execution mode is required"),
  
  // Provider-specific configurations
  analyzer_url: z.string().url("Must be a valid URL"),
  anonymizer_url: z.string().url("Must be a valid URL").optional(),
  threshold: z.number().min(0).max(1).default(0.7),
  entities: z.array(z.string()).min(1, "At least one entity type is required"),
  anonymize_method: z.string().default("replace"),
  mask_pii: z.boolean().default(true),
  language: z.string().default("en"),
  
  // Advanced settings
  timeout_ms: z.number().min(100).max(30000).default(5000),
  retry_attempts: z.number().min(0).max(5).default(2),
  cache_results: z.boolean().default(true),
  log_level: z.enum(["debug", "info", "warn", "error"]).default("info"),
});

type GuardrailConfig = z.infer<typeof guardrailConfigSchema>;

// Constants
const EXECUTION_MODES = [
  { 
    value: "pre_call", 
    label: "Pre-call", 
    description: "Execute before sending to LLM",
    icon: <ArrowLeft className="h-4 w-4" />
  },
  { 
    value: "post_call", 
    label: "Post-call", 
    description: "Execute after LLM response",
    icon: <CheckCircle className="h-4 w-4" />
  },
  { 
    value: "during_call", 
    label: "During-call", 
    description: "Execute in parallel with LLM",
    icon: <Zap className="h-4 w-4" />
  },
  { 
    value: "logging_only", 
    label: "Logging only", 
    description: "Log violations without blocking",
    icon: <AlertTriangle className="h-4 w-4" />
  }
];

const PII_ENTITIES = [
  { value: "PERSON", label: "Person Names", category: "Identity" },
  { value: "EMAIL_ADDRESS", label: "Email Addresses", category: "Contact" },
  { value: "PHONE_NUMBER", label: "Phone Numbers", category: "Contact" },
  { value: "CREDIT_CARD", label: "Credit Cards", category: "Financial" },
  { value: "SSN", label: "Social Security Numbers", category: "Government" },
  { value: "IP_ADDRESS", label: "IP Addresses", category: "Technical" },
  { value: "US_DRIVER_LICENSE", label: "US Driver Licenses", category: "Government" },
  { value: "US_PASSPORT", label: "US Passports", category: "Government" },
  { value: "US_BANK_NUMBER", label: "US Bank Numbers", category: "Financial" },
  { value: "IBAN_CODE", label: "IBAN Codes", category: "Financial" },
  { value: "MEDICAL_LICENSE", label: "Medical License Numbers", category: "Healthcare" },
  { value: "URL", label: "URLs", category: "Technical" }
];

const ANONYMIZE_METHODS = [
  { value: "replace", label: "Replace", description: "Replace with generic tokens" },
  { value: "mask", label: "Mask", description: "Partially hide content (e.g., ***-**-1234)" },
  { value: "redact", label: "Redact", description: "Remove completely" },
  { value: "encrypt", label: "Encrypt", description: "Encrypt the content" },
  { value: "hash", label: "Hash", description: "Hash the content" }
];

const LOG_LEVELS = [
  { value: "debug", label: "Debug", description: "Detailed debugging information" },
  { value: "info", label: "Info", description: "General information messages" },
  { value: "warn", label: "Warning", description: "Warning messages only" },
  { value: "error", label: "Error", description: "Error messages only" }
];

export default function GuardrailConfig() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [testResult, setTestResult] = useState<any>(null);
  const [isTesting, setIsTesting] = useState(false);

  const isEditing = Boolean(id);
  
  const form = useForm<GuardrailConfig>({
    resolver: zodResolver(guardrailConfigSchema),
    defaultValues: {
      enabled: true,
      default_on: false,
      threshold: 0.7,
      anonymize_method: "replace",
      mask_pii: true,
      language: "en",
      timeout_ms: 5000,
      retry_attempts: 2,
      cache_results: true,
      log_level: "info",
      execution_modes: ["pre_call"],
      entities: ["PERSON", "EMAIL_ADDRESS", "PHONE_NUMBER"]
    }
  });

  const { isLoading } = useQuery({
    queryKey: ["guardrail", id],
    queryFn: async () => {
      if (!id) return null;
      // TODO: Implement API call to get guardrail by ID
      return null;
    },
    enabled: !!id,
  });

  const saveMutation = useMutation({
    mutationFn: async (data: GuardrailConfig) => {
      // TODO: Implement API call to save/update guardrail
      console.log("Saving guardrail:", data);
      return data;
    },
    onSuccess: () => {
      toast({
        title: "Success",
        description: `Guardrail ${isEditing ? 'updated' : 'created'} successfully`,
      });
      queryClient.invalidateQueries({ queryKey: ["guardrails"] });
      navigate("/guardrails");
    },
    onError: (error) => {
      toast({
        title: "Error",
        description: `Failed to ${isEditing ? 'update' : 'create'} guardrail: ${error}`,
        variant: "destructive",
      });
    },
  });

  const testMutation = useMutation({
    mutationFn: async (_data: GuardrailConfig) => {
      setIsTesting(true);
      // TODO: Implement API call to test guardrail configuration
      await new Promise(resolve => setTimeout(resolve, 2000));
      return {
        success: true,
        latency: 245,
        test_input: "Hello, my name is John Doe and my email is john@example.com",
        detected_entities: [
          { entity: "PERSON", text: "John Doe", score: 0.95 },
          { entity: "EMAIL_ADDRESS", text: "john@example.com", score: 0.98 }
        ],
        anonymized_output: "Hello, my name is [PERSON] and my email is [EMAIL_ADDRESS]"
      };
    },
    onSuccess: (result) => {
      setTestResult(result);
      setIsTesting(false);
    },
    onError: (error) => {
      setTestResult({ success: false, error: error.message });
      setIsTesting(false);
    },
  });

  const onSubmit = (data: GuardrailConfig) => {
    saveMutation.mutate(data);
  };

  const onTest = () => {
    const formData = form.getValues();
    testMutation.mutate(formData);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const groupedEntities = PII_ENTITIES.reduce((acc, entity) => {
    if (!acc[entity.category]) {
      acc[entity.category] = [];
    }
    acc[entity.category].push(entity);
    return acc;
  }, {} as Record<string, typeof PII_ENTITIES>);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button 
          variant="outline" 
          size="icon"
          onClick={() => navigate("/guardrails")}
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">
            {isEditing ? "Edit Guardrail" : "Create Guardrail"}
          </h1>
          <p className="text-muted-foreground">
            Configure PII detection and content safety settings
          </p>
        </div>
      </div>

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Main Configuration */}
            <div className="lg:col-span-2 space-y-6">
              {/* Basic Information */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Shield className="h-5 w-5" />
                    Basic Information
                  </CardTitle>
                  <CardDescription>
                    Configure the basic settings for your guardrail
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="name"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Name</FormLabel>
                          <FormControl>
                            <Input placeholder="my-pii-guardrail" {...field} />
                          </FormControl>
                          <FormDescription>
                            Unique identifier for this guardrail
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="provider"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Provider</FormLabel>
                          <Select onValueChange={field.onChange} defaultValue={field.value}>
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue placeholder="Select provider" />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="presidio">Microsoft Presidio</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name="description"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Description</FormLabel>
                        <FormControl>
                          <Textarea 
                            placeholder="Describe the purpose of this guardrail..."
                            className="resize-none"
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          Optional description of what this guardrail does
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="grid grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="enabled"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5">
                            <FormLabel className="text-base">Enable Guardrail</FormLabel>
                            <FormDescription>
                              Whether this guardrail is active
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch checked={field.value} onCheckedChange={field.onChange} />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="default_on"
                      render={({ field }) => (
                        <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                          <div className="space-y-0.5">
                            <FormLabel className="text-base">Default On</FormLabel>
                            <FormDescription>
                              Apply to all requests by default
                            </FormDescription>
                          </div>
                          <FormControl>
                            <Switch checked={field.value} onCheckedChange={field.onChange} />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                  </div>
                </CardContent>
              </Card>

              {/* Execution Modes */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Settings2 className="h-5 w-5" />
                    Execution Modes
                  </CardTitle>
                  <CardDescription>
                    Choose when and how this guardrail should execute
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <FormField
                    control={form.control}
                    name="execution_modes"
                    render={() => (
                      <FormItem>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                          {EXECUTION_MODES.map((mode) => (
                            <FormField
                              key={mode.value}
                              control={form.control}
                              name="execution_modes"
                              render={({ field }) => (
                                <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4 hover:bg-accent/50">
                                  <FormControl>
                                    <Checkbox
                                      checked={field.value?.includes(mode.value)}
                                      onCheckedChange={(checked) => {
                                        return checked
                                          ? field.onChange([...field.value, mode.value])
                                          : field.onChange(field.value?.filter((value) => value !== mode.value));
                                      }}
                                    />
                                  </FormControl>
                                  <div className="space-y-1 leading-none flex-1">
                                    <div className="flex items-center gap-2">
                                      {mode.icon}
                                      <FormLabel className="text-sm font-medium">{mode.label}</FormLabel>
                                    </div>
                                    <FormDescription className="text-xs">
                                      {mode.description}
                                    </FormDescription>
                                  </div>
                                </FormItem>
                              )}
                            />
                          ))}
                        </div>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>

              {/* Provider Configuration */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Globe className="h-5 w-5" />
                    Provider Configuration
                  </CardTitle>
                  <CardDescription>
                    Configure connection to Microsoft Presidio services
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormField
                      control={form.control}
                      name="analyzer_url"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Analyzer URL</FormLabel>
                          <FormControl>
                            <Input placeholder="http://presidio-analyzer:3000" {...field} />
                          </FormControl>
                          <FormDescription>
                            Presidio analyzer service endpoint
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="anonymizer_url"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Anonymizer URL (Optional)</FormLabel>
                          <FormControl>
                            <Input placeholder="http://presidio-anonymizer:3000" {...field} />
                          </FormControl>
                          <FormDescription>
                            Presidio anonymizer service endpoint
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <FormField
                      control={form.control}
                      name="threshold"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Detection Threshold</FormLabel>
                          <FormControl>
                            <div className="space-y-2">
                              <Slider
                                min={0}
                                max={1}
                                step={0.1}
                                value={[field.value]}
                                onValueChange={(value) => field.onChange(value[0])}
                                className="w-full"
                              />
                              <div className="text-center text-sm font-medium">
                                {field.value.toFixed(1)}
                              </div>
                            </div>
                          </FormControl>
                          <FormDescription>
                            Confidence threshold (0.0-1.0)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="language"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Language</FormLabel>
                          <Select onValueChange={field.onChange} defaultValue={field.value}>
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              <SelectItem value="en">English</SelectItem>
                              <SelectItem value="es">Spanish</SelectItem>
                              <SelectItem value="fr">French</SelectItem>
                              <SelectItem value="de">German</SelectItem>
                              <SelectItem value="it">Italian</SelectItem>
                              <SelectItem value="pt">Portuguese</SelectItem>
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="anonymize_method"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Anonymization Method</FormLabel>
                          <Select onValueChange={field.onChange} defaultValue={field.value}>
                            <FormControl>
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              {ANONYMIZE_METHODS.map((method) => (
                                <SelectItem key={method.value} value={method.value}>
                                  {method.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name="mask_pii"
                    render={({ field }) => (
                      <FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
                        <div className="space-y-0.5">
                          <FormLabel className="text-base">
                            <Lock className="inline h-4 w-4 mr-2" />
                            Mask PII in Requests
                          </FormLabel>
                          <FormDescription>
                            Automatically mask detected PII before sending to LLM
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch checked={field.value} onCheckedChange={field.onChange} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>

              {/* PII Entity Configuration */}
              <Card>
                <CardHeader>
                  <CardTitle>PII Entity Detection</CardTitle>
                  <CardDescription>
                    Select which types of personally identifiable information to detect
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <FormField
                    control={form.control}
                    name="entities"
                    render={() => (
                      <FormItem>
                        <Tabs defaultValue={Object.keys(groupedEntities)[0]} className="w-full">
                          <TabsList className="grid w-full grid-cols-5">
                            {Object.keys(groupedEntities).map((category) => (
                              <TabsTrigger key={category} value={category} className="text-xs">
                                {category}
                              </TabsTrigger>
                            ))}
                          </TabsList>
                          {Object.entries(groupedEntities).map(([category, entities]) => (
                            <TabsContent key={category} value={category} className="space-y-4">
                              <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                                {entities.map((entity) => (
                                  <FormField
                                    key={entity.value}
                                    control={form.control}
                                    name="entities"
                                    render={({ field }) => (
                                      <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-3 hover:bg-accent/50">
                                        <FormControl>
                                          <Checkbox
                                            checked={field.value?.includes(entity.value)}
                                            onCheckedChange={(checked) => {
                                              return checked
                                                ? field.onChange([...field.value, entity.value])
                                                : field.onChange(field.value?.filter((value) => value !== entity.value));
                                            }}
                                          />
                                        </FormControl>
                                        <FormLabel className="text-sm font-normal leading-relaxed">
                                          {entity.label}
                                        </FormLabel>
                                      </FormItem>
                                    )}
                                  />
                                ))}
                              </div>
                            </TabsContent>
                          ))}
                        </Tabs>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>
            </div>

            {/* Sidebar - Advanced Settings & Testing */}
            <div className="space-y-6">
              {/* Advanced Settings */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Clock className="h-5 w-5" />
                    Advanced Settings
                  </CardTitle>
                  <CardDescription>
                    Performance and reliability configuration
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <FormField
                    control={form.control}
                    name="timeout_ms"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Timeout (ms)</FormLabel>
                        <FormControl>
                          <Input 
                            type="number" 
                            min="100" 
                            max="30000" 
                            {...field}
                            onChange={(e) => field.onChange(parseInt(e.target.value))}
                          />
                        </FormControl>
                        <FormDescription>
                          Request timeout in milliseconds
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="retry_attempts"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Retry Attempts</FormLabel>
                        <FormControl>
                          <Input 
                            type="number" 
                            min="0" 
                            max="5" 
                            {...field}
                            onChange={(e) => field.onChange(parseInt(e.target.value))}
                          />
                        </FormControl>
                        <FormDescription>
                          Number of retries on failure
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="log_level"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Log Level</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            {LOG_LEVELS.map((level) => (
                              <SelectItem key={level.value} value={level.value}>
                                {level.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          Logging verbosity level
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="cache_results"
                    render={({ field }) => (
                      <FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
                        <div className="space-y-0.5">
                          <FormLabel className="text-sm">Cache Results</FormLabel>
                          <FormDescription className="text-xs">
                            Cache detection results for performance
                          </FormDescription>
                        </div>
                        <FormControl>
                          <Switch checked={field.value} onCheckedChange={field.onChange} />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>

              {/* Test Configuration */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <TestTube className="h-5 w-5" />
                    Test Configuration
                  </CardTitle>
                  <CardDescription>
                    Test your guardrail configuration
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <Button 
                    type="button" 
                    variant="outline" 
                    className="w-full"
                    onClick={onTest}
                    disabled={isTesting}
                  >
                    {isTesting ? (
                      <>
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />
                        Testing...
                      </>
                    ) : (
                      <>
                        <TestTube className="h-4 w-4 mr-2" />
                        Test Configuration
                      </>
                    )}
                  </Button>

                  {testResult && (
                    <Alert className={testResult.success ? "border-green-200 bg-green-50" : "border-red-200 bg-red-50"}>
                      <div className="flex items-center gap-2">
                        {testResult.success ? (
                          <CheckCircle className="h-4 w-4 text-green-600" />
                        ) : (
                          <AlertTriangle className="h-4 w-4 text-red-600" />
                        )}
                        <span className="font-medium">
                          {testResult.success ? "Test Successful" : "Test Failed"}
                        </span>
                      </div>
                      <AlertDescription className="mt-2 text-sm">
                        {testResult.success ? (
                          <div className="space-y-2">
                            <div>
                              <strong>Latency:</strong> {testResult.latency}ms
                            </div>
                            <div>
                              <strong>Detected:</strong> {testResult.detected_entities?.length || 0} entities
                            </div>
                            {testResult.detected_entities?.map((entity: any, idx: number) => (
                              <Badge key={idx} variant="outline" className="mr-1">
                                {entity.entity}: {(entity.score * 100).toFixed(0)}%
                              </Badge>
                            ))}
                          </div>
                        ) : (
                          <div className="text-red-700">
                            {testResult.error || "Configuration test failed"}
                          </div>
                        )}
                      </AlertDescription>
                    </Alert>
                  )}
                </CardContent>
              </Card>
            </div>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-4 border-t pt-6">
            <Button 
              type="button" 
              variant="outline"
              onClick={() => navigate("/guardrails")}
            >
              Cancel
            </Button>
            <Button 
              type="submit" 
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />
                  Saving...
                </>
              ) : (
                <>
                  <Save className="h-4 w-4 mr-2" />
                  {isEditing ? "Update Guardrail" : "Create Guardrail"}
                </>
              )}
            </Button>
          </div>
        </form>
      </Form>
    </div>
  );
}