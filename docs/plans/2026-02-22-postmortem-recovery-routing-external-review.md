# Postmortem Recovery Routing: External Review Brief

## Context

This brief evaluates whether Kilroy's default DOT topology over-rotates toward broad rollback after failures, and proposes a constrained change for review.

The scope here is the default ingest path (english-to-dotfile skill + reference template), not core engine semantics.

## Problem

The default topology routes most deterministic implementation/check failures to `postmortem`, then unconditionally routes `postmortem -> check_toolchain`.

In practice, this means many code-local failures (for example compile/test/contract gaps) re-enter bootstrap/planning stages before another implementation attempt, even when prerequisites and plan artifacts are still valid.

Observed evidence:
- Deterministic check failures route to postmortem:
  - `skills/english-to-dotfile/reference_template.dot:325`
  - `skills/english-to-dotfile/reference_template.dot:336`
  - `skills/english-to-dotfile/reference_template.dot:342`
  - `skills/english-to-dotfile/reference_template.dot:348`
  - `skills/english-to-dotfile/reference_template.dot:357`
- Postmortem currently routes to broad rollback start:
  - `skills/english-to-dotfile/reference_template.dot:370`
- This route is enforced by guardrail test:
  - `internal/attractor/validate/reference_template_guardrail_test.go:139`
  - `internal/attractor/validate/reference_template_guardrail_test.go:151`
- Ingest generation follows this default topology path:
  - `internal/attractor/ingest/ingest_prompt.tmpl:1`
  - `skills/english-to-dotfile/SKILL.md:111`

Why this matters:
- Higher token/tool cost on repeated planning/bootstrap for code-local repair scenarios.
- Slower convergence due to repeated early-stage work.
- Tension with the Prime Directive objective to improve system-wide defaults across projects.

## Proposed Solution

Introduce dual-path postmortem recovery in the reference template:

1. Fast repair lane (default for deterministic, non-toolchain failures):
   - `postmortem -> implement` (or `check_implement`, whichever preserves current invariants best).
2. Safety lane (keep existing broad rollback):
   - `postmortem -> check_toolchain` for transient infra/toolchain failures and escalation fallback.
3. Escalation guard:
   - After `N` failed fast-repair iterations, force one broad rollback cycle (`check_toolchain -> ...`) to avoid local minima.

Design intent:
- Preserve battle-tested safety behavior.
- Reduce unnecessary bootstrap/planning replay for code-level repair loops.
- Keep change template/guardrail-first (minimal or no engine semantic changes).

## Constraints

1. Preserve Attractor semantics and spec alignment.
- Engine/spec do not hardcode `postmortem`/`check_toolchain`; routing is graph-defined:
  - `docs/strongdm/attractor/attractor-spec.md:563`
  - `internal/attractor/engine/next_hop.go:25`

2. Preserve template-first ingest contract.
- The english-to-dotfile skill and ingest prompt make template topology the practical default:
  - `skills/english-to-dotfile/SKILL.md:111`
  - `internal/attractor/ingest/ingest_prompt.tmpl:1`

3. Preserve failure-class restart discipline.
- Restart edges on fail paths should remain guarded to transient infra where appropriate:
  - `docs/strongdm/attractor/attractor-spec.md:1509`

4. Preserve battle-tested safety net.
- Do not remove broad rollback path entirely; retain as explicit fallback.

5. Keep implementation footprint small.
- Prefer template + guardrail updates before considering engine-level behavior changes.

## Pros

1. Lower average cost and latency for deterministic repair loops.
2. Faster convergence on code-local failures by avoiding unnecessary bootstrap/planning replay.
3. Better preservation of working implementation context between iterations.
4. Minimal architectural risk if implemented as template/guardrail change with explicit fallback.

## Cons

1. Increased routing complexity in the default template (more conditional paths).
2. More sensitivity to failure classification quality (`context.failure_class`).
3. Risk of local-minimum loops if fast repair lane is not capped/escalated.
4. Requires guardrail/test updates and clear documentation so template behavior remains predictable.

## Recommended Boundaries

1. Do not remove `postmortem -> check_toolchain`; make it fallback/escalation path.
2. Add explicit fast-lane guard conditions rather than unconditional shortcuting.
3. Add/adjust guardrail tests to require:
   - presence of deterministic fast-repair path,
   - presence of broad safety fallback,
   - absence of unconditional bypasses that defeat prerequisite gates.
4. Keep rollout template-scoped first; evaluate engine changes only if template-scoped approach is insufficient.

## Reviewer Questions

1. Is a capped fast-repair lane acceptable as the new default reliability/cost balance?
2. Should escalation to broad rollback be count-based, signal-based (failure signature), or both?
3. Should fast-lane entry exclude known prerequisite/toolchain classes beyond `transient_infra`?
