# MUI core migration — status and Tailwind removal proposal

This document accompanies the `feat/mui-core` batch. It records what was
migrated and the **separate, not-yet-applied** Tailwind removal step.

## Migrated to MUI in this batch

- `src/app/layout.js` — MUI emotion cache + theme moved to the root layout so
  every route (login, landing, dashboard) gets the 9Router palette.
- `src/shared/mui/MuiDashboardLayout.js` — now only owns the chrome; the
  providers it used to wrap were moved to the root layout.
- `src/app/error.js`
- `src/app/global-error.js`
- `src/app/(dashboard)/error.js`
- `src/app/login/page.js`
- `src/app/masuk/MasukClient.js`
- `src/app/callback/page.js`
- `src/app/dashboard/settings/pricing/page.js`
- `src/app/(dashboard)/dashboard/profile/page.js`
- `src/app/(dashboard)/dashboard/endpoint/**`
  (`EndpointPageClient`, `EndpointRow`, `SecurityWarning`, `StatusAlert`,
  `Tooltip`)
- `src/app/landing/**` (`page` + all components)
- `src/app/(dashboard)/dashboard/cli-tools/components/MitmServerCard.js`
- `src/app/(dashboard)/dashboard/cli-tools/components/MitmToolCard.js`

Material Symbols keep using the font class (`Icon` with
`className="material-symbols-outlined"`); no `@mui/icons-material` was added.

## Remaining Tailwind in this batch's scope

The CLI Tools page was removed in the fork, so most
`src/app/(dashboard)/dashboard/cli-tools/components/*` files are **orphaned
dead code** (nothing imports them except `index.js`). They still contain
Tailwind classes but are not in the build graph:

AntigravityToolCard, ClaudeToolCard, ClineToolCard, CodexToolCard,
CopilotToolCard, CoworkToolCard, DeepSeekTuiToolCard, DefaultToolCard,
DroidToolCard, GenericCliToolCard, GrokBuildToolCard, HermesToolCard,
JcodeToolCard, KiloToolCard, MitmLinkCard, OpenClawToolCard, OpenCodeToolCard,
ToolSummaryCard, ApiKeySelect, BaseUrlSelect, EndpointPresetControl,
CodexProfilesSection.

Only `MitmServerCard` and `MitmToolCard` are rendered (by the `mitm` page) and
were migrated. Recommendation: delete the orphaned files, or migrate them in a
follow-up batch if the CLI Tools screen is restored.

## Proposed Tailwind removal (apply LAST, after every component/screen batch)

Do **not** apply this until all other MUI batches have passed review.

1. `src/app/globals.css`
   - Remove `@import "tailwindcss";`
   - Remove `@custom-variant dark (&:where(.dark, .dark *));`
   - Remove the whole `@theme inline { ... }` block.
   - Move the two properties from that block that real CSS still needs into
     `:root`:
     ```css
     --color-accent: var(--color-brand-500); /* .landing-grid uses it */
     --font-sans: var(--font-inter), 'Inter', -apple-system, BlinkMacSystemFont,
       'SF Pro Text', 'SF Pro Display', system-ui, sans-serif; /* body uses it */
     ```
     The other `@theme` entries were only Tailwind utility generators and are no
     longer needed. Keep all `:root` / `.dark` 9Router tokens (colours, radii,
     `--shadow-soft`, `--shadow-warm`, `--shadow-elev`, `--shadow-elevated`) —
     the MUI theme/components reference them.
   - Keep `.material-symbols-outlined` and the Material Symbols font wiring.

2. `postcss.config.mjs` — remove the `@tailwindcss/postcss` plugin. With no
   plugins remaining the file can become:
   ```js
   export default { plugins: {} };
   ```
   (or be deleted).

3. `package.json` — remove `tailwindcss` and `@tailwindcss/postcss` from
   `devDependencies`.

4. Delete `src/app/material-symbols-subset.css` — confirmed unused (the icon
   font is loaded through `next/font/local` in `layout.js`).

Note: the `gitbook/` sub-project has its own Tailwind setup and is out of
scope for this app-level removal.
