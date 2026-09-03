---
name: mobile-offline
description: "Offline-first behaviour in crm-deal-mobile — creating records without a connection, the SQLite store and sync queue, encrypted token storage, and FCM push. Use when work has to survive losing connectivity, or when adding an action that must be available offline."
---

# Offline-first

The mobile app works without a connection. Losing signal is normal operation, not an error state.

## The flow

A salesperson creates an order with no internet. The order is written to **SQLite** (`expo-sqlite`), queued in `shared/offline/`, and **synced when the connection comes back**.

The user is never blocked waiting for the network to return.

## The sync queue

`shared/offline/` holds the SQLite store and the queue of pending operations.

An action that must work offline writes locally first and enqueues the sync. It does not call the API and hope.

## Tokens

Tokens are encrypted in **`expo-secure-store`**, never in plain AsyncStorage. Unlock is biometric or PIN, verified by the device — the CRM never sees the fingerprint, only the device's confirmation.

## Push

**FCM** is what replaces a webhook when the app is closed: the server notifies the device.

While the app is open, changes arrive over SSE or Socket.IO through `shared/realtime/`.

## What NOT to do

- **Do not block an action on the network** when it can be written locally and synced later.
- **Do not store tokens in plain AsyncStorage.**
- **Do not treat losing connectivity as an error.** It is the expected condition this app is built for.
