---
name: backend-persistence
description: "Flyway migrations, TypeORM entities, domain aggregates and the persistence mapper in a CRM DEAL NestJS service. Use when adding or changing a persisted field, writing a repository adapter, or mapping an aggregate to a table."
---

# Persistence

Every persisted entity is three artefacts, written in this order. **The order is the rule:** the table decides the fields, the ORM entity mirrors it, the domain never learns either exists.

## The three artefacts, in order

| # | Artefact | Lives in | Rule |
| --- | --- | --- | --- |
| 1 | Flyway migration | `db/migration/` (repo root) | The source of truth for fields. Postgres, `BIGSERIAL PRIMARY KEY`, schema-qualified table (`sales.orders`), `TIMESTAMPTZ` for every instant. |
| 2 | TypeORM ORM entity | `infrastructure/persistence/entities/` | Mirrors the table 1:1. `synchronize: false`. snake_case columns declared explicitly: `@Column({ name: 'customer_id' })`. No business logic. |
| 3 | Domain aggregate + value objects | `domain/` | Business rules only. Zero framework decorators, no TypeORM import. |

A field change starts in the migration, never in the entity. Write the SQL, then bring the entity into line. Because `synchronize: false`, a drifting entity does **not** fail loudly — it fails at the first query in production.

Migration and entity must match field for field. The domain is deliberately **not** in that correspondence: it may name things differently, or collapse three columns into one value object. The mapper absorbs the difference.

## Worked example

```sql
-- db/migration/V12__create_sales_orders.sql
CREATE TABLE sales.orders (
  id          BIGSERIAL PRIMARY KEY,
  customer_id BIGINT      NOT NULL,
  total_cents BIGINT      NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```ts
@Entity({ schema: 'sales', name: 'orders' })
export class OrderOrmEntity {
  // bigint comes back from Postgres as a string, not a number.
  @PrimaryGeneratedColumn({ type: 'bigint' }) id!: string;
  @Column({ name: 'customer_id', type: 'bigint' }) customerId!: string;
  @Column({ name: 'total_cents', type: 'bigint' }) totalCents!: string;
  @Column({ name: 'created_at', type: 'timestamptz' }) createdAt!: Date;
}
```

```ts
// domain/order.ts — no decorators, no TypeORM import.
export class Order {
  private constructor(
    readonly id: OrderId | null,
    readonly customerId: CustomerId,
    readonly total: Money,
    readonly createdAt: Date,
  ) {}

  static create(customerId: CustomerId, total: Money): Order {
    return new Order(null, customerId, total, new Date());
  }
}
```

## The persistence mapper

It lives in `infrastructure/persistence/mappers/`, **not** `interface/mappers/`. `interface/mappers/` maps the domain to the wire (Zod DTOs); this one maps the domain to the table (TypeORM entities) — same technique, opposite side of the core, and one folder for both would put a TypeORM import next to a DTO.

It is the only file that imports both the aggregate and the ORM entity. That keeps TypeORM out of `domain/`, and keeps the repository adapter down to queries.

## The id comes from the database

The aggregate carries `id: OrderId | null` until the row exists. The repository port is `save(order: Order): Promise<Order>`, returning the aggregate rehydrated with the id the insert produced. The use case uses the **returned** aggregate for anything downstream: the event it publishes, the DTO it maps, the response.

The trade-off: a nullable id shows up even where the row certainly exists. The purer alternative — two types, `NewOrder` and `Order` — removes the null but doubles the aggregate, factory and mapper per entity. We take the null because `BIGSERIAL` is imposed; an app-generated UUID would dissolve the problem, and is off the table.

## What NOT to do

- **Do not change a field in the ORM entity first.** It starts in the migration.
- **Do not turn on `synchronize`.** Flyway owns the schema; two owners means no history.
- **Do not put a TypeORM decorator or a `typeorm` import in `domain/`.**
- **Do not rely on TypeORM's naming strategy for snake_case.** Name every column explicitly, so the mapping reads next to the migration.
- **Do not reuse the ORM entity as the domain aggregate**, and do not return it from a repository. The port returns domain types.
- **Do not read `order.id` before the repository returned the saved aggregate**, and never write `order.id!` to silence the compiler.

## Not in this skill

| Question | Skill |
| --- | --- |
| Which layer a file belongs in, the module tree, the port rule | `backend-architecture` |
| Which adapter folder each connection lives in | `backend-connections` |
| Domain entity → Zod DTO and the OpenAPI spec | `backend-architecture`, `/generate-schema` |
