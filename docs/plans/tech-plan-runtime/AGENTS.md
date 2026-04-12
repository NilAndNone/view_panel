# Tech Plan Runtime Execution Guide

## Purpose

Execute the retained runtime plan set using the files in `docs/plans/tech-plan-runtime/`.

## Plan Directory

`docs/plans/tech-plan-runtime/`

## Execution Graph

```text
P00 -> P02
P01 -> P02
P03 -> P04
P03 -> P05
P03 -> P07
P03 -> P10
P04 -> P05
P04 -> P06
P05 -> P07
P01 -> P08
P02 -> P08
P07 -> P08
P08 -> P09
P09 -> P10
P02 -> P11
P10 -> P11
P11 -> P12
```

## Dispatch Rules

1. Pick the lowest-numbered incomplete plan whose `depends_on` entries are all `done`.
2. Read only this `AGENTS.md`, `README.md`, the target plan file, and directly required dependency plans.
3. Do not start implementation from `README.md` alone.
4. `dispatch_input_v1` uses only these canonical field names:
   - `roleplay_prompt`
   - `discussion_question`
   - `supplementary_materials`
   - `output_contract`
   - `assumptions_and_constraints`
5. Stage 2 plans must not reopen `dispatch_input_v1.json`.
6. Worker execution must stay outside the repo tree, with isolated `cwd`, controlled `HOME`, and controlled `CODEX_HOME`.
7. Raw and certified render outputs are separate permanent boundaries. Do not collapse them into one path.
8. Do not merge multiple plans into one execution unless the target plan says so.
9. After finishing a plan, update both the target plan frontmatter and `docs/plans/tech-plan-runtime/_index.json`.

## Source Of Truth Order

1. The target plan file
2. `docs/plans/tech-plan-runtime/_index.json`
3. `docs/plans/tech-plan-runtime/AGENTS.md`
4. `docs/plans/tech-plan-runtime/README.md`
