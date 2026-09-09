# Writing good tests

Read this when writing or changing a test, adding a mock, or adding a cleanup/helper method for tests.

A test exists to catch a specific break. Two principles govern the rest:

1. **Every test names the break it catches.**
2. **Every test exercises the real thing.**

Going test-first produces both for free: a test written before the code and watched failing against real code has already proven it can fail, and only earns a mock when the real dependency turns out to be slow or external.

## Principle 1 — name the break

Before writing the test body, answer: **what production change should make this test fail, and is that change a bug or a decision?** A test earns its place by catching a wrong branch, a missing side effect, a wrong argument, a boundary case, or a broken contract.

**Derive expectations independently.** Use literals and hand-checked fixtures; a table with literal `want` values is the preferred shape. An expectation computed by the code under test — or its helpers — passes no matter what that code does.

```ts
// Mirror assertion: the same builder computes both sides — always true
const expected = buildSearchQuery({ tag: 'urgent' });
expect(buildSearchQuery({ tag: 'urgent' })).toBe(expected);

// Hand-derived literal
expect(buildSearchQuery({ tag: 'urgent' })).toBe('tag:"urgent"');
```

**No change detectors.** If only an intentional decision can fail a test — a constant's value, exact message wording, a private structure — it fires on every redesign and sleeps through real bugs. Test the behavior that depends on the decision: not `expect(MAX_RETRIES).toBe(5)` but "a failing call is retried 5 times and the 6th attempt never happens".

**Behavior, not text.** Asserting that a script or config contains an exact line only proves the source is the source. Run the thing against controlled inputs and assert outputs, side effects, or exit codes.

**Your code, not the framework.** Test the contract your code makes at its boundaries — the route you register, the query you emit, the payload you produce. Upstream mechanics are their maintainers' tests to write. Constructors, getters, constants, and trivial forwarding earn a test only when they validate, normalize, default, derive, enforce, or cause a side effect.

### Gate

```
Before writing the test body:
  Name the production change that would make this test fail.
    Cannot name one           → redesign around an observable behavior
    "The source text changed"  → run the artifact, assert its effects
    Only an intentional decision → change detector; test the behavior
                                   that depends on the decision
  Confirm the expected value is derived without the code under test.
    If it reuses the code's logic or helpers → replace with a literal
```

## Principle 2 — exercise the real thing

**The mock earns no assertions.** A mock assertion passes when the mock is present and fails when it is absent — it says nothing about the component. Assert the real component's behavior.

```ts
// Real behavior
expect(screen.getByRole('navigation')).toBeInTheDocument();

// Mock existence
expect(screen.getByTestId('sidebar-mock')).toBeInTheDocument();
```

If you catch yourself asserting on a mock, ask: *are we testing the behavior of a mock?* Unmock it or delete the assertion.

**Mock at the right level.** Learn every side effect of the real method before replacing it. Mock the slow or external operation and keep what the test depends on real. When unsure, run the test against the real implementation first and watch what actually needs to happen.

```ts
// The mock swallows the config write that duplicate detection reads
vi.mock('ToolCatalog', () => ({ discoverAndCacheTools: vi.fn().mockResolvedValue(undefined) }));

// Mock only the slow server startup; the config write stays real
vi.mock('MCPServerManager');
```

**Make doubles specific.** When arguments, call counts, or ordering are part of the contract, assert them — a fake that accepts anything verifies nothing. Give each branch (success, error, malformed) its own fixture.

**Mirror real data completely.** Mock the whole structure as it exists in reality, not just the fields your test reads. A partial mock fails silently when downstream code reads an omitted field: the test passes while integration breaks.

**Production classes carry production methods only.** Cleanup that only tests need lives in test utilities, never as a `destroy()` on the production class. Is this method called only from tests? Does this class own the resource's lifecycle? Wrong answers → test utility.

**Prefer real components over complex mocks.** When mock setup outgrows the test logic, or tests break every time the mock changes, switch to an integration test with real components.

### Gate

```
Before adding a mock or test helper:
  List the real method's side effects; keep the ones the test
    depends on real — mock the slow/external level below them.
  Mock responses mirror the complete real structure.
  A method only tests call lives in test utilities, not production.
  About to assert on the mock itself? → unmock it or delete the assertion.
```

## Events across modules

**Test the publisher's contract, not the SDK.** Publishing is correct when the topic, the payload shape and the correlation id are right. Put the double at the SDK level and assert on the message your code builds — the client's internals are its maintainers' tests to write.

**The handler is the consumer's unit.** Feed it the envelope as SQS actually delivers it, not a hand-tidied object. *Mirror real data completely* applies here in full: a partial fixture passes while the real payload breaks on a field the test never wrote.

**Idempotency is a test, not a hope.** `backend-connections` requires every SQS handler to be idempotent. You show it by running the handler twice with the same message and asserting the second run changes nothing observable — no second row, no second outbound call.

**Two sides, one contract, tested apart.** Do not stand up SNS and SQS to prove that two modules agree. Assert that the publisher emits the shape, assert that the consumer accepts it, and keep both honest with one shared fixture of the event.

## The mutation check

Before finishing, mentally mutate the production code. At least one test should fail for each realistic mutation:

- Wrong constant or argument
- Wrong branch handler
- Missing state change or side effect
- Empty or default return
- Missing validation for zero, empty, null, unauthorized, or malformed input

A mutation nothing catches marks the behavior as unprotected — or the test as tautological.

## Quick reference

| When you... | Do |
|---|---|
| Write any test | Name the break it catches — a bug, not a decision |
| Build an expected value | Derive it by hand, never with the code under test |
| Test a script or config | Run it against controlled input; never grep its text |
| Reach for a dependency test | Test your boundary contract, not their documented mechanics |
| Want to assert on a mocked element | Test the real component, or unmock it |
| Are about to mock a method | Learn its side effects; mock the slow/external level |
| Build a mock response | Mirror the real structure completely |
| Need cleanup only tests use | Put it in test utilities |
| Watch mock setup balloon | Switch to an integration test with real components |
| Test an event publisher | Assert the message you build — topic, shape, correlation id |
| Test an SQS handler | Feed the envelope as delivered, then run it twice |
| Cover an event across modules | One shared fixture, two tests — not real SNS/SQS |
| Finish a test file | Run the mutation check |

## Warning signs

- Setup and assertion share the same object, guaranteeing equality
- The test can fail only through a crash or a missing selector
- The test fails on every intentional change, never on accidental breakage
- Expected values are hidden behind loops, builders, or helpers
- The test greps source text, or asserts a removed symbol stays removed
- The test would still matter if only the framework remained
- The test exists for coverage, checking no side effect or outcome
- An assertion checks a `*-mock` test id, or fails if you remove the mock
- A method is called only from test files
- Mock setup is more than half the test, or you cannot explain why the mock is needed
- An event test asserts that the SDK client was called, never the message it carried
- A handler that writes or calls out is run once, and idempotency is only claimed
- The consumer's fixture is a tidied object the publisher would never emit
