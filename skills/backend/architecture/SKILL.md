---
name: backend-architecture
description: "Hexagonal layering and the folder structure of a CRM DEAL NestJS microservice — domain, application, infrastructure, interface, and one module per business domain. Use before creating files in a crm-deal-*-service repository, or when deciding which layer a piece of code belongs in."
---

# Backend architecture

NestJS + TypeScript, hexagonal. One repository per microservice, behind the API Gateway.

## The core is protected

Business rules go in the centre. Technical concerns go outside. **Dependencies point inward.**

| Layer | Holds |
| --- | --- |
| `domain/` | The core: pure business rules. Entities, value objects, ports, exceptions. |
| `application/` | Use cases. Orchestrates the domain to carry out one operation. |
| `interface/` | The way in: receives the HTTP request. |
| `infrastructure/` | The way out: database, events, external APIs. |

`interface/` and `infrastructure/` are **siblings on opposite sides of the core**, not layers wrapping one another. One is entry, the other is exit.

A port is an interface declared in `domain/` and implemented in `infrastructure/`. The domain knows the port; it never knows the adapter.

## Structure

```
crm-deal-<service>-service/
└─ src/
   ├─ main.ts · app.module.ts
   ├─ config/                    # typed env + validation (Zod)
   ├─ common/                    # filters, guards, interceptors, pipes
   ├─ shared/                    # Result, logger port, base errors
   └─ modules/
      └─ orders/                 # one module per domain
         ├─ domain/              # CORE: entities, value-objects, ports, exceptions
         ├─ application/         # use-cases
         ├─ infrastructure/      # ADAPTERS
         │  ├─ persistence/      # TypeORM: entities, repositories
         │  ├─ cache/            # Redis: cache, idempotency, locks
         │  ├─ messaging/        # SNS publisher · SQS consumers
         │  ├─ clients/          # other DEAL microservices (internal)
         │  ├─ integrations/     # external APIs (bank, sap, azure) + ACL
         │  └─ secrets/          # AWS Secrets Manager provider
         ├─ interface/           # controllers, dto, mappers, webhooks, realtime (SSE)
         └─ health/              # Fargate health check
```

Plus, at the repository root: `db/migration` (Flyway), `test` (unit + e2e), and a `Dockerfile`.

## One module per domain

A module is a business domain — `orders`, `customers`, `invoices` — and it carries its own four layers. A new domain is a new folder under `modules/`, not new files spread across existing ones.

`config/`, `common/` and `shared/` are the only things that live outside a module:

- `config/` — environment variables, typed and validated with Zod.
- `common/` — NestJS cross-cutting pieces: filters, guards, interceptors, pipes.
- `shared/` — `Result`, the logger port, base error types.

## One service, one database

Each microservice owns its database. **No cross-service queries.** If you need another service's data, call it through `infrastructure/clients/` or subscribe to its events.

## What NOT to do

- **Do not import from `infrastructure/` inside `domain/` or `application/`.** Dependencies point inward. The core depends on ports.
- **Do not put business rules in a controller.** A controller receives, validates, and delegates to a use case.
- **Do not query another service's database.**
- **Do not spread one domain across the module tree.** Everything about orders lives under `modules/orders/`.
