---
name: backend-architecture
description: "Hexagonal layering and the folder structure of a CRM DEAL NestJS microservice — domain, application, infrastructure, interface, one module per business domain, plus the bootstrap in main.ts, what common/ holds, and how Zod DTOs generate the OpenAPI spec. Use before creating files in a crm-deal-*-service repository, when writing a controller, DTO or mapper, when adding a filter, guard or interceptor, and when deciding which layer a piece of code belongs in."
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
   ├─ main.ts · app.module.ts     # bootstrap: Swagger, Helmet, CORS, global pipes
   ├─ config/                    # typed env + validation (Zod)
   ├─ common/                    # filters, guards, interceptors, pipes
   ├─ shared/                    # Result, logger port, base errors
   ├─ health/                    # liveness + readiness, for Fargate
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
         └─ interface/           # controllers, dto, mappers, webhooks, realtime (SSE)
```

Plus, at the repository root: `db/migration` (Flyway / TypeORM), `test` (unit + e2e), and a `Dockerfile`.

`health/` sits beside the modules, not inside one: a health check reports on the service, not on a business domain. It serves **liveness** and **readiness** separately, because ECS Fargate needs to tell a container that is alive from one that is ready to receive traffic.

## One module per domain

A module is a business domain — `orders`, `customers`, `invoices` — and it carries its own four layers. A new domain is a new folder under `modules/`, not new files spread across existing ones.

`config/`, `common/`, `shared/` and `health/` are the only things that live outside a module:

- `config/` — environment variables, typed and validated with Zod.
- `common/` — NestJS cross-cutting pieces: filters, guards, interceptors, pipes.
- `shared/` — `Result`, the logger port, base error types.
- `health/` — liveness and readiness checks.

### What `common/` holds

| Folder | Holds |
| --- | --- |
| `filters/` | Global exception filter. Emits **RFC 7807** problem details, and never leaks a stack trace. |
| `guards/` | Custom throttler guard: rate limiting backed by Redis, keyed off `X-Forwarded-For`. |
| `interceptors/` | `ZodSerializerInterceptor`, a logging interceptor, and a Prometheus metrics interceptor. |

## Bootstrap

`main.ts` wires what applies to the whole service:

- **Swagger** — `SwaggerModule` plus `patchNestJsSwagger()` from `nestjs-zod`, so Zod DTOs generate the OpenAPI spec.
- **Helmet** — security headers.
- **CORS**.
- **Global validation pipes**.

## Zod generates the OpenAPI spec

One schema validates the request, types the code, **and** documents the endpoint. Three artefacts from one declaration, so they cannot drift.

DTOs live in `interface/dto/` and are exported through `createZodDto(schema)` from `nestjs-zod`. Controllers carry the OpenAPI decorators: `@ApiTags`, `@ApiOperation`, `@ApiResponse`, `@ApiBearerAuth`.

**Swagger descriptions and examples belong in the Zod schema**, not in the decorator:

```ts
z.string().describe('...')
```

A description written in the decorator describes the endpoint; one written in the schema describes the data, and travels with it everywhere the schema is used.

`interface/mappers/` transforms explicitly between domain entities and Zod DTOs. Explicitly means written out: returning a domain entity straight from a controller couples the wire format to the core.

## One service, one database

Each microservice owns its database. **No cross-service queries.** If you need another service's data, call it through `infrastructure/clients/` or subscribe to its events.

## What NOT to do

- **Do not import from `infrastructure/` inside `domain/` or `application/`.** Dependencies point inward. The core depends on ports.
- **Do not put business rules in a controller.** A controller receives, validates, and delegates to a use case.
- **Do not query another service's database.**
- **Do not spread one domain across the module tree.** Everything about orders lives under `modules/orders/`.
- **Do not declare a DTO type by hand.** It comes from the Zod schema through `createZodDto`, so validation, types and the OpenAPI spec stay one thing.
- **Do not return a domain entity from a controller.** Map it to a DTO in `interface/mappers/`.
- **Do not let a stack trace reach an error response.** The global filter answers with RFC 7807 problem details.
