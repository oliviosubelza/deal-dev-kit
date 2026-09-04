---
name: backend-connections
description: "Where every kind of inbound and outbound connection lives in a CRM DEAL NestJS microservice — cache, other services, external APIs, webhooks, realtime streams, events — and the rule each one follows. Use when integrating Redis, SNS/SQS, another DEAL service, a bank or SAP API, an incoming webhook, or a server push, and when deciding which layer a piece of integration code belongs in. Also covers Redis-backed rate limiting and the Fargate health checks."
---

# Connection rules

## The rule that decides every case

**If it talks to something outside the process, it is infrastructure. The domain only knows ports.**

A port is an interface declared in `domain/` and implemented in `infrastructure/`. The use case depends on the interface; it never imports a client, an SDK, or a driver. That is what keeps the core testable without a network, and what lets an adapter be replaced without touching a business rule.

The one exception is direction: something arriving from outside enters through `interface/`, not `infrastructure/`. Interface and infrastructure are siblings on opposite sides of the core, not layers wrapping each other.

## Where each connection lives

| Concern | Lives in | Rule |
| --- | --- | --- |
| Redis | `infrastructure/cache/` | Behind a `CachePort`. Also carries SQS event idempotency and the rate-limit counters. |
| Other DEAL microservices | `infrastructure/clients/` | Direct over internal DNS with a service token. **Never through the Gateway.** |
| External APIs (bank, SAP, Azure) | `infrastructure/integrations/` | Anti-corruption layer + circuit breaker + Secrets Manager. |
| Database | `infrastructure/persistence/` | TypeORM entities and repositories, behind a repository port. |
| Publishing events | `infrastructure/messaging/` | SNS publishes. |
| Consuming events | `infrastructure/messaging/` | SQS consumes. Handlers must be idempotent. |
| Secrets | `infrastructure/secrets/` | AWS Secrets Manager. Never `.env` in production. |
| Incoming HTTP | `interface/controllers/` | Validated with Zod. The Gateway already checked the JWT. |
| Incoming webhook | `interface/webhooks/` | Validated by the provider's **signature**, not a JWT. Idempotent. |
| Server push | `interface/realtime/` | SSE (`@Sse`) or WebSocket. The Gateway passes the stream through unbuffered. |
| Health check | `src/health/` | Liveness and readiness. Required by ECS Fargate. |
| Rate limiting | `common/guards/` | Throttler guard, counters in Redis, keyed off `X-Forwarded-For`. |

## North-South and East-West

Two kinds of traffic, and they do not use the same door.

**North-South** is the outside coming in: web, mobile, external systems. It **always** passes the API Gateway, which validates the JWT (RS256), authorizes scopes, rate limits, and routes. A service can assume the caller's identity was already verified.

**East-West** is service to service, inside the private subnet. It goes **direct**, over internal DNS with a service token — not through the Gateway. Routing internal calls through the public door adds latency and turns the Gateway into a single point of failure.

Prefer an **event** over a direct call. If service A does not need service B's answer to finish its own work, publish to SNS and let B consume from SQS. A direct client couples the two services' availability; an event does not.

## Rate limiting

The Gateway rate limits North-South traffic, and the service limits again in `common/guards/` with a custom throttler guard.

Its counters live in **Redis**, not in process memory. The service runs as several Fargate tasks, so an in-memory counter would let each task allow the full quota on its own.

It keys off **`X-Forwarded-For`**. Behind the Gateway the socket's peer address is the Gateway itself, so every request would otherwise look like one client.

## Talking to another DEAL service

`infrastructure/clients/` holds one client per service it calls, each behind a port declared in `domain/`.

- Resolve the target by **internal DNS**, never a public URL.
- Authenticate with a **service token**, not a user's JWT.
- One database per service: **never query another service's database.** If you need its data, ask it or subscribe to its events.

## Talking to an external API

`infrastructure/integrations/` holds one folder per provider (`bank/`, `sap/`, `azure/`), and each one carries three things:

**Anti-corruption layer.** The provider's shape stops at the boundary. Map its payload into the domain's own types inside the adapter, so a change on their side is a change in one file rather than a change spread through use cases.

**Circuit breaker.** An external provider will be slow or down. Without a breaker, its outage becomes this service's outage as requests pile up on a hanging socket.

**Secrets Manager.** Credentials come from `infrastructure/secrets/`, never from code and never from a committed `.env`.

## Receiving a webhook

A webhook is not an authenticated user, so **JWT does not apply**. Two requirements:

1. **Verify the provider's signature** before doing anything else with the body. An unverified webhook is an unauthenticated write.
2. **Be idempotent.** Providers retry, and a retry must not create a second payment, order, or invoice.

## Events

SNS publishes, SQS consumes, and the fan-out is what keeps services decoupled.

**Every SQS handler must be idempotent.** SQS guarantees at-least-once delivery, so the same message *will* arrive twice — this is normal operation, not an error case. Deduplicate on a message or business key held in Redis (`infrastructure/cache/`), and make the handler safe to run again.

## Realtime

`interface/realtime/` serves server push: SSE via NestJS's `@Sse`, or WebSocket. The Gateway forwards the stream **without buffering** — buffering a stream is the same as breaking it.

The web app receives changes this way; it has no webhooks of its own. The mobile app uses FCM push instead when it is closed.

## Data on the way in and on the way out

These are different concerns and are easy to conflate:

- **Inbound is validation.** Zod checks that what arrives has the right shape, at the `interface/` boundary, before it reaches a use case.
- **Outbound is PII masking.** Phone numbers and identity documents are masked in logs and responses (`712****67`). Validation does not do this, and masking does not validate.

Logs are JSON and carry the `requestId`, so a call can be traced across services in CloudWatch and X-Ray. Propagate it on every East-West call and every event.

## What NOT to do

- **Do not import a client, an SDK, or a driver into `domain/` or `application/`.** They depend on ports. If a use case needs Redis, it depends on `CachePort`, not on a Redis client.
- **Do not call another DEAL service through the Gateway.** That door is for traffic arriving from outside.
- **Do not read another service's database.** One service, one database, no cross-service queries.
- **Do not trust a webhook without checking its signature**, and do not assume it arrives once.
- **Do not take the acting user from the request body.** It comes from the token. An audit row whose user came from the frontend proves nothing.
- **Do not put credentials in code or in a committed `.env`.** Production reads them from Secrets Manager.
