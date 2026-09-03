---
name: mobile-architecture
description: "Feature-based structure of the crm-deal-mobile React Native + Expo app — app, features and shared, including screens, secure storage, notifications and device access. Use before creating files in the mobile repository, or when deciding where a screen, hook, store or schema goes."
---

# Mobile architecture

React Native + Expo, feature-based and offline-first. Talks only to the API Gateway.

## Structure

```
crm-deal-mobile/
└─ src/
   ├─ app/                      # navigation (React Navigation)
   ├─ features/
   │  └─ orders/                # auth, customers, visits, …
   │     ├─ screens/
   │     ├─ components/
   │     ├─ hooks/
   │     ├─ api/                # TanStack Query
   │     ├─ store/              # zustand + persist
   │     └─ schemas/            # zod, shared with web
   └─ shared/
      ├─ ui/                    # React Native Paper
      ├─ lib/http/              # axios + 401→refresh interceptor
      ├─ lib/secure-store/      # expo-secure-store (encrypted tokens)
      ├─ notifications/         # FCM push
      ├─ realtime/              # react-native-sse / Socket.IO
      ├─ offline/               # expo-sqlite + sync queue
      ├─ device/                # camera, geolocation (expo)
      └─ config/
```

## Features own their slice

A feature is a business area — `orders`, `auth`, `customers`, `visits` — and it owns its screens, components, hooks, queries, state and schemas.

Screens live in the feature, not in a global `screens/` folder. Navigation lives in `app/`.

## UI

**React Native Paper.** The shadcn catalog is web only; nothing from `crm-deal-web/src/shared/ui` applies here.

## Tokens

Tokens go in **`expo-secure-store`**, encrypted — never in plain AsyncStorage. Unlock is biometric or PIN.

## Schemas are shared with web

The Zod schemas under `features/*/schemas/` are the same ones the web app uses. One schema validates and types on both platforms.

## Only the Gateway

The mobile app talks to the API Gateway and nothing else.

## What NOT to do

- **Do not store tokens in plain AsyncStorage.** They go in `expo-secure-store`.
- **Do not use the web design system.** Mobile is React Native Paper.
- **Do not call an external service directly.** Everything goes through the Gateway.
- **Do not reach into another feature's folder.** Shared code lives in `shared/`.
