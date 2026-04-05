import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';

type ColorMode = 'light' | 'dark' | 'system';

interface ColorModeContextValue {
  colorMode: ColorMode;
  setColorMode: (mode: ColorMode) => void;
  resolvedMode: 'light' | 'dark';
}

const ColorModeContext = createContext<ColorModeContextValue | undefined>(undefined);

function getSystemPreference(): 'light' | 'dark' {
  if (typeof window === 'undefined') return 'dark';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function resolveMode(mode: ColorMode): 'light' | 'dark' {
  return mode === 'system' ? getSystemPreference() : mode;
}

export function ColorModeProvider({ children }: { children: ReactNode }) {
  const [colorMode, setColorModeState] = useState<ColorMode>(() => {
    const stored = localStorage.getItem('color-mode');
    if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
    return 'dark';
  });

  const [resolvedMode, setResolvedMode] = useState<'light' | 'dark'>(() => resolveMode(colorMode));

  const setColorMode = (mode: ColorMode) => {
    setColorModeState(mode);
    localStorage.setItem('color-mode', mode);
  };

  useEffect(() => {
    const resolved = resolveMode(colorMode);
    setResolvedMode(resolved);
    const root = document.documentElement;
    root.classList.remove('light', 'dark');
    root.classList.add(resolved);
  }, [colorMode]);

  useEffect(() => {
    if (colorMode !== 'system') return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = () => {
      const resolved = resolveMode('system');
      setResolvedMode(resolved);
      const root = document.documentElement;
      root.classList.remove('light', 'dark');
      root.classList.add(resolved);
    };
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [colorMode]);

  return (
    <ColorModeContext.Provider value={{ colorMode, setColorMode, resolvedMode }}>
      {children}
    </ColorModeContext.Provider>
  );
}

export function useColorMode() {
  const ctx = useContext(ColorModeContext);
  if (!ctx) throw new Error('useColorMode must be used within ColorModeProvider');
  return ctx;
}
