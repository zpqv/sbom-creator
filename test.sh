#!/usr/bin/env bash
#
# Validation harness for SBOM Creator, structured on the Validation Atlas
# adoption sequence for a Small (S) project, plus the higher-tier techniques
# that fit a Go CLI (fuzzing, mutation testing, race detection, supply-chain).
#
# Usage:
#   ./test.sh             # run the full gate (fast tiers); fails the build on any miss
#   ./test.sh --mutation  # also run mutation testing (slow)
#   ./test.sh --fuzz 30s   # extend the fuzzing burst (default 10s per target)
#
# Missing optional tools are reported with the exact install command rather than
# silently skipped — "ask the user to install what's required".
set -uo pipefail
cd "$(dirname "$0")"
export PATH="$PATH:$(go env GOPATH)/bin"

COVERAGE_MIN=100.0      # the gate: 100% statement coverage
FUZZTIME=10s
RUN_MUTATION=0
FAIL=0

while [ $# -gt 0 ]; do
  case "$1" in
    --mutation) RUN_MUTATION=1; shift ;;
    --fuzz) FUZZTIME="$2"; shift 2 ;;
    *) echo "unknown arg: $1"; exit 2 ;;
  esac
done

step()  { printf "\n\033[1m== %s ==\033[0m\n" "$1"; }
pass()  { printf "  \033[32mPASS\033[0m %s\n" "$1"; }
fail()  { printf "  \033[31mFAIL\033[0m %s\n" "$1"; FAIL=1; }
need()  { # need <bin> <install-command>
  command -v "$1" >/dev/null && return 0
  printf "  \033[33mMISSING\033[0m %s — install with:\n      %s\n" "$1" "$2"
  return 1
}

# 1. Formatting (linting + formatter)
step "gofmt"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then fail "unformatted files:\n$unformatted"; else pass "all files formatted"; fi

# 2. go vet (static type/correctness checks)
step "go vet"
if go vet ./...; then pass "vet clean"; else fail "vet reported issues"; fi

# 3. staticcheck (deeper static analysis / SAST-adjacent)
step "staticcheck"
if need staticcheck "go install honnef.co/go/tools/cmd/staticcheck@latest"; then
  if staticcheck ./...; then pass "staticcheck clean"; else fail "staticcheck reported issues"; fi
else
  fail "staticcheck not installed (required gate)"
fi

# 3b. Secret scanning (Atlas S-sequence; also enforced by the pre-commit hook)
step "gitleaks (secret scan)"
if need gitleaks "brew install gitleaks"; then
  if gitleaks dir --redact --no-banner .; then pass "no secrets detected"; else fail "gitleaks found potential secrets"; fi
else
  fail "gitleaks not installed (required gate)"
fi

# 4. govulncheck (dependency / supply-chain verification)
step "govulncheck"
if need govulncheck "go install golang.org/x/vuln/cmd/govulncheck@latest"; then
  if govulncheck ./...; then pass "no known vulnerabilities"; else fail "vulnerabilities found"; fi
else
  fail "govulncheck not installed (required gate)"
fi

# 5. Unit/integration tests + race detector + coverage gate
step "go test -race + coverage gate (>= ${COVERAGE_MIN}%)"
if go test -race -covermode=atomic -coverprofile=cover.out . ; then
  pass "tests pass (race clean)"
  total="$(go tool cover -func=cover.out | awk '/^total:/{gsub(/%/,"",$3); print $3}')"
  awk -v t="$total" -v m="$COVERAGE_MIN" 'BEGIN{exit !(t+0 >= m+0)}' \
    && pass "coverage ${total}% >= ${COVERAGE_MIN}%" \
    || fail "coverage ${total}% < ${COVERAGE_MIN}%"
  go tool cover -func=cover.out | grep -v "100.0%" | grep -v "^total" \
    && printf "      (functions above are below 100%%)\n" || true
else
  fail "tests failed"
fi

# 6. Fuzzing burst (robustness + injection guardrail)
step "fuzz (${FUZZTIME} per target)"
for tgt in FuzzRenderInjectionSafety FuzzVendorFromHomepage FuzzParseBrew FuzzParsePip FuzzParseWindowsRegistry FuzzParseGem FuzzClassifyCategory; do
  if go test -run=xxx -fuzz="^${tgt}$" -fuzztime="${FUZZTIME}" . >/tmp/fuzz.$$ 2>&1; then
    pass "$tgt"
  else
    fail "$tgt (see corpus in testdata/fuzz)"; tail -8 /tmp/fuzz.$$
  fi
done
rm -f /tmp/fuzz.$$

# 7. Mutation testing (test-suite quality; defends against coverage gaming)
if [ "$RUN_MUTATION" = 1 ]; then
  step "mutation testing (gremlins)"
  if need gremlins "go install github.com/go-gremlins/gremlins/cmd/gremlins@latest"; then
    gremlins unleash --tags="" || true   # report-only; review the efficacy score
  else
    fail "gremlins not installed"
  fi
else
  printf "\n(skipping mutation testing; pass --mutation to run it)\n"
fi

# 8. Cross-platform build (the deliverable must compile everywhere)
step "cross-compile"
if ./build.sh >/dev/null 2>&1; then pass "all platform binaries built"; else fail "build.sh failed"; fi

echo
if [ "$FAIL" = 0 ]; then
  printf "\033[32mALL GATES PASSED\033[0m\n"; exit 0
else
  printf "\033[31mONE OR MORE GATES FAILED\033[0m\n"; exit 1
fi
