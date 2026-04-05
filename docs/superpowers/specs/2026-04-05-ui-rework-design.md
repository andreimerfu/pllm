# pLLM UI Rework — Teal Studio Design Spec

**Date:** 2026-04-05
**Status:** Approved
**Scope:** Full frontend redesign of all 22 pages

## Overview

Complete visual rework of the pLLM admin UI from its current Radix/Lucide/38-theme system to a refined developer console aesthetic inspired by Stripe and Supabase. Single teal-based theme with dark + light mode parity. All UI icons replaced with Iconify Solar, all brand/provider references use Iconify logos.

## Design Direction: Teal Studio

Balanced, refined, Stripe-inspired polish. Neutral gray foundations with teal (#0D9488 / #14B8A6) used strategically — navigation highlights, CTAs, status accents, focus rings. Clean sans-serif throughout (Inter). Professional without being corporate.

## Design System Foundation

### Color Palette

**Teal Primary Scale (Tailwind teal, shared across modes):**

| Token       | Value     | Usage                              |
|-------------|-----------|-------------------------------------|
| teal-50     | `#F0FDFA` | Light backgrounds, hover fills      |
| teal-100    | `#CCFBF1` | Light mode badges, subtle fills     |
| teal-200    | `#99F6E4` | Light mode active backgrounds       |
| teal-300    | `#5EEAD4` | Light accents                       |
| teal-400    | `#2DD4BF` | Dark mode text accent               |
| teal-500    | `#14B8A6` | Primary — buttons, active nav, focus rings (dark mode) |
| teal-600    | `#0D9488` | Primary — buttons, active nav (light mode) |
| teal-700    | `#0F766E` | Hover states on primary             |
| teal-800    | `#115E59` | Card hover borders (dark)           |
| teal-900    | `#134E4A` | Deep accents                        |

**Dark Mode:**

| Token            | Value     | Usage                     |
|------------------|-----------|---------------------------|
| background       | `#030712` | Page background (gray-950)|
| surface          | `#111827` | Cards, sidebar (gray-900) |
| surface-elevated | `#1F2937` | Table headers, dropdowns (gray-800) |
| border           | `#374151` | Borders (gray-700)        |
| text-primary     | `#F9FAFB` | Headings, values (gray-50)|
| text-secondary   | `#D1D5DB` | Body text (gray-300)      |
| text-muted       | `#9CA3AF` | Labels, captions (gray-400)|
| text-faint       | `#6B7280` | Placeholders (gray-500)   |

**Light Mode:**

| Token            | Value     | Usage                     |
|------------------|-----------|---------------------------|
| background       | `#F9FAFB` | Page background (gray-50) |
| surface          | `#FFFFFF` | Cards, sidebar            |
| surface-elevated | `#F3F4F6` | Table headers (gray-100)  |
| border           | `#E5E7EB` | Borders (gray-200)        |
| text-primary     | `#111827` | Headings (gray-900)       |
| text-secondary   | `#374151` | Body text (gray-700)      |
| text-muted       | `#6B7280` | Labels (gray-500)         |
| text-faint       | `#9CA3AF` | Placeholders (gray-400)   |

**Semantic Colors (both modes):**

| Token      | Value     | Usage                    |
|------------|-----------|--------------------------|
| success    | `#10B981` | Active, healthy, on track|
| warning    | `#F59E0B` | Degraded, near limit     |
| error      | `#EF4444` | Down, over budget, errors|
| info       | `#3B82F6` | Informational badges     |

### Typography

- **Primary font:** Inter (sans-serif) — all UI text
- **Monospace font:** JetBrains Mono — code, model IDs, API keys, metrics, latency values, costs, token counts
- **Scale:**
  - Page titles: 24-28px, bold, -0.02em letter-spacing
  - Section headings: 20px, semibold
  - Card/table headers: 14px, semibold
  - Body text: 13-14px, medium
  - Labels/captions: 11-12px, regular, muted color
  - Buttons: 13-14px, medium
  - Group labels (sidebar): 11px, semibold, uppercase, 0.05em letter-spacing

### Spacing Scale

4px (inline gaps) → 8px (tight) → 12px (component padding) → 16px (card padding) → 24px (section gaps) → 32px (page padding)

### Border Radius

- 4px — buttons, inputs, badges, small tags
- 6px — nav items, filter pills, inline controls
- 8px — cards, dropdowns, table containers
- 12px — modals, panels, large containers
- 9999px (full) — pills, avatars, status dots, toggle tracks

### Elevation & Shadows

Dark mode uses border-first elevation (1px solid borders at different gray levels). Shadows reserved for floating elements:

- **Level 0:** 1px border only — cards, surfaces
- **Level 1:** Subtle shadow — `0 1px 3px rgba(0,0,0,0.3)` — hover cards
- **Level 2:** Medium — `0 4px 12px rgba(0,0,0,0.3)` — dropdowns, popovers
- **Level 3:** Heavy — `0 8px 24px rgba(0,0,0,0.4)` — modals, dialogs
- **Focus ring:** `0 0 0 3px rgba(20,184,166,0.15)` + teal-500 border — all interactive elements

Light mode uses the same shadow scale with reduced opacity.

## Icon Strategy

### Solar Icons (UI Chrome)

All navigation, action, status, and decorative icons use the Iconify Solar icon set via `@iconify/react`:

```tsx
import { Icon } from '@iconify/react';
<Icon icon="solar:widget-bold" />  // Dashboard
<Icon icon="solar:chat-round-line-linear" />  // Chat
<Icon icon="solar:cpu-bolt-linear" />  // Models
<Icon icon="solar:route-linear" />  // Routes
<Icon icon="solar:key-linear" />  // API Keys
<Icon icon="solar:users-group-rounded-linear" />  // Teams
<Icon icon="solar:user-linear" />  // Users
<Icon icon="solar:wallet-linear" />  // Budget
<Icon icon="solar:document-text-linear" />  // Audit Logs
<Icon icon="solar:shield-check-linear" />  // Guardrails
<Icon icon="solar:settings-linear" />  // Settings
```

Remove `lucide-react` entirely from the project.

### Brand Logos (Real-World References)

Provider logos and any reference to real products use the Iconify `logos:` collection, rendered in their original colors:

```tsx
<Icon icon="logos:openai-icon" />      // OpenAI
<Icon icon="logos:anthropic-icon" />   // Anthropic (if available, else custom)
<Icon icon="logos:azure-icon" />       // Azure
<Icon icon="logos:aws" />              // AWS/Bedrock
<Icon icon="logos:google-cloud" />     // Vertex AI / Google
<Icon icon="logos:cohere-icon" />      // Cohere (if available)
```

These appear in: model cards, model detail headers, route flow nodes, provider filter pills, model selector dropdowns, table rows.

## Layout Structure

### App Shell

```
┌─────────────────────────────────────────────────┐
│ Sidebar (240px)  │  Header Bar (52px)           │
│                  │  ┌─────────────────────────┐  │
│  Logo            │  │ Breadcrumb    ⌘K  ☀ v2 │  │
│  ─────────────   │  └─────────────────────────┘  │
│  Core            │                               │
│    Dashboard     │  Page Content                 │
│    Chat          │  ┌─────────────────────────┐  │
│    Models        │  │ Title + Subtitle        │  │
│    Routes        │  │                         │  │
│  Management      │  │ Content Grid            │  │
│    API Keys      │  │                         │  │
│    Teams         │  │                         │  │
│    Users         │  │                         │  │
│  Administration  │  │                         │  │
│    Budget        │  │                         │  │
│    Audit Logs    │  │                         │  │
│    Guardrails    │  │                         │  │
│    Settings      │  │                         │  │
│  ─────────────   │  │                         │  │
│  User Footer     │  └─────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

### Sidebar

- Width: 240px expanded, icon-only collapsed (~48px)
- Background: surface (#111827 dark / #FFFFFF light)
- Border-right: 1px solid border color
- Logo: Teal gradient square (135deg, teal-500 → teal-600) with "P" monogram + "pLLM" text + "Gateway Console" subtitle
- Nav groups: Core, Management, Administration — each with uppercase label
- Active item: teal background tint (`rgba(20,184,166,0.1)`) + teal icon + teal text
- Inactive item: muted icon + muted text, hover shows subtle background
- User footer: Avatar circle (teal bg, white initials), name, role, dropdown chevron
- Mobile: drawer overlay (18rem wide)
- Toggle: keyboard shortcut `B`

### Header Bar

- Height: 52px, sticky top
- Background: page background color
- Border-bottom: 1px solid border color
- Left: Breadcrumb navigation (muted section / bold current page)
- Right: Command palette trigger (`⌘K` shortcut badge), theme toggle (sun/moon), version badge (monospace, pill)

### Page Layout Pattern

Every page follows:
1. Page header: Title (24px bold) + subtitle (13px muted) + optional action buttons (right-aligned)
2. Optional filter bar with search input + filter pills
3. Content area: responsive grid with 16-24px gaps

## Component Design

### Buttons

| Variant      | Dark Mode Style                                              |
|-------------|--------------------------------------------------------------|
| Primary     | bg: teal-600, text: white, hover: teal-700                  |
| Secondary   | bg: gray-800, text: gray-200, border: gray-700              |
| Outline     | bg: transparent, text: gray-300, border: gray-700            |
| Ghost       | bg: transparent, text: gray-300, hover: subtle bg            |
| Destructive | bg: red-900, text: red-300, border: red-800                  |
| Destructive Ghost | bg: transparent, text: red-300                        |

Sizes: Small (6px 12px, 12px text), Default (8px 16px, 13px text), Large (10px 20px, 14px text).
Icon support: left-aligned icon + text, or icon-only square button.

### Form Inputs

- Background: #030712 (dark) / #FFFFFF (light)
- Border: 1px solid gray-700/gray-200
- Focus: teal-500 border + teal focus ring glow
- Error: red border + red focus ring + red helper text
- Labels: 13px medium, above input with 6px gap
- Helper text: 11px, muted, below input with 4px gap
- Toggles: teal track when on, gray when off

### Badges & Tags

- **Status badges:** Pill shape (full radius), dot indicator + text, colored background tint (e.g., `rgba(16,185,129,0.1)` for active green)
- **Provider tags:** 4px radius, gray-800 bg, gray-300 text, 1px gray-700 border
- **Capability tags:** 4px radius, teal tint bg, teal text
- **Monospace labels:** Code font, #030712 bg, teal text for model IDs, gray for API keys

### Data Tables

- Container: surface bg, 1px border, 8px radius
- Toolbar: filter input + column filters + primary CTA button
- Header row: surface-elevated bg, uppercase 11px labels
- Data rows: 1px border-bottom, 10px vertical padding
- Hover: subtle background change
- Pagination: bottom bar with count + numbered buttons (teal active)
- Actions: three-dot menu icon per row

### Content Cards

- Background: surface, 1px border, 8px radius, 16px padding
- Top: icon (36px, teal tint square) or provider logo + status badge
- Middle: title (14px semibold) + subtitle/ID (monospace)
- Optional: capability tags row
- Footer: border-top separator, 3-column metric grid (label 10px muted, value 12-13px monospace)
- Hover: border transitions to teal-800

### Dialogs/Modals

- Overlay: `rgba(0,0,0,0.6)`
- Container: surface bg, 1px border, 12px radius, max-width 420px
- Header: title (16px semibold) + description (13px muted)
- Footer: surface-elevated bg, border-top, right-aligned buttons
- Destructive dialogs: model name in teal monospace code tag

### Empty States

- Centered layout, 48px padding
- Icon: 48px teal tint square with Solar icon
- Title: 15px semibold
- Description: 13px muted
- CTA: primary button with plus icon

## Page Designs

### Dashboard

- 4-column stat cards: Total Requests, Avg Latency, Active Models, Error Rate
- Each card: label, monospace value, trend indicator (green/red percentage)
- Below: 2-column grid — Request Volume chart (teal gradient bars, period toggle 24h/7d/30d) + Provider Breakdown (horizontal progress bars with percentages)

### Models

- **Card view:** 3-column grid. Each card: provider logo (32px square) + model name + monospace ID, capability tags, metric footer (latency, requests, errors). Provider filter pills in toolbar with real logos.
- **Table view:** Columns: Model (name + ID), Provider (logo tag), Status (dot badge), Latency (monospace), Actions (three-dot). Filter bar + search.
- **View toggle:** Cards/Table persisted in user preferences.
- **Model detail:** Back nav, large provider logo (44px) + title + status badge, 5-stat row (requests, latency, errors, tokens, cost), two-column layout: charts left (latency distribution with p50/p95/p99, request volume) + configuration panel right (provider, model ID, max tokens, pricing, capabilities, fallback chain with arrow notation).

### Routes

- **List view:** Vertical card list. Each card: route icon + name + monospace endpoint, strategy label (teal), model count, mini flow preview (provider logo chain with arrows), status badge, chevron.
- **Detail view:** Back nav, title + status + endpoint, strategy selector + save button. Main area: React Flow canvas.

### Routes — React Flow Integration

- **Canvas:** Dark background (#080C14), subtle dot grid pattern (`radial-gradient`, 24px spacing, gray-800 dots)
- **Entry node:** Surface bg, 2px teal border, teal glow shadow. Contains entry icon + endpoint path.
- **Model nodes:** Surface bg, 1px gray-800 border. Contains provider logo (28px) + model name + priority/fallback label + latency. Capability tags below. Primary model full opacity, fallbacks progressively dimmed (0.75, 0.55).
- **Handles:** 12px circles. Teal for connected, gray-700 for available.
- **Edges:** Solid teal (opacity 0.6) for primary connection, dashed gray for fallback paths. Bezier curves.
- **Controls:** Zoom +/−, fit-view — styled with surface bg + border. Bottom-left.
- **Minimap:** 120x80px, surface bg + border. Bottom-right.
- **Add action:** Dashed border button top-right "Add Model to Route".
- **Config panels below canvas:** 3-column grid — Strategy (name + description), Health Check (interval, timeout, threshold), Traffic (requests, fallback rate, latency).

### Chat

- Compact testing interface, not a chat product.
- Header: title + model selector dropdown (provider logo + name) + parameters toggle + clear button.
- Messages: User messages right-aligned (teal bg, white text, rounded), assistant messages left-aligned (surface card bg, border, gray text). Provider logo avatar on assistant messages.
- Response metadata: Below each assistant message — latency, token count, cost (all monospace 10px), copy link.
- Input bar: Bottom pinned, dark bg (#080C14), surface input with placeholder, teal send button.

### Budget

- 4-stat summary: Total Spend MTD (with progress bar showing % of budget), Daily Average, Projected EOM (warning color if over), Active Budgets (alert count).
- Breakdown table: Toggle between Team/Model/Key views. Columns: name, spend (monospace), progress bar, % of budget, status (On track/Near limit/Over). Progress bars use teal for healthy, warning for near limit.

### Keys

- Table view with toolbar (search + create button).
- Columns: Name, Key (masked monospace `sk-...a8f2`), Team, Created, Last Used, Status, Actions.
- Create dialog: form with name, team selector, expiration, permissions.

### Teams

- Card grid. Each card: team icon + name + description, member count, budget progress bar, metric footer (budget, used %, key count).
- Create/edit modals with member management.

### Users

- Simple table: Avatar (initials circle), Name, Email, Role badge, Teams, Last Active.

### Audit Logs

- Dense table with filter bar: date range picker, event type filter, user filter, search.
- Columns: Timestamp (monospace), User, Event, Resource, Status badge.
- Expandable rows: JSON detail viewer with syntax highlighting.

### Guardrails

- Card grid: shield icon + name + description, enable/disable toggle, status badge, metric footer (scanned, blocked, latency).
- **Config page:** Multi-step form wizard (same input/toggle styling), preview panel.
- **Marketplace:** Card grid with category filter pills, install buttons.

### Settings (Simplified)

- Single-column form layout (max-width 640px).
- Sections as cards:
  - **Appearance:** Light/Dark/System segmented toggle. No theme gallery.
  - **Gateway:** Default timeout input, request logging toggle, streaming toggle.
  - **Authentication:** OIDC provider info, master key status.
  - **Danger Zone:** Red border card, reset button.

## Theme System Changes

**Remove entirely:**
- `ThemeContext.tsx` with 38 theme definitions
- Theme gallery in Settings page
- All theme-related CSS variables and switching logic
- `data-theme` attribute system

**Replace with:**
- Simple `ColorModeContext` with three states: `light`, `dark`, `system`
- CSS variables defined once in `index.css` for light and dark via `prefers-color-scheme` media query and a `.dark` / `.light` class override
- Persisted to `localStorage` as a string (`"light"`, `"dark"`, `"system"`)

## Dependencies Changes

**Remove:**
- `lucide-react` — replaced by Iconify Solar

**Keep:**
- `@iconify/react` — already installed, used for both Solar UI icons and brand logos
- All Radix UI primitives — restyle, don't replace
- `recharts` / `echarts` — restyle charts with teal palette
- `@xyflow/react` — restyle nodes/edges/canvas
- All other deps unchanged

**Add:**
- `@fontsource/inter` — Inter font (or load via Google Fonts)
- `@fontsource/jetbrains-mono` — JetBrains Mono (or load via Google Fonts)

## Migration Approach

This is a visual-only rework. No routing changes, no API changes, no new features, no backend changes. The React component tree structure stays the same — we're restyling every component and replacing icons.

Key migration steps:
1. Update design tokens (CSS variables in `index.css`)
2. Replace `ThemeContext` with `ColorModeContext`
3. Replace all Lucide imports with Solar Icon equivalents
4. Restyle Radix primitives (button, input, badge, etc.)
5. Restyle sidebar, header, layout shell
6. Restyle each page one by one
7. Restyle React Flow nodes/edges/canvas
8. Restyle charts with teal palette
9. Remove unused theme code

## Visual Mockups

Interactive mockups for this spec are saved in `.superpowers/brainstorm/` and include:
- `design-approaches.html` — 3 direction options (Teal Studio selected)
- `design-system.html` — Color palette, typography, spacing, elevation
- `layout-navigation.html` — Full app shell dark + light mode
- `components.html` — Buttons, inputs, badges, tables, cards, dialogs, empty states
- `page-designs-models.html` — Models list (card view) + model detail
- `page-designs-routes.html` — Routes list + route detail with React Flow
- `page-designs-remaining.html` — Chat, Budget, Settings
