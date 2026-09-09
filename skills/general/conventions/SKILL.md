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

Commits follow **Conventional Commits**: `feat`, `fix`, `chore`, `docs`, and the rest of the standard set, as `type(scope): subject`.

The scope is the entity or area the change belongs to — `order`, `invoice`, `route`. The Jira key closes the line, in parentheses, so the board links the commit to its card without anyone doing it by hand.

```
feat(order): migración, ORM-entity y clases de dominio (SALES-12)
fix(invoice): mask phone numbers in the audit log (SALES-48)
chore(ci): pin the Node version in the build image (SALES-51)
```

Use the real key from your card — `SALES-12` here is a placeholder, and the project prefix differs per board.

A commit is authored by the person who owns the change, and by nobody else. **Never add a `Co-Authored-By` trailer for an AI agent, and never add any other AI attribution** — not in the commit message, not in the PR description, not in a code comment.

```
Co-Authored-By: Claude <noreply@anthropic.com>   ← never
```

The history records who is accountable for the change, which is always a person. Which tools they used to write it is not what a trailer is for, and a machine address in the log makes `git log --author` and `git blame` answer the wrong question.

## Zod is the single source of truth

One schema both validates and types. Never declare a TypeScript type and a validator separately for the same data — they drift, and the drift shows up in production.

Schemas are shared between web and mobile.

## Polyrepo + trunk-based

One repository per project. Short branches, integrated often.

A branch that lives for days accumulates conflicts and hides work from everyone else. Merge small and merge frequently.

**Never commit to `main`.** Branch as `feat/<JIRA>-short-description` — `feat/SALES-12-order-domain` — and open a Pull Request. `main` moves only through a merged PR, so every change has a place where someone else can read it before it lands.

Moving the Jira card — In Progress when you start, Done when you push — is yours, not the agent's. The key in the branch and in the commit is what ties the two together.

## No secrets in code

`.env` locally, **AWS Secrets Manager in production**.

Nothing that is a credential — a password, a token, a key, a connection string — goes in a committed file.
