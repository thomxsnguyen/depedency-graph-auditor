# Testing Guide

## Philosophy

Tests are written **as each component is completed**, not deferred to the end.
Every package that contains logic ships a `_test.go` file alongside it.
Deduplication, concurrency safety, and the self-feeding expansion mechanic are
explicitly verified — these are the properties that distinguish a working system
from one that merely compiles.

---

## How to Run Tests

```bash
# Run all tests (includes race detector)
go test -race ./...

# Run a specific package
go test -race ./internal/queue/...

# Verbose output (see each test name)
go test -race -v ./internal/...

# Run a single named test
go test -run TestHandleHappyPath ./internal/auditor/

# Show coverage report in browser
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out
```

> Always use `-race`. Concurrency bugs are silent without it.

---

## Test Structure

Tests live **co-located** with the package they exercise — one `_test.go` file
per package. The `package foo_test` (external/black-box) form is preferred; it
enforces that tests only rely on exported API surface.

```
internal/
├── auditor/
│   ├── handler.go
│   ├── handler_test.go      ← AuditHandler + inline mock registry
│   ├── policy.go
│   ├── policy_test.go       ← LicensePolicy verdict table
│   ├── registry.go
│   ├── report.go
│   ├── store.go
│   └── store_test.go        ← PackageStore + EdgeStore + concurrency
├── queue/
│   ├── queue.go
│   └── queue_test.go        ← Submit/Dequeue contract + Close behaviour
├── semver/
│   ├── semver.go
│   └── semver_test.go       ← ParseRange + Resolve for all operators
└── worker/
    ├── pool.go
    └── pool_test.go          ← Job processing + Done signal + self-feeding
```

Shared test doubles that span packages live inline in the test file that uses
them. There is currently no `internal/testutil` package — duplication has been
minimal enough to keep things self-contained.

---

## Package Coverage

### `internal/queue`

| Test | What it verifies |
|:---|:---|
| `TestSubmitDequeue` | A submitted job comes back from Dequeue with the same ID |
| `TestDequeueOrderFIFO` | Jobs are delivered in submission order |
| `TestCloseUnblocksDequeue` | `Close()` unblocks a goroutine waiting on `Dequeue()` |
| `TestCloseAndDrainReturnsAllJobs` | Jobs submitted before `Close` are all delivered; the channel then returns `ok=false` |

---

### `internal/semver`

| Test | What it verifies |
|:---|:---|
| `TestParseRangeValid` | All supported operator forms parse without error |
| `TestParseRangeInvalid` | Garbage input returns a non-nil error |
| `TestResolveCaretPicksHighest` | `^4.18.0` resolves to the highest `4.x.x` in available |
| `TestResolveTildePicksHighest` | `~1.2.3` resolves to the highest `1.2.x` in available |
| `TestResolveExactVersion` | An exact version string resolves to itself |
| `TestResolveExplicitRange` | `>=1.0.0 <2.0.0` resolves to the highest within that band |
| `TestResolveNoMatchErrors` | Returns an error when no version satisfies the constraint |
| `TestResolveMalformedVersionsSkipped` | Unparseable entries in available are silently skipped |

---

### `internal/auditor` — Store

| Test | What it verifies |
|:---|:---|
| `TestPackageStoreAddNewReturnsTrue` | First `Add` of a package returns `true` |
| `TestPackageStoreAddDuplicateReturnsFalse` | Second `Add` of the same package returns `false` |
| `TestPackageStoreExistsAfterAdd` | `Exists` is `false` before `Add`, `true` after |
| `TestPackageStoreExistsDifferentVersionFalse` | `Exists` distinguishes by version |
| `TestPackageStoreAllReturnsSnapshot` | `All` returns every added package |
| `TestPackageStoreConcurrentSafe` | 20 concurrent goroutines — no races, exactly 1 package stored |
| `TestEdgeStoreAddAndAll` | `Add` then `All` returns every edge |
| `TestEdgeStoreAllReturnsIndependentSnapshot` | Mutating the slice from `All` does not corrupt internal state |
| `TestEdgeStoreConcurrentSafe` | 20 concurrent `Add` calls — no races, all edges recorded |

---

### `internal/auditor` — Policy

| Test | What it verifies |
|:---|:---|
| `TestLicensePolicyAllowedLicenses` | `MIT`, `Apache-2.0`, `ISC`, `BSD-2-Clause`, `BSD-3-Clause` all return `pass` |
| `TestLicensePolicyDisallowedLicenses` | `GPL-*`, `AGPL-*`, `WTFPL`, `CC-BY-SA-*` all return `policy_violation` |
| `TestLicensePolicyEmptyLicenseIsViolation` | An empty/missing license string is treated as a violation |

---

### `internal/auditor` — Handler

The `AuditHandler` tests use an **inline `mockRegistry`** struct that implements
`RegistryClient` from a deterministic in-memory map. No real HTTP is required.

| Test | What it verifies |
|:---|:---|
| `TestHandleHappyPath` | Two unseen deps → 2 child jobs, 1 stored node, 2 stored edges |
| `TestHandleVerdictPolicyViolation` | Disallowed license → `VerdictPolicyViolation` on stored node |
| `TestHandleDeduplicatesAlreadySeenPackage` | Package already in store → `nil` jobs, `nil` error, no edges written |
| `TestHandleSkipsChildJobForSeenDependency` | One seen, one unseen dep → 1 child job, but both edges recorded |
| `TestHandleRegistryErrorPropagates` | Registry failure → error returned, nothing written to stores |
| `TestHandleBadPayloadReturnsError` | Malformed JSON payload → unmarshal error before registry is called |

---

### `internal/worker`

The pool tests use small in-file handler types (`noopHandler`, `countingHandler`,
`selfFeedingHandler`, and the `handlerFunc` adapter) so that tests stay
self-contained.

| Test | What it verifies |
|:---|:---|
| `TestPoolProcessesAllJobs` | 10 submitted jobs → handler called exactly 10 times, Done fires |
| `TestPoolDoneFiresOnlyAfterWork` | Done fires after a single job's inFlight counter drops to zero |
| `TestPoolSelfFeedingExpansion` | Root → child → grandchild → Done (3 jobs total, no more) |
| `TestPoolWorkerCountBounded` | Pool of 3 handles exactly 3 jobs, Done fires, workers exit cleanly |

---

## Mocking Strategy

| Component | Approach |
|:---|:---|
| `RegistryClient` | Inline `mockRegistry` in `handler_test.go` — deterministic, no HTTP |
| `job.Handler` | Inline `noopHandler`, `countingHandler`, `selfFeedingHandler` in `pool_test.go` |
| `http.Client` | Not mocked — `NpmClient.baseURL` is injectable for future integration tests |

---

## Not Yet Tested

These components have no tests because they are not yet implemented:

| Package | Reason |
|:---|:---|
| `internal/depfile` | Stub — implementation pending |
| `internal/auditor/report.go` | Stub — implementation pending |
| `cmd/auditor/main.go` | Stub — entry point pending |

Integration test (mock-registry `httptest.Server` end-to-end) is tracked in
the phase 1 exit criteria and will be added once `depfile` and `report` are
implemented.
