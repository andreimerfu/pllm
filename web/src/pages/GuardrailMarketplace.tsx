import { useState } from "react";
import { useNavigate } from "react-router-dom";

import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionLabel } from "@/components/common/status-chip";
import { getBrandLogo, isColoredLogo } from "@/lib/brand-logos";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Catalog — curated list of installable guardrail integrations. Static
// reference data (the marketplace listing), not runtime metrics.
// ---------------------------------------------------------------------------

type Category =
  | "PII & Privacy"
  | "Content Safety"
  | "Prompt Security"
  | "Toxicity"
  | "Secrets"
  | "Compliance";

type Pricing = "Free" | "Paid" | "Enterprise";

interface Guardrail {
  id: string;
  name: string;
  provider: string;
  category: Category;
  description: string;
  pricing: Pricing;
  rating: number;
  downloads: number;
  verified: boolean;
  installed: boolean;
  featured: boolean;
}

const GUARDRAILS: Guardrail[] = [
  { id: "presidio", name: "Microsoft Presidio", provider: "Microsoft", category: "PII & Privacy", pricing: "Free", rating: 4.8, downloads: 12400, verified: true, installed: true, featured: true, description: "Detect and redact 40+ PII entity types — names, SSNs, cards, emails — before requests reach the model." },
  { id: "lakera", name: "Lakera Guard", provider: "Lakera", category: "Prompt Security", pricing: "Paid", rating: 4.9, downloads: 8100, verified: true, installed: true, featured: true, description: "Real-time defense against prompt injection, jailbreaks and data exfiltration, backed by a live threat database." },
  { id: "openai-moderation", name: "OpenAI Moderation", provider: "OpenAI", category: "Content Safety", pricing: "Free", rating: 4.7, downloads: 21300, verified: true, installed: true, featured: true, description: "Flag hate, harassment, self-harm, sexual and violent content across requests and responses." },
  { id: "azure-content-safety", name: "Azure AI Content Safety", provider: "Microsoft", category: "Content Safety", pricing: "Paid", rating: 4.6, downloads: 9700, verified: true, installed: false, featured: false, description: "Severity-scored moderation across four harm categories with configurable blocklists." },
  { id: "bedrock-guardrails", name: "Bedrock Guardrails", provider: "Amazon", category: "Content Safety", pricing: "Paid", rating: 4.5, downloads: 6200, verified: false, installed: false, featured: false, description: "Denied topics, word filters and contextual grounding checks managed natively in AWS." },
  { id: "nemo-guardrails", name: "NeMo Guardrails", provider: "NVIDIA", category: "Prompt Security", pricing: "Free", rating: 4.5, downloads: 4800, verified: true, installed: false, featured: false, description: "Programmable rails for dialog, topic and safety control using Colang flows." },
  { id: "protect-ai", name: "Protect AI Guardian", provider: "Protect AI", category: "Secrets", pricing: "Paid", rating: 4.4, downloads: 3100, verified: true, installed: false, featured: false, description: "Catch API keys, tokens and credentials leaking through prompts and tool calls." },
  { id: "guardrails-ai", name: "Guardrails AI", provider: "Guardrails AI", category: "Compliance", pricing: "Free", rating: 4.6, downloads: 5500, verified: true, installed: false, featured: false, description: "Open-source validators for structure, format and policy compliance on every output." },
  { id: "skyflow", name: "Skyflow PII Shield", provider: "Skyflow", category: "PII & Privacy", pricing: "Paid", rating: 4.4, downloads: 1800, verified: true, installed: false, featured: false, description: "Tokenize and de-identify sensitive data with a privacy vault before it ever hits an LLM." },
  { id: "detoxify", name: "Detoxify", provider: "Community", category: "Toxicity", pricing: "Free", rating: 4.2, downloads: 2900, verified: false, installed: false, featured: false, description: "Lightweight toxicity, severe-toxicity and threat scoring on model responses." },
  { id: "aporia", name: "Aporia Guardrails", provider: "Aporia", category: "Content Safety", pricing: "Paid", rating: 4.5, downloads: 2100, verified: true, installed: false, featured: false, description: "Hallucination, RAG-grounding and off-topic detection tuned for production traffic." },
  { id: "robust-intelligence", name: "Robust Intelligence", provider: "Cisco", category: "Compliance", pricing: "Enterprise", rating: 4.6, downloads: 1500, verified: true, installed: false, featured: false, description: "Continuous AI validation and policy enforcement mapped to regulatory frameworks." },
  { id: "rebuff", name: "Rebuff", provider: "Community", category: "Prompt Security", pricing: "Free", rating: 4.2, downloads: 2400, verified: false, installed: false, featured: false, description: "Self-hardening prompt-injection detector combining heuristics, LLM checks and a vector store." },
  { id: "langkit", name: "WhyLabs LangKit", provider: "WhyLabs", category: "Toxicity", pricing: "Free", rating: 4.1, downloads: 1900, verified: true, installed: false, featured: false, description: "Out-of-the-box text metrics — toxicity, sentiment, PII and quality — for monitoring LLM traffic." },
];

const CATEGORIES: ("All" | Category)[] = [
  "All", "PII & Privacy", "Content Safety", "Prompt Security", "Toxicity", "Secrets", "Compliance",
];

// Category → kind tint (color reserved for meaning).
const CATEGORY_TINT: Record<Category, string> = {
  "PII & Privacy": "bg-kind-model/12 text-kind-model",
  "Content Safety": "bg-kind-prompt/12 text-kind-prompt",
  "Prompt Security": "bg-kind-skill/12 text-kind-skill",
  Toxicity: "bg-bad/12 text-bad",
  Secrets: "bg-kind-mcp/12 text-kind-mcp",
  Compliance: "bg-kind-agent/12 text-kind-agent",
};

// Solid text color per category — used for the verified seal so it reads
// in the provider's category color, matching the reference.
const CATEGORY_TEXT: Record<Category, string> = {
  "PII & Privacy": "text-kind-model",
  "Content Safety": "text-kind-prompt",
  "Prompt Security": "text-kind-skill",
  Toxicity: "text-bad",
  Secrets: "text-kind-mcp",
  Compliance: "text-kind-agent",
};

function VerifiedSeal({ category }: { category: Category }) {
  return <Icon icon="lucide:badge-check" className={cn("h-4 w-4 shrink-0", CATEGORY_TEXT[category])} />;
}

function fmtDownloads(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);
}

function PricingTag({ pricing }: { pricing: Pricing }) {
  const cls =
    pricing === "Free"
      ? "bg-good/12 text-good"
      : pricing === "Enterprise"
        ? "bg-info/12 text-info"
        : "bg-surface-2 text-text-dim";
  return (
    <span className={cn("inline-flex items-center rounded-md px-2 py-[3px] text-[11px] font-medium", cls)}>
      {pricing}
    </span>
  );
}

function CategoryTag({ category }: { category: Category }) {
  return (
    <span className={cn("inline-flex items-center rounded-md px-2 py-[3px] text-[10.5px] font-semibold", CATEGORY_TINT[category])}>
      {category}
    </span>
  );
}

function Rating({ rating, downloads }: { rating: number; downloads: number }) {
  return (
    <div className="flex items-center gap-3 text-[12px] text-text-dim">
      <span className="inline-flex items-center gap-1">
        <Icon icon={icons.star} className="h-3.5 w-3.5 text-warn" />
        <span className="font-mono text-foreground">{rating.toFixed(1)}</span>
      </span>
      <span className="inline-flex items-center gap-1">
        <Icon icon={icons.download} className="h-3.5 w-3.5" />
        <span className="font-mono">{fmtDownloads(downloads)}</span>
      </span>
    </div>
  );
}

export default function GuardrailMarketplace() {
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<"All" | Category>("All");
  const [installed, setInstalled] = useState<Record<string, boolean>>(
    Object.fromEntries(GUARDRAILS.filter((g) => g.installed).map((g) => [g.id, true])),
  );

  const install = (g: Guardrail) => {
    setInstalled((prev) => ({ ...prev, [g.id]: true }));
    navigate(`/guardrails/config/new?provider=${g.id}`);
  };

  const matches = (g: Guardrail) => {
    const q = search.trim().toLowerCase();
    const inSearch =
      !q || g.name.toLowerCase().includes(q) || g.provider.toLowerCase().includes(q) || g.description.toLowerCase().includes(q);
    const inCategory = category === "All" || g.category === category;
    return inSearch && inCategory;
  };

  const featured = GUARDRAILS.filter((g) => g.featured);
  const all = GUARDRAILS.filter(matches);

  // Featured cards use a solid green "Installed" button.
  const FeaturedInstall = ({ g }: { g: Guardrail }) =>
    installed[g.id] ? (
      <Button size="sm" disabled>Installed</Button>
    ) : (
      <Button size="sm" onClick={() => install(g)}>Install</Button>
    );

  // Grid cards show installed as inline green text, install as an outline button.
  const GridInstall = ({ g }: { g: Guardrail }) =>
    installed[g.id] ? (
      <span className="inline-flex items-center gap-1.5 text-[13px] font-medium text-primary">
        <Icon icon={icons.check} className="h-4 w-4" />
        Installed
      </span>
    ) : (
      <Button size="sm" variant="outline" onClick={() => install(g)}>Install</Button>
    );

  const Tile = ({ g, size = 38 }: { g: Guardrail; size?: number }) => {
    // Prefer the real provider/brand logo (Microsoft, OpenAI, AWS, NVIDIA…);
    // fall back to a category-tinted shield for vendors without one.
    const brand = getBrandLogo(g.provider) || getBrandLogo(g.name);
    const colored = brand ? isColoredLogo(brand) : false;
    return (
      <span
        className={cn(
          "inline-flex shrink-0 items-center justify-center rounded-[10px]",
          colored ? "bg-surface-2" : CATEGORY_TINT[g.category],
        )}
        style={{ width: size, height: size }}
      >
        <Icon icon={brand ?? icons.shield} className="h-[18px] w-[18px]" />
      </span>
    );
  };

  return (
    <div className="space-y-6 pb-10">
      {/* Header */}
      <div className="space-y-3">
        <button
          onClick={() => navigate("/guardrails")}
          className="inline-flex items-center gap-1.5 text-[13px] text-text-dim hover:text-foreground"
        >
          <Icon icon={icons.arrowLeft} className="h-4 w-4" />
          Guardrails
        </button>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="max-w-2xl space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">Guardrail Marketplace</h1>
            <p className="text-[13px] text-muted-foreground">
              Discover and install safety guardrails from trusted providers and the community. One click
              adds them to your gateway — governed by your local policies.
            </p>
          </div>
          <div className="relative w-full sm:w-72">
            <Icon icon={icons.search} className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-dim" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search guardrails…"
              className="h-9 pl-9"
            />
          </div>
        </div>
      </div>

      {/* Category pills */}
      <div className="flex flex-wrap items-center gap-2">
        {CATEGORIES.map((c) => {
          const active = c === category;
          return (
            <button
              key={c}
              onClick={() => setCategory(c)}
              className={cn(
                "h-8 rounded-full px-3.5 text-[13px] font-medium transition-colors",
                active
                  ? "bg-primary text-primary-foreground"
                  : "border border-border bg-card text-muted-foreground hover:text-foreground",
              )}
            >
              {c}
            </button>
          );
        })}
      </div>

      {/* Featured */}
      {category === "All" && !search && (
        <section className="space-y-3">
          <SectionLabel className="flex items-center gap-1.5">
            <Icon icon={icons.star} className="h-3.5 w-3.5 text-warn" />
            Featured
          </SectionLabel>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            {featured.map((g) => (
              <div key={g.id} className="flex flex-col rounded-[14px] border border-border bg-card p-5 shadow-sm">
                <div className="flex items-start gap-3">
                  <Tile g={g} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <h3 className="truncate font-semibold text-foreground">{g.name}</h3>
                      {g.verified && <VerifiedSeal category={g.category} />}
                    </div>
                    <p className="truncate text-[12px] text-text-dim">by {g.provider}</p>
                  </div>
                  <PricingTag pricing={g.pricing} />
                </div>
                <p className="mt-3 line-clamp-2 text-[13px] text-muted-foreground">{g.description}</p>
                <div className="mt-4 flex items-center justify-between border-t border-border pt-4">
                  <Rating rating={g.rating} downloads={g.downloads} />
                  <FeaturedInstall g={g} />
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* All guardrails */}
      <section className="space-y-3">
        <div className="flex items-baseline justify-between">
          <SectionLabel>All guardrails</SectionLabel>
          <span className="text-[12px] text-text-dim">{all.length} guardrails</span>
        </div>
        {all.length === 0 ? (
          <div className="rounded-[14px] border border-border bg-card p-12 text-center text-sm text-muted-foreground">
            No guardrails match your filters.
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {all.map((g) => (
              <div key={g.id} className="flex flex-col rounded-[14px] border border-border bg-card p-5 shadow-sm">
                <div className="flex items-start gap-3">
                  <Tile g={g} size={36} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <h3 className="truncate font-semibold text-foreground">{g.name}</h3>
                      {g.verified && <VerifiedSeal category={g.category} />}
                    </div>
                    <p className="truncate text-[12px] text-text-dim">by {g.provider}</p>
                  </div>
                </div>
                <p className="mt-3 line-clamp-2 text-[13px] text-muted-foreground">{g.description}</p>
                <div className="mt-3 flex items-center gap-3">
                  <CategoryTag category={g.category} />
                  <Rating rating={g.rating} downloads={g.downloads} />
                </div>
                <div className="mt-4 flex items-center justify-between border-t border-border pt-3">
                  <PricingTag pricing={g.pricing} />
                  <GridInstall g={g} />
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
