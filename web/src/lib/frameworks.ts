// Mapping of agent framework strings (as captured in
// `pllm.agent.framework` OTel resource attribute / RegistryAgent.framework)
// to iconify slugs and display labels. Used everywhere we want to
// render the framework's vendor logo.

export interface FrameworkInfo {
  /** Display label, e.g. "LangGraph". */
  label: string;
  /** iconify slug. Pick logos:* when available; falls back to simple-icons:*. */
  icon: string;
  /** Optional tailwind text-color class for the icon glyph (used when the
   * iconify icon is monochrome). Empty for full-color logos. */
  tone?: string;
}

const FRAMEWORKS: Record<string, FrameworkInfo> = {
  langgraph:   { label: "LangGraph",   icon: "logos:langchain" },
  langchain:   { label: "LangChain",   icon: "logos:langchain" },
  crewai:      { label: "CrewAI",      icon: "simple-icons:crewai", tone: "text-orange-600 dark:text-orange-400" },
  autogen:     { label: "AutoGen",     icon: "logos:microsoft" },
  llamaindex:  { label: "LlamaIndex",  icon: "logos:llamaindex" },
  haystack:    { label: "Haystack",    icon: "simple-icons:deepset", tone: "text-sky-600 dark:text-sky-400" },
  semantickernel: { label: "Semantic Kernel", icon: "logos:microsoft" },
  pydanticai:  { label: "Pydantic AI", icon: "simple-icons:pydantic", tone: "text-emerald-600 dark:text-emerald-400" },
};

const UNKNOWN: FrameworkInfo = {
  label: "Custom",
  icon: "solar:cpu-bolt-linear",
  tone: "text-muted-foreground",
};

export function frameworkInfo(framework?: string | null): FrameworkInfo | null {
  if (!framework) return null;
  const key = framework.trim().toLowerCase().replace(/[\s_-]+/g, "");
  return FRAMEWORKS[key] ?? { ...UNKNOWN, label: framework };
}
