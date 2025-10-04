import { useState } from "react";
import { useNavigate } from "react-router-dom";

import { 
  Shield, 
  Search, 
  Filter,
  Star,
  Download,
  ExternalLink,
  Lock,
  ArrowRight,
  CheckCircle,
  AlertTriangle,
  Info,
  Github,
  Book
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Separator } from "@/components/ui/separator";
import { Alert, AlertDescription } from "@/components/ui/alert";

// Types
interface GuardrailProvider {
  id: string;
  name: string;
  description: string;
  short_description: string;
  provider: string;
  category: "privacy" | "safety" | "compliance" | "security";
  subcategory: string;
  version: string;
  author: {
    name: string;
    avatar?: string;
    company?: string;
    verified: boolean;
  };
  pricing: "free" | "paid" | "freemium";
  rating: number;
  downloads: number;
  last_updated: string;
  tags: string[];
  features: string[];
  supported_languages: string[];
  execution_modes: string[];
  integration_difficulty: "easy" | "medium" | "hard";
  documentation_url?: string;
  github_url?: string;
  homepage_url?: string;
  demo_available: boolean;
  enterprise_ready: boolean;
  open_source: boolean;
  compliance: string[];
  use_cases: string[];
  performance: {
    avg_latency_ms: number;
    throughput_rps: number;
    accuracy: number;
  };
}

// Mock data for demonstration
const MOCK_PROVIDERS: GuardrailProvider[] = [
  {
    id: "presidio",
    name: "Microsoft Presidio",
    description: "Microsoft Presidio is an open source data protection toolkit that helps identify, classify, and anonymize sensitive data in text, images, and documents. It provides built-in recognizers for common PII types and allows custom recognizers for specific use cases.",
    short_description: "Enterprise-grade PII detection and anonymization toolkit from Microsoft",
    provider: "Microsoft",
    category: "privacy",
    subcategory: "PII Detection",
    version: "2.2.354",
    author: {
      name: "Microsoft",
      avatar: "/api/placeholder/40/40",
      company: "Microsoft Corporation",
      verified: true
    },
    pricing: "free",
    rating: 4.8,
    downloads: 125000,
    last_updated: "2024-01-15",
    tags: ["PII", "anonymization", "GDPR", "HIPAA", "enterprise"],
    features: [
      "Multi-language PII detection",
      "Custom entity recognizers", 
      "Multiple anonymization methods",
      "REST API integration",
      "Batch processing support"
    ],
    supported_languages: ["en", "es", "fr", "de", "it", "pt", "he"],
    execution_modes: ["pre_call", "post_call", "logging_only"],
    integration_difficulty: "easy",
    documentation_url: "https://microsoft.github.io/presidio/",
    github_url: "https://github.com/microsoft/presidio",
    homepage_url: "https://presidio.dev/",
    demo_available: true,
    enterprise_ready: true,
    open_source: true,
    compliance: ["GDPR", "HIPAA", "PCI DSS", "SOX"],
    use_cases: ["Healthcare data protection", "Financial services", "Customer support", "Document processing"],
    performance: {
      avg_latency_ms: 85,
      throughput_rps: 1200,
      accuracy: 0.94
    }
  },
  {
    id: "aws-comprehend-pii",
    name: "AWS Comprehend PII",
    description: "Amazon Comprehend provides natural language processing capabilities including PII detection and redaction. It can identify common PII entities like names, addresses, credit card numbers, and more across multiple languages.",
    short_description: "AWS managed PII detection service with high accuracy and scalability",
    provider: "Amazon Web Services",
    category: "privacy",
    subcategory: "PII Detection",
    version: "1.0.0",
    author: {
      name: "AWS",
      avatar: "/api/placeholder/40/40",
      company: "Amazon Web Services",
      verified: true
    },
    pricing: "paid",
    rating: 4.6,
    downloads: 85000,
    last_updated: "2024-01-10",
    tags: ["AWS", "managed", "scalable", "multi-language"],
    features: [
      "Managed service",
      "High accuracy detection",
      "Batch and real-time processing",
      "Custom entity types",
      "Confidence scoring"
    ],
    supported_languages: ["en", "es", "fr", "de", "it", "pt", "ar", "hi", "ja", "ko", "zh"],
    execution_modes: ["pre_call", "post_call"],
    integration_difficulty: "medium",
    documentation_url: "https://docs.aws.amazon.com/comprehend/",
    homepage_url: "https://aws.amazon.com/comprehend/",
    demo_available: false,
    enterprise_ready: true,
    open_source: false,
    compliance: ["SOC 2", "HIPAA", "PCI DSS"],
    use_cases: ["Content moderation", "Document analysis", "Customer data protection"],
    performance: {
      avg_latency_ms: 120,
      throughput_rps: 2000,
      accuracy: 0.96
    }
  },
  {
    id: "openai-moderation",
    name: "OpenAI Moderation",
    description: "OpenAI's moderation models are designed to check whether content complies with OpenAI's usage policies. These models provide classification capabilities across several categories of content.",
    short_description: "AI-powered content moderation from OpenAI for safety and compliance",
    provider: "OpenAI",
    category: "safety", 
    subcategory: "Content Moderation",
    version: "text-moderation-007",
    author: {
      name: "OpenAI",
      avatar: "/api/placeholder/40/40",
      company: "OpenAI",
      verified: true
    },
    pricing: "freemium",
    rating: 4.5,
    downloads: 67000,
    last_updated: "2024-01-08",
    tags: ["content moderation", "safety", "AI", "real-time"],
    features: [
      "Real-time content analysis",
      "Multiple violation categories",
      "Confidence scoring",
      "Low latency",
      "Easy integration"
    ],
    supported_languages: ["en"],
    execution_modes: ["pre_call", "post_call", "during_call"],
    integration_difficulty: "easy",
    documentation_url: "https://platform.openai.com/docs/guides/moderation",
    homepage_url: "https://openai.com/",
    demo_available: true,
    enterprise_ready: true,
    open_source: false,
    compliance: ["OpenAI Usage Policies"],
    use_cases: ["Chatbot safety", "User-generated content", "Community moderation"],
    performance: {
      avg_latency_ms: 45,
      throughput_rps: 5000,
      accuracy: 0.92
    }
  },
  {
    id: "perspective-api",
    name: "Perspective API",
    description: "Perspective API uses machine learning models to score the perceived impact a comment might have on a conversation. It helps online platforms foster healthier discussions by identifying toxic, insulting, or threatening comments.",
    short_description: "Google's toxicity detection API for healthier online conversations",
    provider: "Google Jigsaw",
    category: "safety",
    subcategory: "Toxicity Detection",
    version: "1.0.0",
    author: {
      name: "Jigsaw",
      avatar: "/api/placeholder/40/40", 
      company: "Google",
      verified: true
    },
    pricing: "free",
    rating: 4.3,
    downloads: 45000,
    last_updated: "2023-12-20",
    tags: ["toxicity", "harassment", "community", "free"],
    features: [
      "Toxicity scoring",
      "Multiple attribute analysis",
      "Language support",
      "Free tier available",
      "Real-time analysis"
    ],
    supported_languages: ["en", "es", "fr", "de", "it", "pt", "ru"],
    execution_modes: ["pre_call", "post_call"],
    integration_difficulty: "easy",
    documentation_url: "https://developers.perspectiveapi.com/",
    homepage_url: "https://perspectiveapi.com/",
    demo_available: true,
    enterprise_ready: false,
    open_source: false,
    compliance: ["Google Cloud Terms"],
    use_cases: ["Community moderation", "Comment filtering", "Social media safety"],
    performance: {
      avg_latency_ms: 95,
      throughput_rps: 800,
      accuracy: 0.89
    }
  },
  {
    id: "custom-regex",
    name: "Custom Regex Guardrail",
    description: "A flexible guardrail that allows you to define custom regular expressions for pattern matching and content filtering. Suitable for organization-specific rules and custom compliance requirements.",
    short_description: "Flexible pattern-based guardrail using custom regular expressions",
    provider: "Community",
    category: "compliance",
    subcategory: "Pattern Matching",
    version: "1.2.0",
    author: {
      name: "Community",
      avatar: "/api/placeholder/40/40",
      company: "Open Source",
      verified: false
    },
    pricing: "free",
    rating: 4.1,
    downloads: 23000,
    last_updated: "2024-01-05",
    tags: ["regex", "custom", "pattern", "compliance"],
    features: [
      "Custom regex patterns",
      "Multiple pattern sets",
      "Case-sensitive options",
      "Whitelist/blacklist modes",
      "Fast pattern matching"
    ],
    supported_languages: ["universal"],
    execution_modes: ["pre_call", "post_call", "logging_only"],
    integration_difficulty: "medium",
    github_url: "https://github.com/community/regex-guardrail",
    demo_available: false,
    enterprise_ready: false,
    open_source: true,
    compliance: ["Custom"],
    use_cases: ["Custom compliance rules", "Industry-specific patterns", "Internal policies"],
    performance: {
      avg_latency_ms: 15,
      throughput_rps: 10000,
      accuracy: 0.85
    }
  }
];

const CATEGORIES = [
  { value: "all", label: "All Categories", icon: Shield },
  { value: "privacy", label: "Privacy & PII", icon: Lock },
  { value: "safety", label: "Content Safety", icon: AlertTriangle },
  { value: "compliance", label: "Compliance", icon: CheckCircle },
  { value: "security", label: "Security", icon: Shield }
];

const PRICING_FILTERS = [
  { value: "all", label: "All" },
  { value: "free", label: "Free" },
  { value: "freemium", label: "Freemium" },
  { value: "paid", label: "Paid" }
];

const DIFFICULTY_FILTERS = [
  { value: "all", label: "All" },
  { value: "easy", label: "Easy" },
  { value: "medium", label: "Medium" },
  { value: "hard", label: "Hard" }
];

export default function GuardrailMarketplace() {
  const navigate = useNavigate();
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedCategory, setSelectedCategory] = useState("all");
  const [pricingFilter, setPricingFilter] = useState("all");
  const [difficultyFilter, setDifficultyFilter] = useState("all");
  const [sortBy, setSortBy] = useState("downloads"); // downloads, rating, updated

  // Filter and sort providers
  const filteredProviders = MOCK_PROVIDERS
    .filter(provider => {
      const matchesSearch = searchQuery === "" || 
        provider.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        provider.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
        provider.tags.some(tag => tag.toLowerCase().includes(searchQuery.toLowerCase()));
      
      const matchesCategory = selectedCategory === "all" || provider.category === selectedCategory;
      const matchesPricing = pricingFilter === "all" || provider.pricing === pricingFilter;
      const matchesDifficulty = difficultyFilter === "all" || provider.integration_difficulty === difficultyFilter;
      
      return matchesSearch && matchesCategory && matchesPricing && matchesDifficulty;
    })
    .sort((a, b) => {
      switch (sortBy) {
        case "rating":
          return b.rating - a.rating;
        case "updated":
          return new Date(b.last_updated).getTime() - new Date(a.last_updated).getTime();
        case "downloads":
        default:
          return b.downloads - a.downloads;
      }
    });

  const formatDownloads = (count: number) => {
    if (count >= 1000000) return `${(count / 1000000).toFixed(1)}M`;
    if (count >= 1000) return `${(count / 1000).toFixed(1)}K`;
    return count.toString();
  };

  const getPricingColor = (pricing: string) => {
    switch (pricing) {
      case "free": return "bg-green-100 text-green-800";
      case "freemium": return "bg-blue-100 text-blue-800";
      case "paid": return "bg-yellow-100 text-yellow-800";
      default: return "bg-gray-100 text-gray-800";
    }
  };

  const getDifficultyColor = (difficulty: string) => {
    switch (difficulty) {
      case "easy": return "bg-green-100 text-green-800";
      case "medium": return "bg-yellow-100 text-yellow-800";
      case "hard": return "bg-red-100 text-red-800";
      default: return "bg-gray-100 text-gray-800";
    }
  };

  const getCategoryIcon = (category: string) => {
    const categoryData = CATEGORIES.find(c => c.value === category);
    return categoryData?.icon || Shield;
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl lg:text-3xl font-bold">Guardrail Marketplace</h1>
        <p className="text-muted-foreground">
          Discover and integrate guardrails from verified providers to protect your LLM applications
        </p>
      </div>

      {/* Featured Notice */}
      <Alert>
        <Info className="h-4 w-4" />
        <AlertDescription>
          These guardrail providers offer various levels of PII detection, content safety, and compliance features. 
          Choose based on your specific needs and integration requirements.
        </AlertDescription>
      </Alert>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Filter className="h-5 w-5" />
            Filters & Search
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col md:flex-row gap-4">
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search guardrails, providers, or features..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>
            <div className="flex gap-2">
              <Select value={pricingFilter} onValueChange={setPricingFilter}>
                <SelectTrigger className="w-[130px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PRICING_FILTERS.map(filter => (
                    <SelectItem key={filter.value} value={filter.value}>
                      {filter.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={difficultyFilter} onValueChange={setDifficultyFilter}>
                <SelectTrigger className="w-[130px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DIFFICULTY_FILTERS.map(filter => (
                    <SelectItem key={filter.value} value={filter.value}>
                      {filter.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={sortBy} onValueChange={setSortBy}>
                <SelectTrigger className="w-[130px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="downloads">Most Downloaded</SelectItem>
                  <SelectItem value="rating">Highest Rated</SelectItem>
                  <SelectItem value="updated">Recently Updated</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Categories */}
      <Tabs value={selectedCategory} onValueChange={setSelectedCategory}>
        <TabsList className="grid w-full grid-cols-5">
          {CATEGORIES.map((category) => {
            const Icon = category.icon;
            return (
              <TabsTrigger key={category.value} value={category.value} className="flex items-center gap-2">
                <Icon className="h-4 w-4" />
                <span className="hidden md:inline">{category.label}</span>
              </TabsTrigger>
            );
          })}
        </TabsList>

        {CATEGORIES.map((category) => (
          <TabsContent key={category.value} value={category.value} className="space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                {filteredProviders.length} guardrails found
              </p>
            </div>

            {/* Provider Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {filteredProviders.map((provider) => {
                const CategoryIcon = getCategoryIcon(provider.category);
                return (
                  <Card key={provider.id} className="flex flex-col hover:shadow-md transition-shadow">
                    <CardHeader className="pb-2">
                      <div className="flex items-start justify-between">
                        <div className="flex items-center gap-3">
                          <div className="p-2 bg-primary/10 rounded-lg">
                            <CategoryIcon className="h-5 w-5 text-primary" />
                          </div>
                          <div className="flex-1">
                            <CardTitle className="text-lg leading-tight">{provider.name}</CardTitle>
                            <div className="flex items-center gap-2 mt-1">
                              <Avatar className="h-4 w-4">
                                <AvatarImage src={provider.author.avatar} alt={provider.author.name} />
                                <AvatarFallback>{provider.author.name.charAt(0)}</AvatarFallback>
                              </Avatar>
                              <span className="text-xs text-muted-foreground">{provider.author.name}</span>
                              {provider.author.verified && (
                                <CheckCircle className="h-3 w-3 text-blue-500" />
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                      <CardDescription className="text-sm line-clamp-2">
                        {provider.short_description}
                      </CardDescription>
                    </CardHeader>
                    
                    <CardContent className="flex-1 space-y-4">
                      {/* Stats */}
                      <div className="flex items-center justify-between text-sm">
                        <div className="flex items-center gap-1">
                          <Star className="h-3 w-3 fill-yellow-400 text-yellow-400" />
                          <span className="font-medium">{provider.rating}</span>
                        </div>
                        <div className="flex items-center gap-1 text-muted-foreground">
                          <Download className="h-3 w-3" />
                          <span>{formatDownloads(provider.downloads)}</span>
                        </div>
                      </div>

                      {/* Badges */}
                      <div className="flex flex-wrap gap-2">
                        <Badge className={getPricingColor(provider.pricing)}>
                          {provider.pricing}
                        </Badge>
                        <Badge className={getDifficultyColor(provider.integration_difficulty)}>
                          {provider.integration_difficulty}
                        </Badge>
                        {provider.enterprise_ready && (
                          <Badge variant="outline" className="text-xs">
                            Enterprise
                          </Badge>
                        )}
                        {provider.open_source && (
                          <Badge variant="outline" className="text-xs">
                            Open Source
                          </Badge>
                        )}
                      </div>

                      {/* Key Features */}
                      <div>
                        <p className="text-xs font-medium text-muted-foreground mb-1">Key Features</p>
                        <div className="text-xs text-muted-foreground">
                          {provider.features.slice(0, 2).join(" • ")}
                          {provider.features.length > 2 && ` • +${provider.features.length - 2} more`}
                        </div>
                      </div>

                      {/* Performance Metrics */}
                      <div className="grid grid-cols-2 gap-2 text-xs">
                        <div>
                          <span className="text-muted-foreground">Latency:</span>
                          <span className="font-medium ml-1">{provider.performance.avg_latency_ms}ms</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Accuracy:</span>
                          <span className="font-medium ml-1">{(provider.performance.accuracy * 100).toFixed(0)}%</span>
                        </div>
                      </div>

                      <Separator />

                      {/* Actions */}
                      <div className="flex items-center gap-2">
                        <Button
                          size="sm"
                          className="flex-1"
                          onClick={() => navigate(`/guardrails/config/new?provider=${provider.id}`)}
                        >
                          <ArrowRight className="h-3 w-3 mr-1" />
                          Configure
                        </Button>
                        <div className="flex gap-1">
                          {provider.documentation_url && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => window.open(provider.documentation_url, '_blank')}
                            >
                              <Book className="h-3 w-3" />
                            </Button>
                          )}
                          {provider.github_url && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => window.open(provider.github_url, '_blank')}
                            >
                              <Github className="h-3 w-3" />
                            </Button>
                          )}
                          {provider.homepage_url && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => window.open(provider.homepage_url, '_blank')}
                            >
                              <ExternalLink className="h-3 w-3" />
                            </Button>
                          )}
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                );
              })}
            </div>

            {filteredProviders.length === 0 && (
              <Card>
                <CardContent className="flex flex-col items-center justify-center py-12">
                  <Shield className="h-12 w-12 text-muted-foreground mb-4" />
                  <h3 className="text-lg font-semibold mb-2">No Guardrails Found</h3>
                  <p className="text-muted-foreground text-center max-w-md">
                    No guardrails match your current filters. Try adjusting your search criteria or browse all categories.
                  </p>
                  <Button
                    variant="outline"
                    onClick={() => {
                      setSearchQuery("");
                      setSelectedCategory("all");
                      setPricingFilter("all");
                      setDifficultyFilter("all");
                    }}
                    className="mt-4"
                  >
                    Clear Filters
                  </Button>
                </CardContent>
              </Card>
            )}
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}