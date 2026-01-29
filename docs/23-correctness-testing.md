# Correctness Testing Standard (Spec + Codegen)

Goal: ensure each language feature is **semantically correct** *and* **codegen correct**, not just “compiles without errors”.

This standard is required for both Stage0 and Stage2.

---

## The 4-Layer Correctness Model

Each feature should be validated in (at least) these layers:

1) Semantics (frontend truth)
- `compile-fail`: wrong programs must fail with diagnostics.
- `run-pass`: correct programs must run and produce verifiable output.

2) IR invariants (middle-end truth)
- IR must pass validation (`ir_validate` / `ir-verify`).
- IR shape should be constrained for key features.

3) Codegen snapshots (backend truth)
- IR → QBE output must be stable for key feature cases.
- Use snapshot tests for minimal, representative programs.

4) Cross-backend / cross-stage equivalence (ultimate truth)
- Same input should behave the same across:
  - interpreter vs native (when both exist)
  - stage0 vs stage2

---

## Required Test Artifacts Per Feature

When adding or modifying a feature, aim to add the following “4-piece set”:

1) `compiler/tests/compile-fail/<feature>*.dast`
2) `compiler/tests/run-pass/<NNN>_<feature>*.dast`
3) `compiler/tests/ir-gen/<feature>*.dast` (+ paired `.ir` snapshot)
4) `compiler/tests/ir-qbe/<feature>*.ir` (+ paired `.qbe` snapshot)

Notes:
- Keep cases **minimal but decisive**.
- Prefer deterministic outputs (use explicit prints / asserts).
- Use numeric prefixes in `run-pass` to control ordering.
- Avoid large combinatorial test sets in `run-pass`/`compile-fail`. Prefer `*_test.dast` unit tests (loop/assert style)
  and small representative compile-fail samples. Large matrices significantly slow compilation and can mask real
  performance issues in the compiler/runtime.

---

## Shared Test Locations (Stage0 + Stage2)

Language-level correctness tests should be shared:

- Shared language tests:
  - `compiler/tests/run-pass/`
  - `compiler/tests/compile-fail/`
- Shared IR/codegen snapshots:
  - `compiler/tests/ir-gen/`
  - `compiler/tests/ir-qbe/`

Stage-specific tests are still allowed when needed:
- Stage0-specific: `compiler/bootstrap/stage0/tests/`
- Stage2-specific: `compiler/stage2/tests/`

Makefile note:
- `make test-stage0` and `make test-ir-*` now also look in
  `compiler/tests/ir-gen` and `compiler/tests/ir-qbe` when present.

---

## Minimum Verification Commands

Use these as the default verification sequence:

1) Stage0 full verification:
- `make test-stage0`
- `make test-ir-gen`
- `make test-ir-qbe`

2) Stage2 verification (when stable enough):
- `make test-stage2`

If a feature touches IR/codegen, do not stop at run-pass.

---

## Feature Acceptance Checklist

A feature is “correct enough to land” when:

- compile-fail and run-pass exist and pass
- IR validates
- IR/QBE snapshots exist for key paths
- (when feasible) stage0 and stage2 agree on behavior
