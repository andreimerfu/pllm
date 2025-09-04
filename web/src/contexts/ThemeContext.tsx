import React, { createContext, useContext, useEffect, useState } from 'react';

// Theme categories for organization
type ThemeCategory = 'popular' | 'nature' | 'corporate' | 'vibrant' | 'warm' | 'cool' | 'retro' | 'monochrome' | 'gradient';

// Extended theme configurations with 38 themes across 9 categories
const themeConfigs = {
  // POPULAR THEMES (6)
  default: {
    name: "Default",
    description: "Classic blue tones",
    category: 'popular' as ThemeCategory,
    colors: {
      "--chart-1": "220 70% 50%",
      "--chart-2": "160 60% 45%",
      "--chart-3": "30 80% 55%",
      "--chart-4": "280 65% 60%",
      "--chart-5": "340 75% 55%",
    }
  },
  modern: {
    name: "Modern",
    description: "Clean contemporary palette",
    category: 'popular' as ThemeCategory,
    colors: {
      "--chart-1": "210 50% 50%",
      "--chart-2": "140 40% 50%",
      "--chart-3": "260 50% 60%",
      "--chart-4": "40 60% 55%",
      "--chart-5": "320 50% 55%",
    }
  },
  professional: {
    name: "Professional",
    description: "Business-ready colors",
    category: 'popular' as ThemeCategory,
    colors: {
      "--chart-1": "215 25% 45%",
      "--chart-2": "205 20% 50%",
      "--chart-3": "225 20% 40%",
      "--chart-4": "195 15% 55%",
      "--chart-5": "235 15% 35%",
    }
  },
  minimal: {
    name: "Minimal",
    description: "Subtle and refined",
    category: 'popular' as ThemeCategory,
    colors: {
      "--chart-1": "210 15% 35%",
      "--chart-2": "200 12% 40%",
      "--chart-3": "220 10% 45%",
      "--chart-4": "190 8% 50%",
      "--chart-5": "230 6% 55%",
    }
  },
  elegant: {
    name: "Elegant",
    description: "Sophisticated neutrals",
    category: 'popular' as ThemeCategory,
    colors: {
      "--chart-1": "25 15% 40%",
      "--chart-2": "200 10% 45%",
      "--chart-3": "260 12% 50%",
      "--chart-4": "120 8% 35%",
      "--chart-5": "340 10% 55%",
    }
  },
  balanced: {
    name: "Balanced",
    description: "Harmonious color blend",
    category: 'popular' as ThemeCategory,
    colors: {
      "--chart-1": "200 40% 50%",
      "--chart-2": "150 35% 45%",
      "--chart-3": "50 45% 55%",
      "--chart-4": "280 40% 60%",
      "--chart-5": "20 35% 50%",
    }
  },

  // NATURE THEMES (5)
  ocean: {
    name: "Ocean",
    description: "Deep blue waters",
    category: 'nature' as ThemeCategory,
    colors: {
      "--chart-1": "194 80% 40%",
      "--chart-2": "200 85% 45%",
      "--chart-3": "206 90% 50%",
      "--chart-4": "212 85% 45%",
      "--chart-5": "218 80% 40%",
    }
  },
  forest: {
    name: "Forest",
    description: "Rich woodland greens",
    category: 'nature' as ThemeCategory,
    colors: {
      "--chart-1": "142 76% 36%",
      "--chart-2": "138 76% 32%",
      "--chart-3": "134 76% 28%",
      "--chart-4": "130 76% 24%",
      "--chart-5": "126 76% 20%",
    }
  },
  mountain: {
    name: "Mountain",
    description: "Earth and stone tones",
    category: 'nature' as ThemeCategory,
    colors: {
      "--chart-1": "25 45% 40%",
      "--chart-2": "20 50% 35%",
      "--chart-3": "15 55% 30%",
      "--chart-4": "30 40% 45%",
      "--chart-5": "35 35% 50%",
    }
  },
  meadow: {
    name: "Meadow",
    description: "Fresh spring greens",
    category: 'nature' as ThemeCategory,
    colors: {
      "--chart-1": "120 60% 50%",
      "--chart-2": "110 55% 45%",
      "--chart-3": "130 65% 55%",
      "--chart-4": "100 50% 40%",
      "--chart-5": "140 70% 60%",
    }
  },
  desert: {
    name: "Desert",
    description: "Warm sand and clay",
    category: 'nature' as ThemeCategory,
    colors: {
      "--chart-1": "35 50% 55%",
      "--chart-2": "30 45% 50%",
      "--chart-3": "25 55% 60%",
      "--chart-4": "40 40% 45%",
      "--chart-5": "20 60% 65%",
    }
  },

  // CORPORATE THEMES (4)
  corporate: {
    name: "Corporate",
    description: "Professional blues",
    category: 'corporate' as ThemeCategory,
    colors: {
      "--chart-1": "210 40% 40%",
      "--chart-2": "200 35% 45%",
      "--chart-3": "220 30% 50%",
      "--chart-4": "230 35% 45%",
      "--chart-5": "240 40% 40%",
    }
  },
  executive: {
    name: "Executive",
    description: "Premium business tones",
    category: 'corporate' as ThemeCategory,
    colors: {
      "--chart-1": "225 30% 35%",
      "--chart-2": "215 25% 40%",
      "--chart-3": "205 20% 45%",
      "--chart-4": "235 15% 30%",
      "--chart-5": "195 10% 50%",
    }
  },
  finance: {
    name: "Finance",
    description: "Trust and stability",
    category: 'corporate' as ThemeCategory,
    colors: {
      "--chart-1": "210 60% 25%",
      "--chart-2": "200 55% 30%",
      "--chart-3": "220 50% 35%",
      "--chart-4": "190 45% 40%",
      "--chart-5": "230 40% 20%",
    }
  },
  consulting: {
    name: "Consulting",
    description: "Strategic and refined",
    category: 'corporate' as ThemeCategory,
    colors: {
      "--chart-1": "240 25% 45%",
      "--chart-2": "210 20% 50%",
      "--chart-3": "180 15% 55%",
      "--chart-4": "270 30% 40%",
      "--chart-5": "150 10% 60%",
    }
  },

  // VIBRANT THEMES (5)
  neon: {
    name: "Neon",
    description: "Electric bright colors",
    category: 'vibrant' as ThemeCategory,
    colors: {
      "--chart-1": "315 100% 50%",
      "--chart-2": "180 100% 50%",
      "--chart-3": "60 100% 50%",
      "--chart-4": "120 100% 50%",
      "--chart-5": "270 100% 50%",
    }
  },
  electric: {
    name: "Electric",
    description: "High-energy palette",
    category: 'vibrant' as ThemeCategory,
    colors: {
      "--chart-1": "300 85% 60%",
      "--chart-2": "190 80% 55%",
      "--chart-3": "45 90% 65%",
      "--chart-4": "120 85% 60%",
      "--chart-5": "270 80% 70%",
    }
  },
  tropical: {
    name: "Tropical",
    description: "Vibrant paradise colors",
    category: 'vibrant' as ThemeCategory,
    colors: {
      "--chart-1": "170 70% 50%",
      "--chart-2": "340 80% 60%",
      "--chart-3": "50 85% 55%",
      "--chart-4": "280 75% 65%",
      "--chart-5": "20 90% 70%",
    }
  },
  rainbow: {
    name: "Rainbow",
    description: "Full spectrum colors",
    category: 'vibrant' as ThemeCategory,
    colors: {
      "--chart-1": "0 100% 50%",
      "--chart-2": "120 100% 40%",
      "--chart-3": "240 100% 60%",
      "--chart-4": "60 100% 45%",
      "--chart-5": "300 100% 55%",
    }
  },
  carnival: {
    name: "Carnival",
    description: "Festive celebration colors",
    category: 'vibrant' as ThemeCategory,
    colors: {
      "--chart-1": "330 90% 65%",
      "--chart-2": "50 95% 60%",
      "--chart-3": "280 85% 70%",
      "--chart-4": "160 80% 55%",
      "--chart-5": "15 90% 65%",
    }
  },

  // WARM THEMES (4)
  sunset: {
    name: "Sunset",
    description: "Golden hour warmth",
    category: 'warm' as ThemeCategory,
    colors: {
      "--chart-1": "48 96% 53%",
      "--chart-2": "43 96% 56%",
      "--chart-3": "38 92% 50%",
      "--chart-4": "32 95% 44%",
      "--chart-5": "25 95% 39%",
    }
  },
  fire: {
    name: "Fire",
    description: "Blazing reds and oranges",
    category: 'warm' as ThemeCategory,
    colors: {
      "--chart-1": "15 90% 50%",
      "--chart-2": "25 85% 55%",
      "--chart-3": "35 80% 60%",
      "--chart-4": "5 95% 45%",
      "--chart-5": "45 75% 65%",
    }
  },
  autumn: {
    name: "Autumn",
    description: "Fall foliage colors",
    category: 'warm' as ThemeCategory,
    colors: {
      "--chart-1": "25 70% 45%",
      "--chart-2": "35 75% 50%",
      "--chart-3": "15 80% 40%",
      "--chart-4": "45 65% 55%",
      "--chart-5": "55 60% 60%",
    }
  },
  spice: {
    name: "Spice",
    description: "Rich culinary tones",
    category: 'warm' as ThemeCategory,
    colors: {
      "--chart-1": "20 60% 40%",
      "--chart-2": "30 55% 35%",
      "--chart-3": "40 65% 45%",
      "--chart-4": "10 70% 30%",
      "--chart-5": "50 50% 50%",
    }
  },

  // COOL THEMES (4)
  arctic: {
    name: "Arctic",
    description: "Icy blues and whites",
    category: 'cool' as ThemeCategory,
    colors: {
      "--chart-1": "195 70% 80%",
      "--chart-2": "200 65% 75%",
      "--chart-3": "205 60% 70%",
      "--chart-4": "190 55% 85%",
      "--chart-5": "210 50% 65%",
    }
  },
  midnight: {
    name: "Midnight",
    description: "Deep night blues",
    category: 'cool' as ThemeCategory,
    colors: {
      "--chart-1": "220 60% 25%",
      "--chart-2": "230 55% 30%",
      "--chart-3": "210 65% 20%",
      "--chart-4": "240 50% 35%",
      "--chart-5": "200 45% 40%",
    }
  },
  teal: {
    name: "Teal",
    description: "Blue-green harmony",
    category: 'cool' as ThemeCategory,
    colors: {
      "--chart-1": "180 50% 40%",
      "--chart-2": "175 55% 45%",
      "--chart-3": "185 45% 35%",
      "--chart-4": "170 60% 50%",
      "--chart-5": "190 40% 30%",
    }
  },
  glacier: {
    name: "Glacier",
    description: "Pristine ice tones",
    category: 'cool' as ThemeCategory,
    colors: {
      "--chart-1": "190 40% 60%",
      "--chart-2": "200 35% 65%",
      "--chart-3": "180 45% 55%",
      "--chart-4": "210 30% 70%",
      "--chart-5": "170 50% 50%",
    }
  },

  // RETRO THEMES (4)
  vintage: {
    name: "Vintage",
    description: "Classic muted tones",
    category: 'retro' as ThemeCategory,
    colors: {
      "--chart-1": "25 30% 50%",
      "--chart-2": "200 25% 45%",
      "--chart-3": "40 35% 55%",
      "--chart-4": "180 20% 40%",
      "--chart-5": "60 25% 60%",
    }
  },
  retro80s: {
    name: "Retro 80s",
    description: "Neon decade vibes",
    category: 'retro' as ThemeCategory,
    colors: {
      "--chart-1": "300 80% 60%",
      "--chart-2": "180 75% 55%",
      "--chart-3": "60 85% 65%",
      "--chart-4": "320 70% 70%",
      "--chart-5": "200 65% 50%",
    }
  },
  sepia: {
    name: "Sepia",
    description: "Nostalgic photo tones",
    category: 'retro' as ThemeCategory,
    colors: {
      "--chart-1": "35 40% 40%",
      "--chart-2": "30 35% 45%",
      "--chart-3": "25 45% 35%",
      "--chart-4": "40 30% 50%",
      "--chart-5": "20 50% 30%",
    }
  },
  pastel: {
    name: "Pastel",
    description: "Soft vintage colors",
    category: 'retro' as ThemeCategory,
    colors: {
      "--chart-1": "340 40% 80%",
      "--chart-2": "200 35% 75%",
      "--chart-3": "60 45% 85%",
      "--chart-4": "280 30% 70%",
      "--chart-5": "120 25% 90%",
    }
  },

  // MONOCHROME THEMES (3)
  monochrome: {
    name: "Monochrome",
    description: "Pure black and white",
    category: 'monochrome' as ThemeCategory,
    colors: {
      "--chart-1": "0 0% 20%",
      "--chart-2": "0 0% 35%",
      "--chart-3": "0 0% 50%",
      "--chart-4": "0 0% 65%",
      "--chart-5": "0 0% 80%",
    }
  },
  grayscale: {
    name: "Grayscale",
    description: "Refined gray spectrum",
    category: 'monochrome' as ThemeCategory,
    colors: {
      "--chart-1": "210 5% 25%",
      "--chart-2": "200 3% 40%",
      "--chart-3": "220 4% 55%",
      "--chart-4": "190 2% 70%",
      "--chart-5": "230 1% 85%",
    }
  },
  charcoal: {
    name: "Charcoal",
    description: "Deep dark grays",
    category: 'monochrome' as ThemeCategory,
    colors: {
      "--chart-1": "0 0% 15%",
      "--chart-2": "0 0% 25%",
      "--chart-3": "0 0% 35%",
      "--chart-4": "0 0% 45%",
      "--chart-5": "0 0% 55%",
    }
  },

  // GRADIENT THEMES (3)
  sunset_gradient: {
    name: "Sunset Gradient",
    description: "Flowing warm transition",
    category: 'gradient' as ThemeCategory,
    colors: {
      "--chart-1": "15 90% 60%",
      "--chart-2": "25 85% 55%",
      "--chart-3": "35 80% 50%",
      "--chart-4": "45 75% 45%",
      "--chart-5": "55 70% 40%",
    }
  },
  ocean_gradient: {
    name: "Ocean Gradient",
    description: "Deep to light blue flow",
    category: 'gradient' as ThemeCategory,
    colors: {
      "--chart-1": "200 80% 30%",
      "--chart-2": "205 75% 40%",
      "--chart-3": "210 70% 50%",
      "--chart-4": "215 65% 60%",
      "--chart-5": "220 60% 70%",
    }
  },
  rainbow_gradient: {
    name: "Rainbow Gradient",
    description: "Spectrum color flow",
    category: 'gradient' as ThemeCategory,
    colors: {
      "--chart-1": "0 70% 50%",
      "--chart-2": "72 65% 55%",
      "--chart-3": "144 60% 60%",
      "--chart-4": "216 55% 65%",
      "--chart-5": "288 50% 70%",
    }
  },
};

type ThemeKey = keyof typeof themeConfigs;

interface ThemeConfig {
  name: string;
  description: string;
  category: ThemeCategory;
  colors: Record<string, string>;
}

interface ThemeContextType {
  currentTheme: ThemeKey;
  setTheme: (theme: ThemeKey) => void;
  themes: Record<ThemeKey, ThemeConfig>;
  categories: Record<ThemeCategory, { name: string; description: string }>;
  getThemesByCategory: (category: ThemeCategory) => Array<{ key: ThemeKey; config: ThemeConfig }>;
  searchThemes: (query: string) => Array<{ key: ThemeKey; config: ThemeConfig }>;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [currentTheme, setCurrentTheme] = useState<ThemeKey>('default');

  // Apply theme to DOM
  const applyTheme = (themeKey: ThemeKey) => {
    const theme = themeConfigs[themeKey];
    if (!theme) return;
    
    const root = document.documentElement;
    Object.entries(theme.colors).forEach(([property, value]) => {
      root.style.setProperty(property, value);
    });
  };

  // Load saved theme on mount
  useEffect(() => {
    const savedTheme = localStorage.getItem('chart-theme') as ThemeKey;
    if (savedTheme && themeConfigs[savedTheme]) {
      setCurrentTheme(savedTheme);
      applyTheme(savedTheme);
    } else {
      // Apply default theme on first load
      applyTheme('default');
    }
  }, []);

  const setTheme = (theme: ThemeKey) => {
    setCurrentTheme(theme);
    applyTheme(theme);
    localStorage.setItem('chart-theme', theme);
  };

  // Get themes by category
  const getThemesByCategory = (category: ThemeCategory) => {
    return Object.entries(themeConfigs)
      .filter(([_, config]) => config.category === category)
      .map(([key, config]) => ({ key: key as ThemeKey, config }));
  };

  // Search themes by name or description
  const searchThemes = (query: string) => {
    if (!query.trim()) return [];
    
    const lowercaseQuery = query.toLowerCase();
    return Object.entries(themeConfigs)
      .filter(([, config]) => 
        config.name.toLowerCase().includes(lowercaseQuery) ||
        config.description.toLowerCase().includes(lowercaseQuery) ||
        categoryInfo[config.category].name.toLowerCase().includes(lowercaseQuery)
      )
      .map(([key, config]) => ({ key: key as ThemeKey, config }));
  };

  return (
    <ThemeContext.Provider value={{ 
      currentTheme, 
      setTheme, 
      themes: themeConfigs,
      categories: categoryInfo,
      getThemesByCategory,
      searchThemes
    }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (context === undefined) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return context;
}

// Category information for UI organization
const categoryInfo: Record<ThemeCategory, { name: string; description: string }> = {
  popular: { name: 'Popular', description: 'Most loved themes' },
  nature: { name: 'Nature', description: 'Earth and organic tones' },
  corporate: { name: 'Corporate', description: 'Professional business colors' },
  vibrant: { name: 'Vibrant', description: 'Bold and energetic' },
  warm: { name: 'Warm', description: 'Cozy and inviting' },
  cool: { name: 'Cool', description: 'Calm and refreshing' },
  retro: { name: 'Retro', description: 'Vintage and nostalgic' },
  monochrome: { name: 'Monochrome', description: 'Timeless grayscale' },
  gradient: { name: 'Gradient', description: 'Smooth color transitions' },
};

export { themeConfigs, categoryInfo };
export type { ThemeKey, ThemeCategory };