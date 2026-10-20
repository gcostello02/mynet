# M3.2 dashboard design system

## Visual direction

Dark-first admin UI with CSS variable tokens in `web/admin/src/index.css`. Light mode toggles the same tokens via `html.light`. Accent: sky (`#0ea5e9` dark / `#0284c7` light). Surfaces use layered neutrals with subtle borders — no decorative gradients or chart widgets.

## Typography

- UI: system sans stack
- IDs, fingerprints, tokens: monospace
- Page titles: `text-xl`–`2xl`, semibold
- Metadata: `text-xs` with muted foreground

## Spacing & layout

- Shell: fixed `w-56` sidebar (desktop), slide-over nav (mobile)
- Content max readable width on settings; tables full width
- Card padding `p-4` / `p-5`, radius `0.75rem`

## Components (`src/components/ui/`)

| Component | Use |
|-----------|-----|
| `Button` | primary / secondary / ghost / danger |
| `Input` | labeled fields |
| `Badge` | status chips (never “VPN connected”) |
| `Card` | sections and metrics |
| `Modal` | create invitation, revoke confirm, token reveal |
| `Drawer` | device details |
| `Skeleton` | loading placeholders |
| `EmptyState` | zero-data views |
| `CopyButton` | fingerprint / URL / enrollment string |
| `PageHeader` | title + actions |
| `ToastProvider` | mutation feedback |

## Data honesty

- **Signaling online/offline** only — always labeled as signaling
- Overview “recent devices” = enrollment sort from API
- No bandwidth, latency, or tunnel health metrics

## Preview

```bash
cd web/admin && npm ci && npm run dev
# Proxies /v1 to localhost:8080 — coordinator must be running for live data
```

Production embed: `make build-dashboard && make build`

## Premium redesign (M3.3)

Visual-first pass over the whole dashboard; no API, auth, or backend changes.

- **System:** zinc surface ladder (canvas/panel/raised/hover) with 1px borders, one sky accent for interactive elements, green/amber/red reserved for real states, 4–10px radii, tabular numerals for IPs/metrics, monospace for identifiers, Lucide icons. Dark-first; light mode overrides the same CSS variables.
- **Shell:** fixed 224px sidebar, compact top bar (breadcrumb, fingerprint chip, coordinator health, user menu with appearance + sign-out), mobile nav dialog.
- **Overview:** one stats strip (registered / authorized / signaling online / revoked), a membership map, recent enrollments, enrollment mode + open invitations. The map shows *enrollment* only; its caption states dotted lines are not live tunnels, and node labels say "signaling", never "connected".
- **Devices:** dense table with search (`/` focuses), All/Authorized/Revoked filter, row menu, inspector drawer (`?device=<id>` deep link), type-the-name revoke confirmation, cards on mobile.
- **Invitations:** create dialog (expiry presets, max uses, live summary), one-time reveal (explains hash-only storage, held in component state only), `ConfirmDialog` for revoke. No `window.confirm`.
- **Accessibility:** `useDialog` traps focus, handles Escape, locks scroll, restores focus to the opener; menus support arrow keys.
- **Storage:** only the appearance preference is stored in `localStorage`. Secrets and tokens never are.

Screenshots: `docs/screenshots/m3-redesign/`. Local preview used a throwaway coordinator with its own database.
