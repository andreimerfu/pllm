import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Palette, Search, ChevronDown, ChevronRight, Settings as SettingsIcon, Shield, Zap, CheckCircle, Clock } from "lucide-react";
import { useTheme } from "@/contexts/ThemeContext";
import type { ThemeCategory, ThemeKey } from "@/contexts/ThemeContext";

// Enhanced theme gallery component
function ThemeGallery() {
  const { currentTheme, setTheme, themes, categories, getThemesByCategory, searchThemes } = useTheme();
  const [searchQuery, setSearchQuery] = useState('');
  const [expandedCategories, setExpandedCategories] = useState<Set<ThemeCategory>>(new Set(['popular']));
  
  const toggleCategory = (category: ThemeCategory) => {
    const newExpanded = new Set(expandedCategories);
    if (newExpanded.has(category)) {
      newExpanded.delete(category);
    } else {
      newExpanded.add(category);
    }
    setExpandedCategories(newExpanded);
  };

  const searchResults = searchQuery ? searchThemes(searchQuery) : [];
  const categoryOrder: ThemeCategory[] = ['popular', 'nature', 'corporate', 'vibrant', 'warm', 'cool', 'retro', 'monochrome', 'gradient'];

  return (
    <div className="space-y-6">
      {/* Current theme display */}
      <div className="space-y-4">
        <div className="flex items-start justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Label className="text-lg font-semibold">{themes[currentTheme].name}</Label>
              <Badge variant="secondary" className="text-xs">
                {categories[themes[currentTheme].category].name}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground">
              {themes[currentTheme].description}
            </p>
          </div>
          <div className="flex gap-1.5">
            {Object.values(themes[currentTheme].colors).map((color, i) => (
              <div
                key={i}
                className="w-5 h-5 rounded-full border-2 border-background shadow-sm ring-1 ring-border/20"
                style={{ backgroundColor: `hsl(${color})` }}
                title={`Color ${i + 1}`}
              />
            ))}
          </div>
        </div>
        
        {/* Preview chart */}
        <div className="p-4 bg-gradient-to-br from-muted/30 to-muted/50 rounded-xl border">
          <div className="flex items-end justify-between gap-2 h-16 mb-2">
            {Object.values(themes[currentTheme].colors).map((color, i) => (
              <div
                key={i}
                className="flex-1 rounded-t-lg transition-all duration-300 hover:opacity-80 hover:scale-105"
                style={{
                  backgroundColor: `hsl(${color})`,
                  height: `${50 + (i * 10)}%`,
                  boxShadow: `0 2px 8px hsla(${color}, 0.3)`
                }}
              />
            ))}
          </div>
          <p className="text-xs text-muted-foreground text-center font-medium">
            Live Chart Preview
          </p>
        </div>
      </div>

      {/* Search */}
      <div className="space-y-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground h-4 w-4" />
          <Input
            placeholder="Search themes by name, description, or category..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 h-11"
          />
        </div>
      </div>

      <ScrollArea className="h-[600px] w-full">
        <div className="space-y-6 pr-4">
          {/* Search results */}
          {searchQuery && (
            <div className="space-y-3">
              <Label className="text-base font-semibold flex items-center gap-2">
                <Search className="h-4 w-4" />
                Search Results ({searchResults.length})
              </Label>
              {searchResults.length > 0 ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                  {searchResults.map(({ key, config }) => (
                    <ThemeCard key={key} themeKey={key} config={config} currentTheme={currentTheme} onSelect={setTheme} />
                  ))}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground py-8 text-center">
                  No themes found matching "{searchQuery}"
                </p>
              )}
            </div>
          )}

          {/* Category sections */}
          {!searchQuery && categoryOrder.map((category) => {
            const categoryThemes = getThemesByCategory(category);
            if (categoryThemes.length === 0) return null;
            
            const isExpanded = expandedCategories.has(category);
            
            return (
              <Collapsible key={category} open={isExpanded} onOpenChange={() => toggleCategory(category)}>
                <CollapsibleTrigger className="flex items-center justify-between w-full p-3 rounded-lg hover:bg-muted/50 transition-colors group">
                  <div className="flex items-center gap-3">
                    <div className="flex items-center gap-2">
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-muted-foreground group-hover:text-foreground transition-colors" />
                      )}
                      <Label className="text-base font-semibold cursor-pointer">
                        {categories[category].name}
                      </Label>
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {categoryThemes.length}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground hidden sm:block">
                    {categories[category].description}
                  </p>
                </CollapsibleTrigger>
                
                <CollapsibleContent className="space-y-3 pt-3">
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                    {categoryThemes.map(({ key, config }) => (
                      <ThemeCard key={key} themeKey={key} config={config} currentTheme={currentTheme} onSelect={setTheme} />
                    ))}
                  </div>
                </CollapsibleContent>
              </Collapsible>
            );
          })}
        </div>
      </ScrollArea>
    </div>
  );
}

// Individual theme card component
function ThemeCard({ 
  themeKey, 
  config, 
  currentTheme, 
  onSelect 
}: { 
  themeKey: ThemeKey; 
  config: any; 
  currentTheme: ThemeKey; 
  onSelect: (theme: ThemeKey) => void;
}) {
  const isSelected = currentTheme === themeKey;
  
  return (
    <button
      onClick={() => onSelect(themeKey)}
      className={`group relative p-4 rounded-xl border-2 transition-all duration-200 text-left hover:shadow-lg ${
        isSelected 
          ? 'border-primary bg-primary/5 shadow-md ring-2 ring-primary/20' 
          : 'border-border hover:border-muted-foreground/30 hover:bg-muted/30'
      }`}
    >
      {isSelected && (
        <div className="absolute -top-1 -right-1">
          <div className="bg-primary rounded-full p-1">
            <CheckCircle className="h-3 w-3 text-primary-foreground" />
          </div>
        </div>
      )}
      
      <div className="space-y-3">
        <div className="flex items-start justify-between">
          <div className="space-y-1 flex-1 min-w-0">
            <div className="font-medium text-sm truncate group-hover:text-primary transition-colors">
              {config.name}
            </div>
            <p className="text-xs text-muted-foreground line-clamp-2">
              {config.description}
            </p>
          </div>
        </div>
        
        {/* Color preview */}
        <div className="flex gap-1">
          {Object.values(config.colors).map((color, i) => (
            <div
              key={i}
              className="flex-1 h-6 first:rounded-l-md last:rounded-r-md transition-transform group-hover:scale-105"
              style={{ 
                backgroundColor: `hsl(${color as string})`,
                boxShadow: `inset 0 0 0 1px hsla(${color as string}, 0.1)` 
              }}
            />
          ))}
        </div>
      </div>
    </button>
  );
}

// Coming soon placeholder
function ComingSoonTab({ icon: Icon, title, description }: { 
  icon: React.ElementType; 
  title: string; 
  description: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center space-y-4">
      <div className="p-4 rounded-full bg-muted/50">
        <Icon className="h-8 w-8 text-muted-foreground" />
      </div>
      <div className="space-y-2">
        <h3 className="text-lg font-semibold text-muted-foreground">{title}</h3>
        <p className="text-sm text-muted-foreground max-w-md">
          {description}
        </p>
        <div className="flex items-center gap-2 justify-center pt-2">
          <Clock className="h-4 w-4 text-muted-foreground" />
          <span className="text-xs text-muted-foreground">Coming Soon</span>
        </div>
      </div>
    </div>
  );
}

export default function Settings() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold bg-gradient-to-r from-foreground to-foreground/70 bg-clip-text text-transparent">
          Settings
        </h1>
        <p className="text-muted-foreground mt-2">
          Configure your dashboard preferences and customize your experience
        </p>
      </div>

      <Tabs defaultValue="appearance" className="space-y-6">
        <TabsList className="grid w-full grid-cols-2 lg:grid-cols-4 h-11">
          <TabsTrigger value="appearance" className="flex items-center gap-2 text-sm">
            <Palette className="h-4 w-4" />
            <span>Appearance</span>
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-xs bg-green-100 text-green-800 border-green-200">
              ✓
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="general" className="flex items-center gap-2 text-sm" disabled>
            <SettingsIcon className="h-4 w-4" />
            <span className="hidden sm:inline">General</span>
            <Badge variant="outline" className="ml-1 h-5 px-1.5 text-xs">
              Soon
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="security" className="flex items-center gap-2 text-sm" disabled>
            <Shield className="h-4 w-4" />
            <span className="hidden sm:inline">Security</span>
            <Badge variant="outline" className="ml-1 h-5 px-1.5 text-xs">
              Soon
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="integrations" className="flex items-center gap-2 text-sm" disabled>
            <Zap className="h-4 w-4" />
            <span className="hidden sm:inline">Integrations</span>
            <Badge variant="outline" className="ml-1 h-5 px-1.5 text-xs">
              Soon
            </Badge>
          </TabsTrigger>
        </TabsList>

        <TabsContent value="appearance" className="space-y-0">
          <Card>
            <CardHeader>
              <CardTitle className="text-2xl flex items-center gap-3">
                <div className="p-2 rounded-lg bg-primary/10">
                  <Palette className="h-6 w-6 text-primary" />
                </div>
                Dashboard Themes
              </CardTitle>
              <CardDescription className="text-base leading-relaxed">
                Personalize your dashboard with beautiful color themes. Choose from 38 professionally designed themes across 9 categories.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <ThemeGallery />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="general">
          <ComingSoonTab 
            icon={SettingsIcon}
            title="General Settings"
            description="Configure basic gateway settings, API endpoints, logging preferences, and system-wide options."
          />
        </TabsContent>

        <TabsContent value="security">
          <ComingSoonTab 
            icon={Shield}
            title="Security Settings"
            description="Manage authentication, authorization, rate limiting, CORS policies, and security configurations."
          />
        </TabsContent>

        <TabsContent value="integrations">
          <ComingSoonTab 
            icon={Zap}
            title="Integrations"
            description="Connect with external services, configure webhooks, email notifications, and third-party integrations."
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
