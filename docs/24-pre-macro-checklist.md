# Pre-Macro Readiness Checklist (Stage0/Stage2)

This checklist defines the **mandatory completion gates before any macro work**.  
Order is strict. Do not start macro work until all gates are green.

---

## Scope

Applies to:
- Stage0 compiler (Go bootstrap)
- Stage2 compiler (Dast)
- Shared tests (compiler/tests)
- Prelude / compiler-provided builtins

Goal: Ensure core types + control flow + generics/methods + drop are correct and stable **before macros**.

---

## Gate 1 — Prelude & String/Str Unification (Required)

**Goal:** Unify types to **String / str / &String / &str** only.  
User code must not use `string` (lowercase). Diagnostics must use `String`/`str`.

**Requirements:**
1. Prelude API uses `&str` for read-only inputs:
   - `len`, `char_at`, `substr`, string comparison helpers, file IO APIs, etc.
2. Stage2 stdlib/prelude contains the canonical signatures.
3. Stage0 compiler builtins mirror the same signatures and names.
4. `string` (lowercase) is **forbidden** in user code:
   - parser/typechecker emits clear error message: use `String`/`str`.

**Validation:**
- Compile and run basic string tests in `compiler/tests/run-pass/*string*`.
- Diagnostics for `string` usage are stable and readable.

---

## Gate 2 — Drop + Control Flow Correctness (Required)

**Goal:** Drop is correct under all core control-flow shapes.

**Coverage must include:**
- Straight-line scope end
- Early return
- Nested scopes
- If/else
- Loop break/continue
- Match arms
- Arrays of drop types
- Struct fields with drop types
- Enums with payload drop types

**Validation:**
- Shared tests in `compiler/tests/run-pass` include `drop_*`.
- Stage0 + Stage2 both pass drop tests.
- No leaks or double-drop for these cases.

---

## Gate 3 — Generics + Methods Coverage (Required)

**Goal:** Generics and impl methods behave correctly end-to-end.

**Must support:**
- Generic functions (`fn f[T](...)`)
- Generic structs and enums
- Impl blocks on generic types
- Trait impls on generic types
- Method call dispatch with generics

**Validation:**
- Shared tests in `compiler/tests/run-pass` for generic functions and impl methods.
- Stage0 + Stage2 both pass the same tests.

---

## Gate 4 — Macro Work Starts Only After Gates 1–3

Macro work (compile! / quote / AST splice) is blocked until:
- Gate 1 ✅
- Gate 2 ✅
- Gate 3 ✅

---

## Shared Test Policy

- All language tests should live in `compiler/tests` and be used by both stage0 and stage2.
- Stage-specific tests are allowed only when behavior is intentionally different and documented.
- Use **simple → combo → integration** ordering.

---

## Definition of Done

All gates are green and:
- Stage0 passes its test suite.
- Stage2 passes the same shared test suite.
- No unresolved performance or memory regressions for the covered cases.

---

## Known Regression Log (keep current)

### 2026-01-28 — Stage2 crash on `010_char_lit.dast`

**Symptom:** Stage2 compiler crashes at runtime (stage0 runtime OOB) while compiling even a minimal program.

**Repro:**
```bash
./compiler/stage2/target/dast-stage2 run compiler/tests/run-pass/010_char_lit.dast
./compiler/stage2/target/dast-stage2 run /tmp/dast_min.dast
```

**Runtime error:**
```
stage0: <runtime>:0:0: error index 0 out of bounds len=0 id=2307 (or 2637)
```

**Timeline notes / suspected trigger:**
- Regression appeared after adding generic enums in prelude (Option/Result) and then extending enum generics support in
  stage2 parser + compiler (EnumDecl type_params + EnumSig type_params + type_subst in enum payload/tag).
- The failure persists with a minimal program (no macros), so it’s likely a compiler-internal AST/expr-args bug rather
  than user code.

**Status:** Investigating. Add backtrace via `DAST_ARRAY_DEBUG_ID`/`DAST_ARRAY_BT` and locate the empty-args expression
creator. Consider bisect from last known passing state if needed.
