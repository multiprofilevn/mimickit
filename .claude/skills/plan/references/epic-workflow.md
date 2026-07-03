---
title: "Epic Workflow"
scope: "plan epic subcommand"
---

# Epic Workflow (`/plan epic`)

An **epic** is a container that groups related `plan-NN-<slug>/` child plans under a single
`epic.md` index. Use an epic when a large initiative spans multiple independent plans that must
execute in a dependency-ordered sequence.

## When to Use

Use `/plan epic` when:
- The initiative has **3+ child plans** with blockedBy dependencies between them.
- Different plans may be cooked independently in separate sessions.
- You need a single-glance status view across all child plans.

For a single plan with multiple phases, use `/plan` + `/cook` directly — no epic needed.

## Detail-Dependency vs Cook-Dependency

Two distinct questions, two distinct mechanisms — do not conflate them:

| Question | Mechanism | Controls |
|----------|-----------|----------|
| When can this plan **run** (cook)? | `blockedBy` frontmatter | `next` (pick plan to cook) |
| When can this plan's **phases be written** (detail)? | `detailMode` + stub-state | `next-to-detail` (pick plan to detail) |

- **`detailMode: sequential`** (default, safe) — detailing a later plan needs the **OUTPUT** of an
  upstream plan (e.g. plan-01 reverse-engineers a spec; plans 02-05 can only be detailed accurately
  once that spec exists). Detail in waves: only stubs whose `blockedBy` are `completed` are detailed
  now; the rest are detailed at resume time after upstream is cooked. Never guess phases against
  output that does not exist yet.
- **`detailMode: upfront`** — every child plan can be detailed from **static CONTEXT** already
  available (no plan depends on another's runtime output). Detail all stubs immediately, in parallel.

Picking the wrong mode has a clear failure signature: `upfront` on an output-dependent epic produces
hand-wavy phases written against a spec that isn't there. When unsure, prefer `sequential`.

## Subcommand Flow

```
/plan epic <epic-id>
```

1. **Scaffold + classify** — Create `plans/<epic-id>/` (or current dir if path given):
   - Stub child plan dirs: `plan-NN-<slug>/plan.md` for each named child plan (overview only).
   - **Classify the epic's `detailMode` BEFORE init** (classify-first): auto-infer from the child
     plans' `blockedBy` (all empty → suggest `upfront`; chained `blockedBy` → suggest `sequential`),
     then confirm/override with the user. See "Detail-Dependency vs Cook-Dependency" below.
   - Init state with the chosen mode:
     ```bash
     node .claude/hooks/epic-state.cjs init <epic-dir> --detail-mode <sequential|upfront>
     ```
     Writes `epic-state.json` `{epicId, plans:[], status:"draft", detailMode}` + scaffolds `epic.md`.
     Omit the flag → defaults to `sequential` (safe). There is **no** separate `set-detail-mode`
     command — pass the mode to `init`.

2. **Detail child plans (auto)** — Each child `plan.md` MUST have flat frontmatter:
   ```
   status: pending     # pending | in-progress | completed | blocked
   blockedBy: [plan-NN-slug, ...]   # bare dir names, same epic scope
   ```
   Dependency truth lives **only** in child `plan.md` frontmatter. `epic.md` is index-only —
   never parse it for dependencies.

   Phase files are filled via `/plan` (standard planning flow), driven by `next-to-detail`:
   ```bash
   node .claude/hooks/epic-state.cjs next-to-detail <epic-dir>
   ```
   Prints stub plans (0 phase files) ready to detail, one dir per line:
   - `upfront` → all stubs (skill fans out `planner` agents in parallel to detail every child at once).
   - `sequential` → only stubs whose `blockedBy` are all `completed` (detail in waves; later plans
     are detailed at resume time, once upstream OUTPUT exists).
   Prints `no-plan-to-detail` and exits 0 when nothing is ready to detail.

   **Stub-detection:** a plan dir with **0** `phase-*.md` files is treated as "not yet detailed".
   After detailing, every child has ≥1 phase file. (No `detailed` frontmatter flag — file count is
   the source of truth.)

3. **Rollup** — After editing child plans, refresh epic state:
   ```bash
   node .claude/hooks/epic-state.cjs rollup <epic-dir>
   ```
   Reads each `plan-NN-*/plan.md`, aggregates `status`/`blockedBy`, writes `epic-state.json`,
   refreshes the `epic.md` table. **`rollup` fully regenerates `epic.md`** (machine-managed) —
   do NOT add human notes there, they will be overwritten; put notes in child `plan-NN-*/plan.md`.

4. **Select next plan** — Before each cook session:
   ```bash
   node .claude/hooks/epic-state.cjs next <epic-dir>
   ```
   Returns the dir name of the next READY plan (`status: pending` AND all `blockedBy` entries
   are `completed`). Tie-break: fewest phase files first (smallest blast radius).
   Prints `no-ready-plan` and exits 0 when nothing is ready — signal to pause.

5. **Run/resume** — Cook the selected plan:
   ```bash
   /cook plans/<epic-id>/<ready-plan-dir>/
   ```
   This is standard `/cook` — no new command. `--resume` flag (Phase 3) handles cross-session
   state restoration.

6. **Check overall state** — At any point:
   ```bash
   node .claude/hooks/epic-state.cjs check <epic-dir>
   ```
   Prints `{status, totalPlans, readyPlans, plansToDetail, detailMode, isComplete}` JSON to stdout.

## Auto-Detail Orchestration (skill behavior)

`/plan epic` drives detailing automatically — the user no longer hand-prompts "now detail plan-02":

1. **Scaffold stubs** → write each `plan-NN-<slug>/plan.md` overview (status, blockedBy, summary).
2. **Classify (auto-infer + confirm)** → inspect child `blockedBy`: all empty → recommend `upfront`;
   chained → recommend `sequential`. Open `AskUserQuestion` (header `Detail Mode`) with the inferred
   option marked `(Recommended)`; each option `description` ≥ 40 chars (decision-visibility gate).
   User confirms or overrides → `init --detail-mode <chosen>`.
3. **Auto-detail the ready set** → `next-to-detail <epic-dir>`:
   - `upfront` → returns all stubs → spawn one `planner` agent **per stub in parallel** (each writes
     phase files into its own `plan-NN-<slug>/` — no shared-file contention; rollup runs once after).
   - `sequential` → returns only the first wave (no-blocker stubs) → detail those now; later waves are
     detailed at resume time (see "Resume auto-detail" below).
4. **Rollup** → `rollup <epic-dir>` to refresh `epic-state.json` + `epic.md`.
5. **Hand off** → print a table: which child plans were detailed now vs deferred (and why — "waits on
   OUTPUT of plan-NN"), then offer cooking the first ready-to-cook plan.

**Planner spawn template** (context isolation per `orchestration-protocol.md`):
```
Task: Detail phase files for <plan-NN-slug> in epic <epic-id>
Agent: planner
Context:
  - Child plan dir: plans/<epic-id>/<plan-NN-slug>/
  - Plan overview: <plan.md summary + acceptance from the overview>
  - Epic context: <epic goal; upstream OUTPUT available if sequential>
  - Work context: <project root>
Acceptance: phase-*.md files written into the child dir per the canonical phase template.
```

This is PLANNING only (writes phase files) — it never cooks, so it does not violate the cook
HARD-GATE or the epic human-checkpoint.

### Resume auto-detail (sequential)

For `sequential` epics, detailing the next wave happens at the **start of the resume session**
(`/cook <epic>/<next> --resume`), before cooking the target plan — once upstream OUTPUT exists.
The epic boundary after a cook still only snapshots + stops (human-checkpoint unchanged). Contract:
`.claude/rules/phase-completion-gate.md` → "Epic-boundary exception".

## Epic State Machine

```
draft → running → passed
            ↓
          blocked   (any child plan has status: blocked)
```

Derived from children:
- All `completed` → `passed`
- Any `blocked` → `blocked`
- Any `pending`/`in-progress` → `running`
- No children yet → `draft`

## epic-state.json Schema

```json
{
  "epicId": "260621-1708-my-epic",
  "plans": [
    { "dir": "plan-01-foundation", "status": "completed", "blockedBy": [] },
    { "dir": "plan-02-feature",    "status": "pending",   "blockedBy": ["plan-01-foundation"] }
  ],
  "status": "running",
  "detailMode": "sequential",
  "createdAt": "2026-06-21T00:00:00.000Z",
  "lastUpdated": "2026-06-21T12:00:00.000Z"
}
```

`epic-state.json` is the machine source of truth. `epic.md` is the human-readable index —
never parse `epic.md` for machine decisions.

## File Ownership

| File | Owner | Parse? |
|------|-------|--------|
| `epic-state.json` | `epic-state.cjs` | Yes — machine state |
| `epic.md` | Human + `epic-state.cjs rollup` | No — index only |
| `plan-NN-*/plan.md` | `/plan` skill | Yes — `status` + `blockedBy` flat frontmatter |

## Single-Cook-Per-Epic Limitation

Running two cook sessions concurrently on the same epic causes last-write-wins on
`epic-state.json` — no file lock is implemented (YAGNI for human-checkpoint workflow).
Run one plan at a time within an epic. This matches the same documented limitation in
`phase-completion-gate.md` for `cook-state.json`.

## Plans Under gitignore

`plans/` is gitignored (`.gitignore:14`). `epic-state.json` is a local-session artifact.
Cross-machine resume is out of scope (tracked for Phase 3 snapshot location design).

## Hook Script Reference

Script: `.claude/hooks/epic-state.cjs`

```
node .claude/hooks/epic-state.cjs init           <epic-dir> [--detail-mode sequential|upfront]
node .claude/hooks/epic-state.cjs rollup         <epic-dir>   # preserves detailMode
node .claude/hooks/epic-state.cjs next           <epic-dir>   # next plan to COOK
node .claude/hooks/epic-state.cjs next-to-detail <epic-dir>   # stub plans to DETAIL (per detailMode)
node .claude/hooks/epic-state.cjs check          <epic-dir>
```

Exit codes: `0` = OK / fail-open, `1` = bad args / write failure.
Fail-open: malformed child `plan.md` or corrupt `epic-state.json` → treated as no-state,
execution continues gracefully.
