# Validation Harness

This harness is structured on the [Validation Atlas](https://the-validation-atlas.thinkbridge.academy/)
(thinkbridge, v1.6). SBOM Creator is a **Small (S)** project by the Atlas's
sizing (~1.3K LOC, single maintainer, no money/health/infra risk), so the
baseline is the Atlas's **S adoption sequence**. We also pull forward several
higher-tier techniques that are cheap and high-return for a single-binary Go CLI
that shells out and emits HTML: fuzzing, mutation testing, race detection,
differential/golden testing, and supply-chain verification.

## A note on the 100% coverage target

The Atlas's Common Failure #2 is *treating coverage as a goal* (Goodhart's law):
100% line coverage can be reached with tests that execute code but assert
nothing. We asked for and hit 100% statement coverage — and then backed it with
**mutation testing at 100% efficacy**, which mutates the source and fails if any
change goes undetected. Coverage says every line ran; mutation says the tests
would *catch a bug* on every line. The second is the real guarantee.

## Current status (run `./test.sh`)

| Gate | Result |
|------|--------|
| `gofmt` | clean |
| `go vet` | clean |
| `staticcheck` | clean |
| `govulncheck` (supply-chain) | no known vulnerabilities (stdlib only, zero deps) |
| `go test -race` | pass, race-clean |
| statement coverage | **100.0%** |
| mutation efficacy (gremlins) | **100.00%**, mutator coverage 100% |
| fuzzing | seed corpus deterministic on every push; nightly exploration (5M+ execs/target locally), no crashes, injection invariant holds |

## Atlas S-sequence — verdicts

| # | Technique | Verdict @ S | How it's implemented here |
|---|-----------|-------------|---------------------------|
| 1 | Static type checking | Must-have | Go is statically typed; `go vet` + `staticcheck` add strictness |
| 2 | Linting + formatter | Must-have | `gofmt -l` gate + `go vet` + `staticcheck` |
| 3 | Secret scanning | Must-have | `gitleaks` runs in `test.sh`, in CI, and as a version-controlled pre-commit hook (`hooks/pre-commit`, enable with `git config core.hooksPath hooks`) |
| 4 | Dependency scanning | Must-have | `govulncheck`; the binary has **zero third-party deps** (stdlib only), which is the strongest form |
| 5 | Unit tests on riskiest 20% | Must-have | Exhaustive table tests on the classifier (the brains) and every parser; 100% coverage overall |
| 6 | Acceptance criteria for golden path | Must-have | `TestRealMain*` assert the end-to-end exit-code contract (0/1/2) and report output |
| 7 | One E2E smoke test | Must-have | CI `e2e` job builds + runs the tool on macOS/Linux/Windows and validates the generated page |
| 8 | Error tracking (production) | Recommended | N/A for an offline CLI; exit codes + the on-page "missing tools" banner are the user-facing signal |

## Higher-tier techniques pulled forward (cheap + high-return here)

| Technique | Atlas tier | Why included | Where |
|-----------|-----------|--------------|-------|
| Race detection | L | `runIn` used goroutines (now context-based); `-race` guards regressions | `go test -race` |
| Fuzz testing | L | Parsers consume external tool output; the renderer embeds attacker-influenceable names into HTML | `fuzz_test.go` |
| Output guardrails | L (AI tier, repurposed) | The injection-safety fuzz target proves no scanned value can break out of the inline `<script>` | `FuzzRenderInjectionSafety` |
| Mutation testing | L | Defends the 100% coverage number against Goodhart's law | `gremlins`, `./test.sh --mutation` |
| Differential/golden testing | — | Round-trip: render → parse the embedded JSON back → assert equality | `TestRenderRoundTrip` |
| SBOM & supply-chain | L | Fitting, since this *is* an SBOM tool; `govulncheck` + zero-dependency design | CI + `go.mod` |

## Deliberately out of scope (cost exceeds benefit at S)

Per the Atlas's own guidance against stacking redundant techniques, these are
**Skip / Optional** for a project this size and are not implemented:
SAST beyond staticcheck, container/IaC scanning (no containers/IaC), DAST/API
fuzzing (no network surface), contract testing (no APIs), load/soak/chaos (a
short-lived CLI), visual regression, formal verification, distributed tracing,
RUM/synthetic monitoring. If SBOM Creator grew a server or web service, the
Atlas's M/L sequences would promote several of these.

## What the harness does *not* guarantee

- The unit suite mocks external commands (`brew`, `winget`, `plutil`, …) with
  canned output. It proves the **parsing and classification** are correct. It
  does **not** prove a given OS's real tool emits the format we expect — that is
  what the CI `e2e` job (running the real binary on each OS) is for.
- Provider/category inference is heuristic. Tests pin the heuristics' behavior;
  they don't claim every component on every machine is categorized "correctly"
  in a subjective sense.

## Running it

```
./test.sh                 # full gate (fast tiers); non-zero exit on any failure
./test.sh --mutation      # also run mutation testing (~25s)
./test.sh --fuzz 60s      # longer fuzzing burst per target
```

CI (`.github/workflows/ci.yml`) runs the `test` and `e2e` jobs on
ubuntu/macos/windows, plus a Linux `mutation` job, on every push and PR. Every
`Fuzz*` target's seed corpus runs deterministically inside `test` (regression
coverage on all three OSes). Time-bounded `-fuzz` exploration is
nondeterministic — the engine can report "context deadline exceeded" on slow
runners — so it is kept off the merge path and runs in a separate `fuzz` job
nightly and on manual dispatch (ubuntu), where it can still surface new
crashers without flaking unrelated PRs.
