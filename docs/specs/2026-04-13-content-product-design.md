# Content Product Line Design

## Goal

Build a separate content workflow that raises persona distinctness and product quality without reopening the runtime architecture. The runtime remains the execution engine; this design adds a source-of-truth content layer, a compiler that emits runtime-ready bundles, and an evaluation path that measures whether persona outputs are meaningfully different.

## Scope

This workstream covers:
- persona source packs with stronger authoring structure than the current flat JSON persona files
- domain material packs that can be compiled into the runtime's canonical five-field material contract
- a build tool that emits runtime-compatible persona-set and material artifacts
- a distinctness evaluation tool and a small content smoke surface
- authoring docs plus an initial production cohort of stronger personas

This workstream does not cover:
- changing Stage 1/2/3 artifact names
- changing the runtime dispatch schema
- changing render semantics
- replacing the runtime's existing persona loader contract

## Current Constraints

The current runtime already supports arbitrary persona fields through `internal/persona.Library` and template expansion through `internal/stage/prepare/prepare.go`. That means the content layer should compile rich persona source packs into the same flat JSON shape the runtime already consumes, instead of changing runtime code paths first.

The current smoke personas are intentionally thin. They prove runtime wiring, not product value. The content line should treat them as fixtures, not as the target quality bar.

## Architecture

### 1. Source content tree

Add an authoring tree under `content/src/`:
- `content/src/personas/index.json`
- `content/src/personas/<persona_id>/meta.json`
- `content/src/personas/<persona_id>/profile.md`
- `content/src/personas/<persona_id>/psychology.md`
- `content/src/personas/<persona_id>/anti_patterns.md`
- `content/src/personas/<persona_id>/domains/<domain>.md`
- `content/src/materials/<domain>.json`
- `content/src/evals/distinctness/questions.json`

`meta.json` holds stable structured fields. Markdown files hold longer human-authored content.

### 2. Compiler boundary

Add a separate content compiler instead of teaching the runtime to read the rich authoring tree directly.

The compiler will emit a runtime-ready bundle under `content/build/<bundle_id>/runtime/`:
- `persona-index.json`
- `personas/<persona_id>.json`
- `materials/<domain>.json`
- `bundle-manifest.json`

Compiled persona JSON remains compatible with the current loader: one `persona_id` plus flat fields. The compiler is responsible for flattening `meta.json` and markdown fragments into a deterministic field map such as `profile_long`, `psychology_summary`, `anti_patterns`, `domain_finance`, and `domain_technology`.

### 3. Distinctness evaluation

Add a separate evaluation tool that uses the existing CLI against a compiled bundle. It should not change runtime stage logic. The evaluator runs a curated question set, captures outputs, and scores problems such as:
- near-duplicate answers across personas
- repeated generic framing
- missing domain anchors
- failure to respect anti-pattern exclusions

The output should be a report artifact under `content/build/<bundle_id>/evals/<run_id>/` so content quality can improve independently of runtime code.

### 4. Initial rollout strategy

Do not try to author every persona from scratch in one pass. Start with a structured pilot cohort that exercises different viewpoints and writing styles, then expand.

The initial cohort should include at least:
- `analyst`
- `builder`
- `historian`
- `operator`
- `skeptic`
- `policy`

Each persona pack must include:
- a long-form profile
- psychology notes
- anti-pattern guidance
- 2 to 3 strongest domains
- concrete references or anchors

## Data Model

### Persona source pack

`meta.json` should include:
- `persona_id`
- `display_name`
- `core_lens`
- `decision_style`
- `tone`
- `default_bias`
- `priority_stack`
- `strong_domains`
- `reference_anchors`

Markdown files provide long-form content:
- `profile.md`: who this persona is and how they think
- `psychology.md`: motivations, blind spots, conflict style
- `anti_patterns.md`: what generic or wrong answers this persona tends to drift into and must avoid
- `domains/<domain>.md`: domain-specific heuristics and anchors

### Domain material pack

Each `content/src/materials/<domain>.json` remains compile-time input, not direct runtime input. The compiler emits the runtime five-field material JSON that the CLI already understands.

## Error Handling

- Missing persona files are build errors
- Unknown fields in `meta.json` are build errors
- Duplicate `persona_id` entries are build errors
- Missing required domain files for declared `strong_domains` are build errors
- Conflicting compiled field names are build errors
- Distinctness evaluation never blocks bundle build; it emits a scored report

## Validation Strategy

Validation happens at three layers:
1. schema validation for source content files
2. compiler validation for deterministic bundle output
3. evaluator validation for content quality signals

## Success Criteria

This line is successful when:
- source content lives in a richer authoring format than flat runtime persona JSON
- the compiler produces runtime-ready bundles without changing the runtime contract
- there is a repeatable distinctness report for a curated question set
- at least one stronger non-smoke persona cohort exists and is runnable through the current CLI
