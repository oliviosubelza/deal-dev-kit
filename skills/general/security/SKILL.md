---
name: general-security
description: "Cross-cutting API security rules for CRM DEAL — token TTL and scopes, refresh rotation, CORS, geoblocking, rate limiting, PII masking, and transport/at-rest encryption. Use when writing anything that issues, checks, or handles a token, a CORS rule, PII, or a request the API Gateway rate-limits."
applies_to: [backend, web, mobile]
---

# Security

These rules come from the coordinator's security architecture review and apply to every project type — backend, web, and mobile all talk to the same API Gateway under the same policy.

## Tokens and scopes

Access tokens are short-lived. The security review states **15 minutes of inactivity** for the **mobile** session token specifically, not as a figure for every client. Refresh tokens are **rotated on each use**.

Token TTL, rotation and revocation rules appear in the security review's own pre-Sprint-1 checklist as **still to be finalised**. Treat these numbers as the reviewed starting point, not settled policy, and do not infer a revocation rule — no document states one yet.

Scopes are named `verb:resource` — `read:orders`, `write:collections` — never a single all-access token. A token's scopes gate what it can do per module.

For where the web app and the mobile app each keep their tokens, see the `web-architecture` and `mobile-architecture` skills — that storage detail is not repeated here.

## Network boundary

- **CORS**: only `*.grupovenado` origins are allowed.
- **Geoblocking**: public API and login endpoints reject traffic from outside Bolivia and approved partner countries.
- **Rate limiting**: 100 requests per minute per device — the figure the review gives for the mobile fleet.
- **Transport**: TLS 1.2+ or 1.3 everywhere. No plain HTTP.

## Data protection

- **At rest**: AES-256.
- **PII masking**: customer phone numbers and ID numbers are masked everywhere **except** the Sales and Collection modules.

## Who enforces this

The **API Gateway** owns authentication, rate limiting, and logging — one central point in front of every service. A backend service should not reimplement its own throttling as the standard path; the Gateway is where these rules live.

## What NOT to do

- Do not issue a token with broader scope than the caller needs.
- Do not accept a request over plain HTTP.
- Do not display an unmasked customer phone or ID number outside Sales or Collection.
- Do not build a second rate limiter inside a service as the primary defense — that is the Gateway's job.
