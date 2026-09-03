---
name: web-architecture
description: "Feature-based structure of the crm-deal-web React + Vite app — app, features and shared, and what belongs in each. Use before creating files in the web repository, or when deciding where a component, hook, store or schema goes."
---

# Web architecture

React + Vite, feature-based. Talks only to the API Gateway.

## Structure

```
crm-deal-web/
└─ src/
   ├─ main.tsx
   ├─ app/                    # providers, router (routes by permission)
   ├─ features/
   │  └─ orders/              # auth, customers, dashboard, …
   │     ├─ api/              # TanStack Query hooks
   │     ├─ components/
   │     ├─ hooks/
   │     ├─ store/            # zustand
   │     ├─ schemas/          # zod, shared with mobile
   │     └─ types/            # enums
   └─ shared/
      ├─ ui/                  # design system (Button, Table)
      ├─ lib/http/            # axios + 401→refresh interceptor
      ├─ lib/query/           # QueryClient + persistence
      ├─ lib/realtime/        # SSE (EventSource) + Socket.IO
      ├─ hooks/
      ├─ utils/               # debounce, mediaQuery
      └─ config/              # import.meta.env, no secrets
```

## Features own their slice

A feature is a business area — `orders`, `auth`, `customers`, `dashboard` — and it owns everything about itself: its queries, components, hooks, state, schemas and types.

Code moves to `shared/` when a second feature needs it, not before.

## Shared

- `ui/` — the design system. See the `web-ui` skill for the catalog.
- `lib/http/` — axios, with the interceptor that refreshes on a 401.
- `lib/query/` — the QueryClient and its persistence.
- `lib/realtime/` — SSE via EventSource, and Socket.IO.
- `config/` — reads `import.meta.env`. **No secrets ever reach the bundle.**

## Data

**TanStack Query owns server state**: deduplication and stale-while-revalidate come from it. Do not cache server responses in zustand.

**zustand owns client state** — what the UI is doing, not what the server said.

**The web app has no webhooks.** Changes arrive over SSE or WebSocket.

## Tokens

The access token lives **in memory**, with a refresh token alongside it. Nothing secret is in the bundle.

## Only the Gateway

The web app talks to the API Gateway and nothing else. Never a bank, never SAP, never a microservice directly.

## What NOT to do

- **Do not put a secret in `config/` or anywhere else in the bundle.** Everything shipped to the browser is public.
- **Do not call an external service directly.** Everything goes through the Gateway.
- **Do not duplicate a schema as a TypeScript type.** The Zod schema is the source of both.
- **Do not reach into another feature's folder.** If two features need the same thing, it belongs in `shared/`.
- **Do not mirror server data into zustand.** That is what TanStack Query is for.
