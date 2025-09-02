import React, { createContext, useContext, useEffect, useState } from 'react';

// Theme configurations
const themeConfigs = {
  default: {
    name: "Default",
    colors: {
      "--chart-1": "220 70% 50%",
      "--chart-2": "160 60% 45%",
      "--chart-3": "30 80% 55%",
      "--chart-4": "280 65% 60%",
      "--chart-5": "340 75% 55%",
    }
  },
  green: {
    name: "Green",
    colors: {
      "--chart-1": "142 76% 36%",
      "--chart-2": "138 76% 32%",
      "--chart-3": "134 76% 28%",
      "--chart-4": "130 76% 24%",
      "--chart-5": "126 76% 20%",
    }
  },
  blue: {
    name: "Blue",
    colors: {
      "--chart-1": "221 83% 53%",
      "--chart-2": "212 90% 48%",
      "--chart-3": "204 96% 43%",
      "--chart-4": "199 89% 48%",
      "--chart-5": "192 82% 51%",
    }
  },
  amber: {
    name: "Amber",
    colors: {
      "--chart-1": "48 96% 53%",
      "--chart-2": "43 96% 56%",
      "--chart-3": "38 92% 50%",
      "--chart-4": "32 95% 44%",
      "--chart-5": "25 95% 39%",
    }
  },
  rose: {
    name: "Rose",
    colors: {
      "--chart-1": "330 81% 60%",
      "--chart-2": "333 71% 51%",
      "--chart-3": "336 84% 57%",
      "--chart-4": "339 82% 52%",
      "--chart-5": "342 75% 47%",
    }
  },
  purple: {
    name: "Purple",
    colors: {
      "--chart-1": "263 70% 50%",
      "--chart-2": "250 84% 54%",
      "--chart-3": "240 100% 50%",
      "--chart-4": "229 84% 54%",
      "--chart-5": "217 70% 50%",
    }
  },
};

type ThemeKey = keyof typeof themeConfigs;

interface ThemeContextType {
  currentTheme: ThemeKey;
  setTheme: (theme: ThemeKey) => void;
  themes: typeof themeConfigs;
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

  return (
    <ThemeContext.Provider value={{ currentTheme, setTheme, themes: themeConfigs }}>
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

export { themeConfigs };
export type { ThemeKey };