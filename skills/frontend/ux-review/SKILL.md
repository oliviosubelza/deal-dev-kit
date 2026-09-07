---
name: frontend-ux-review
description: "UX/UI review lens for CRM DEAL screens — eight principles (Hick's Law, Miller's Law, white space, KISS, minimalism, Don't Make Me Think, progressive disclosure, visual hierarchy) applied to real code, reported as findings anchored to file:line with a concrete diff and one of four severities. Use when reviewing or writing UI in crm-deal-web or crm-deal-mobile."
---

# UX/UI review

A reading lens for a screen, not a linter. Run it when reviewing UI someone wrote, and against your own before calling it done.

Every finding names a principle, a `file:line`, what is wrong, and the change that fixes it. A finding with no location is an opinion; a finding with no diff is a complaint.

## Numbers are triggers, not verdicts

Every threshold below means *stop and justify*, never *reject*. A `DataTable` with twenty columns is correct — columns are data, not decisions. A 200-line form that reads top to bottom gains nothing from being cut into three tabs; the user now hunts across tabs for a field that was in front of them.

A reviewer that rejects correct work gets ignored, and then the real findings go with it. When a threshold fires and the code is right, say so and move on.

## Severity

| Severity | Meaning |
| --- | --- |
| **Critical** | The user cannot finish the task, or finishes it wrong. Blocks the merge. |
| **Major** | Finishable, but the screen costs real time or invites an error every time it is used. |
| **Minor** | Friction the user absorbs. Fix it while the file is open. |
| **Cosmetic** | Inconsistent with the rest of the app, no functional cost. |

Nothing here is accept/reject. A review is a list of findings with severities and a per-principle line, and the author decides what ships.

## The eight principles

### Hick's Law

Decision time grows with the number and complexity of the options offered at once.

**Trigger:** more than ~7 primary actions competing in one view; a select or menu whose options are unordered and unranked; a toolbar where every button looks equally likely.

**Judgement:** is every option needed *at this step*? Ordering by frequency, a real default, grouping, and deferring the rare ones (see [progressive disclosure](#progressive-disclosure)) each cut the cost without removing capability. Options behind a `DropdownMenu` still exist — they just stop being asked on every glance.

### Miller's Law

Working memory holds about seven chunks. Grouping is what makes more than that tractable.

**Trigger:** more than ~7 sibling fields, links or cards in one visual run with nothing separating them.

**Judgement:** group by meaning, not by count. `FieldSet` + `FieldLegend`, a `Card` per concern, a heading, or plain spacing all turn a run of twenty into four groups of five. Splitting a coherent group to hit a number is the failure mode, not the fix.

### White space

Deliberate emptiness: breathing room, legibility, and grouping without drawing anything.

**Trigger:** a `Separator` or border doing work that spacing would do; the same gap between everything, so nothing reads as a group; content pressed against its container.

**Judgement:** proximity groups more cheaply than a line does. Vary the gap between groups and within them, on the spacing scale — a one-off margin is a value nobody else will match.

### KISS

The simplest thing that works, scoped here to **UI complexity**: nesting, conditional rendering, and how much state one screen juggles.

**Trigger:** JSX nested past ~4 levels; a ternary chain choosing what to render; a component holding more than ~3 independent pieces of local state.

**Judgement:** a component this tangled is usually doing two jobs, and the fix is to name the second one. Early returns beat nested ternaries. Where the extracted piece then lives — component, hook, store, schema — is `web-architecture` and `mobile-architecture`, not this review.

### Minimalism

Only the essential elements. Ornament, redundant labels, and non-essential borders are subtractions waiting to happen.

**Trigger:** a label repeating its own placeholder; an icon, its text and its tooltip all saying the same thing; a border around something already set apart by space; decoration carrying no information.

**Judgement:** remove it and read the screen again. If nothing was lost, it was not essential.

### Don't Make Me Think

The interface has to be self-evident. The user should never stop to work out how it works or where to click.

**Trigger:** a control whose label does not say what it does (`Submit`, `OK`, a bare icon button); the primary action not visibly primary; an empty state with no next step; an error message with no remedy.

**Judgement:** read the screen cold, as someone who has not seen the ticket. Anywhere you have to reason about what happens next, the user will too — and they will do it under time pressure.

### Progressive disclosure

Show what is needed now; defer the advanced and the secondary.

**Trigger:** rare options sitting at the same level as the common path; a form asking for everything before it will accept anything; a settings panel with every section expanded.

**Judgement:** what does the user need in order to act *right now*? Everything else belongs behind a `Collapsible`, an `Accordion`, a second step, or an "advanced" section. Deferred is not hidden: the path to it has to be visible.

### Visual hierarchy

Size, colour, contrast, alignment and spacing pointing the eye at what matters first.

**Trigger:** two filled buttons competing for primary; heading sizes that do not follow nesting; a destructive action as prominent as the safe one; no single element that is clearly first.

**Judgement:** squint at the screen. What you see first is what the design says is most important — check that against what the task actually needs first. In this catalog that is expressed through `Button` variants and semantic tokens, never a hand-picked colour.

## Output

Terse and structured. Findings first, then one line per principle.

````md
### Findings

**Major · Hick's Law** — `src/features/orders/components/OrderToolbar.tsx:34`
Nine buttons in one row, all the same weight; six are used less than weekly, so
the two that matter are found by reading all nine.

```diff
-      <Button>Anular</Button>
-      <Button>Duplicar</Button>
-      <Button>Exportar</Button>
+      <DropdownMenu>…</DropdownMenu>
```

**Minor · Minimalism** — `src/features/orders/components/OrderFilters.tsx:12`
The label and the placeholder both read "Buscar pedido".

### Per principle

Hick's Law fail · Miller's Law pass · White space pass · KISS pass ·
Minimalism fail · Don't Make Me Think pass · Progressive disclosure pass ·
Visual hierarchy pass
````

No findings is a result: say so, and still print the per-principle line. Silence reads as "not reviewed".

## Not in this review

Point at these; do not restate them.

| Question | Skill |
| --- | --- |
| Strict TypeScript, Zod as the source of truth, no loose `any` | `general-conventions` |
| Which catalog component to use instead of custom markup, and the composition rules | `web-ui` |
| Where a component, hook, store or schema goes | `web-architecture`, `mobile-architecture` |

`crm-deal-mobile` uses React Native Paper, so `web-ui` does not apply there. The eight principles do.

Accessibility is **out of scope** for this version. Do not report a11y findings here.

## What NOT to do

- Do not turn a threshold into a verdict. Every number above asks for a justification, and "justified" is a valid answer.
- Do not file a finding without a `file:line` and a diff.
- Do not report the same problem under three principles. Pick the one that explains it and file it once.
- Do not restate what `general-conventions`, `web-ui`, `web-architecture` or `mobile-architecture` already say.
- Do not raise accessibility findings; they are deliberately not part of this lens.
