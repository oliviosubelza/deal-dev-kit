---
name: general-tdd
description: "Opt-in test-first workflow for CRM DEAL — offer to write the test before the implementation, then run the RED-GREEN-REFACTOR cycle: watch the test fail for the right reason, write the minimal code to pass, refactor on green. Use before writing implementation code for any feature or bugfix in crm-deal-web, crm-deal-mobile, or a crm-deal-*-service."
applies_to: [backend, web, mobile]
---

# Test-driven development

Writing the test first has one concrete payoff: you watch it fail, so you know it can catch the bug it describes. A test written after the code passes on the first run — which proves nothing about whether it tests the right thing.

## Offer it, don't impose it

Before writing implementation code for a feature or a bugfix:

1. Say in one line what test-first buys here — *"if I write the test first, we see it fail before the fix, so we know it actually covers this."*
2. Ask whether to write the test first.
3. **Yes** → run the cycle below.
   **No** → implement directly, and don't raise it again for the same task.

Throwaway spikes, generated code, and config files don't need the question — just write them.

## The cycle

### RED — write one failing test

One test, one behavior, a name that says what the behavior is. Exercise the real code, not a mock of it.

```ts
test('retries a failing operation three times', async () => {
  let attempts = 0;
  const op = () => { attempts++; if (attempts < 3) throw new Error('fail'); return 'ok'; };

  const result = await retryOperation(op);

  expect(result).toBe('ok');
  expect(attempts).toBe(3);
});
```

### Verify RED — watch it fail

Run the test. Confirm it **fails**, not errors, and that the message is the one you expect — the feature is missing, not a typo in the test.

- Passes already? It is testing behavior that already exists. Rewrite it.
- Errors instead of failing? Fix the error and re-run until it fails cleanly.

### GREEN — minimal code to pass

The simplest thing that makes the test green. No extra options, no speculative parameters, no refactoring of nearby code.

### Verify GREEN — watch it pass

Run the test. It passes, the rest of the suite still passes, and the output is clean — no new warnings. If the test fails, fix the code, not the test.

### REFACTOR — clean up on green

Only once green: remove duplication, improve names, extract helpers. No new behavior. Tests stay green throughout.

Then the next failing test for the next behavior.

## Bugfixes

Reproduce the bug with a failing test first, then fix it. The test proves the fix works and stops the bug from coming back.

## Writing tests worth keeping

When you write or change a test, or add a mock, read [writing-good-tests.md](writing-good-tests.md). The short version:

- Before writing the body, name the production change that should make this test fail. Can't name one → it isn't testing anything.
- Derive expected values by hand. An expectation computed by the code under test always passes.
- Assert real behavior, never a mock's behavior. A mock assertion only proves the mock is wired up.
- Mock the slow or external thing one level down; keep what the test depends on real.

## What NOT to do

- Do not skip the offer silently. The developer decides whether to go test-first, but they decide out loud.
- Do not nag. Asked once and answered "no" settles it for that task.
- Do not write the assertion against a mock (`expect(mock).toHaveBeenCalled()`) as the point of the test.
- Do not mark work done on the strength of a test that never failed first — you have not shown it can catch anything.
- Do not report work as done on green tests alone. Green says the units behave; it says nothing about whether the app starts. Run the boot gate in `general-smoke-run` before calling it finished.

---

Adapted for CRM DEAL from [github.com/obra/superpowers](https://github.com/obra/superpowers) (`test-driven-development`), which mandates the cycle; here it is offered per task.
