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
