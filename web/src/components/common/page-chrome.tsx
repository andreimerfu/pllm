import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

// PageChrome shares two slots between the current page and the top-bar:
//
//   actions     — the primary CTA(s) for the page (e.g. "New use case",
//                 "Publish server"). Rendered in the top-bar so page body
//                 starts with content rather than a header row of buttons.
//
//   entityCrumb — the last breadcrumb segment when the user has drilled
//                 into something (e.g. open drawer, selected route). Kept
//                 out of the URL so drawers without routing can still show
//                 the entity name in the crumb.
//
// Pages set these via the <PageActions/> and <PageEntityCrumb/> helpers;
// the top-bar reads them via usePageChrome().

type ChromeState = {
  actions: ReactNode | null;
  entityCrumb: string | null;
};

type ChromeAPI = ChromeState & {
  setActions: (node: ReactNode | null) => void;
  setEntityCrumb: (label: string | null) => void;
};

const PageChromeContext = createContext<ChromeAPI | null>(null);

export function PageChromeProvider({ children }: { children: ReactNode }) {
  const [actions, setActions] = useState<ReactNode | null>(null);
  const [entityCrumb, setEntityCrumb] = useState<string | null>(null);
  return (
    <PageChromeContext.Provider
      value={{ actions, entityCrumb, setActions, setEntityCrumb }}
    >
      {children}
    </PageChromeContext.Provider>
  );
}

export function usePageChrome(): ChromeAPI {
  const ctx = useContext(PageChromeContext);
  if (!ctx) throw new Error("usePageChrome must be used inside PageChromeProvider");
  return ctx;
}

// Renders nothing; registers its children in the chrome's actions slot
// for the lifetime of the mounted page. Safe to nest — last mount wins,
// earlier ones restore on unmount.
export function PageActions({ children }: { children: ReactNode }) {
  const { setActions } = usePageChrome();
  useEffect(() => {
    setActions(children);
    return () => setActions(null);
  }, [children, setActions]);
  return null;
}

// Same shape as PageActions but sets the trailing breadcrumb segment —
// the "entity" a user has drilled into (an open drawer, a selected row).
export function PageEntityCrumb({ label }: { label: string | null }) {
  const { setEntityCrumb } = usePageChrome();
  useEffect(() => {
    setEntityCrumb(label);
    return () => setEntityCrumb(null);
  }, [label, setEntityCrumb]);
  return null;
}
