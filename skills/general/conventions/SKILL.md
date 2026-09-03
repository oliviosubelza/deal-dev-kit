---
name: general-conventions
description: "Cross-cutting rules every CRM DEAL repository follows — strict TypeScript, ESLint and Prettier with Conventional Commits, Zod as the single source of truth, polyrepo with trunk-based development, and no secrets in code. Use before writing or reviewing code in crm-deal-web, crm-deal-mobile, or any crm-deal-*-service."
---

# Team conventions

The same rules in every repository, so anyone can move between projects without friction.

## Strict TypeScript

No loose `any`. Types catch errors before the code runs, and an `any` gives that up exactly where it matters most.

## ESLint + Prettier

Formatting is automatic — never argue about it in review, and never hand-format.

Commits follow **Conventional Commits**: `feat`, `fix`, `docs`, and the rest of the standard set, as `type: subject`.

```
feat: add order cancellation
fix: mask phone numbers in the audit log
docs: document the filter contract
```

## Zod is the single source of truth

One schema both validates and types. Never declare a TypeScript type and a validator separately for the same data — they drift, and the drift shows up in production.

Schemas are shared between web and mobile.

## Polyrepo + trunk-based

One repository per project. Short branches, integrated often.

A branch that lives for days accumulates conflicts and hides work from everyone else. Merge small and merge frequently.

## No secrets in code

`.env` locally, **AWS Secrets Manager in production**.

Nothing that is a credential — a password, a token, a key, a connection string — goes in a committed file.
