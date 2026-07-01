# Verdex Frontend Architecture

## Overview

The Verdex frontend is a Next.js 14 (App Router) application written in TypeScript with
Tailwind CSS for styling. It serves as the judicial-facing UI for the Verdex platform,
providing case management, setup configuration, and display of AI-generated draft analyses.

All draft analyses produced by the system are non-binding and require review and sign-off
by a qualified judge before any legal use.

## Directory Structure

```
apps/web/
├── src/
│   ├── app/                    # Next.js App Router pages
│   │   ├── (auth)/login/       # Login page + LoginForm component
│   │   ├── (setup)/setup/      # 4-step setup wizard
│   │   │   └── steps/          # Wizard step components
│   │   ├── dashboard/          # Main dashboard
│   │   └── layout.tsx          # Root layout (Inter font, metadata)
│   ├── components/
│   │   ├── layout/             # AppShell, Sidebar, TopBar
│   │   └── ui/                 # Primitive UI components
│   ├── lib/
│   │   ├── api.ts              # Typed fetch wrapper (apiFetch)
│   │   ├── auth.ts             # Session helpers (sessionStorage)
│   │   └── hooks/              # useAuth, useSetup
│   └── types/
│       └── index.ts            # Shared TypeScript types
├── __tests__/                  # Component tests (Jest + Testing Library)
├── docs/
│   └── frontend-architecture.md
├── package.json
├── tsconfig.json
├── next.config.ts
├── tailwind.config.ts
└── postcss.config.mjs
```

## Technology Choices

| Concern | Choice | Rationale |
|---|---|---|
| Framework | Next.js 14 (App Router) | Server components, streaming, built-in routing |
| Language | TypeScript (strict) | Type safety across API boundaries |
| Styling | Tailwind CSS v3 | Utility-first, token-based design system |
| Forms | @tailwindcss/forms | Consistent baseline form styles |
| Icons | lucide-react | Consistent, tree-shakeable icon set |
| Testing | Jest + @testing-library/react | Component-level unit tests |

## Design Tokens

All design tokens are defined in `tailwind.config.ts`:

- **Primary (Judicial Blue)**: `#1e3a5f` — used for primary actions, active nav states
- **Accent (Gold)**: `#c9a84c` — used for logo, highlights, accents
- **Neutral Grays**: full scale from 50–900 for text and backgrounds
- **Font**: Inter (via Google Fonts, `next/font/google`)

## Authentication Flow

1. User navigates to `/` — root page checks `sessionStorage` via `getSession()`
2. If authenticated → redirect to `/dashboard`
3. If not → redirect to `/login`
4. `LoginForm` calls `POST /api/v1/auth/login` via `apiFetch()`
5. On success, `setSession()` persists `{ token, user }` to `sessionStorage`
6. The `useAuth` hook exposes `session`, `login()`, `logout()`, `isAuthenticated`

Session state is stored in `sessionStorage` (tab-scoped, cleared on close) rather than
`localStorage` to reduce XSS risk for judicial credentials.

## API Layer

`src/lib/api.ts` exports a single `apiFetch<T>()` function:

- Reads `NEXT_PUBLIC_API_URL` for the base URL (default: `http://localhost:8080`)
- Automatically injects `Authorization: Bearer <token>` from `getSession()`
- Parses error responses and throws `ApiError` with `status` and `message`
- Returns typed `Promise<T>` for all 2xx responses

## Setup Wizard

The wizard at `/setup` is implemented as a 4-step client component:

1. **Jurisdiction** — country + court level selection (fetches `/api/v1/jurisdictions`)
2. **Language** — multi-select for Arabic, Urdu, Tamil, English
3. **Provider** — AI provider type + endpoint URL + model ID
4. **Complete** — confirmation with disclaimer

State is managed by `useSetup()` hook; `submitSetup()` POSTs to `/api/v1/setup`.

## Layout System

- `AppShell` — top-level layout with sidebar + main area
- `Sidebar` — nav links; shows admin items only when user has `admin` role
- `TopBar` — user menu with logout + notifications bell

## Non-Binding Disclaimer

The `Disclaimer` component (`src/components/Disclaimer.tsx`) renders a prominent amber
banner that MUST appear on every page that displays AI-generated reasoning:

- Dashboard page (always visible)
- Setup wizard complete step
- Login page footer note
- Sidebar footer note

This is a platform-wide requirement. Any new page that surfaces AI output must import
and render `<Disclaimer />`.

## Testing

Tests live in `__tests__/` and use Jest with jsdom + `@testing-library/react`.

Run: `npm test` from `apps/web/`

Coverage targets:
- LoginForm — renders correctly, validates fields, calls API, handles errors, redirects
- StepIndicator — correct step states (complete/current/upcoming), aria attributes
