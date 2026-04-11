# Tech Plan V1 Execution Guide

## Purpose

Execute the plan set for `docs/tech-plan-v1.md` using the files in `docs/plans/tech-plan-v1/`.

## Plan Directory

`docs/plans/tech-plan-v1/`

## Execution Graph

```text
P01 -> P02
P03 -> P04
P02 -> P04
P03 -> P05
P04 -> P06
P05 -> P06
P06 -> P07
P07 -> P08
P08 -> P09
P09 -> P10
P10 -> P11
```

## Dispatch Rules

1. Pick the lowest-numbered incomplete plan whose `depends_on` entries are all `done`.
2. Read only `AGENTS.md`, `docs/plans/tech-plan-v1/README.md`, the target plan file, and directly required dependency plans.
3. Do not start implementation from `README.md` alone.
4. Do not merge multiple plans into one execution unless the target plan says so.
5. After finishing a plan, update both the target plan frontmatter and `docs/plans/tech-plan-v1/_index.json`.

## Source Of Truth Order

1. The target plan file
2. `docs/plans/tech-plan-v1/_index.json`
3. `AGENTS.md`
4. `docs/plans/tech-plan-v1/README.md`
