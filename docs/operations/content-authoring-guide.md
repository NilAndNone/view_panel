# Content Authoring Guide

Batch A authoring introduces a richer source tree under `content/src` while the
runtime continues to consume flat compiled persona and material bundles.

Authoring workflow stays split on purpose:

- write/edit source content under `content/src`
- compile with `make build-content` into `content/build/<bundle_id>/runtime/`
- run direct panel or eval workflows only against the compiled bundle, never against source files directly

## Persona source packs

Every persona source pack must contain:

- `meta.json`
- `profile.md`
- `psychology.md`
- `anti_patterns.md`
- `domains/<domain>.md` for every entry declared in `strong_domains`

Every persona directory under `content/src/personas/` must also be declared in
`content/src/personas/index.json`.

The current authored cohort declared there is:

- `analyst`
- `builder`
- `historian`
- `operator`
- `policy`
- `skeptic`

`meta.json` is strict source metadata. Do not add ad hoc keys.
The compiler rejects unknown JSON fields and trailing JSON content instead of
silently dropping them.

`meta.json` must contain exactly the current source metadata keys:

- `persona_id`
- `display_name`
- `core_lens`
- `decision_style`
- `tone`
- `default_bias`
- `priority_stack`
- `strong_domains`
- `reference_anchors`

Compiler-enforced `meta.json` constraints:

- `persona_id` must match `^[a-z][a-z0-9_]*$`, match the persona directory
  name, and be declared in `content/src/personas/index.json`
- `display_name`, `core_lens`, `decision_style`, `tone`, and `default_bias`
  must be non-empty after trimming
- `priority_stack` must contain at least 3 unique, non-empty entries
- `strong_domains` must contain at least 2 unique, non-empty entries
- `reference_anchors` must contain at least 2 unique, non-empty entries

Persona IDs and material IDs must match `^[a-z][a-z0-9_]*$` so compiled
runtime artifact paths remain valid under the bundle manifest contract.

Markdown files must be non-empty and specific. Avoid generic self-help phrasing,
neutralized consensus language, and abstract claims with no operational anchor.

Every domain file must include concrete anchors such as incidents, institutions,
decision rituals, or artifacts that make the persona feel inspectable rather
than interchangeable.

`strong_domains` entries must be template-safe identifiers matching
`^[a-z][a-z0-9_]*$` so compiled fields remain addressable as
`{{.Fields.domain_<domain>}}`.
The `domains/` directory is strict too: it must contain exactly one non-empty
`<domain>.md` file for each declared `strong_domains` entry and no extra files
or subdirectories.

## Materials

Each material source file is a JSON document with:

- `material_id`
- `title`
- `domain`
- `roleplay_prompt`
- `discussion_question`
- `supplementary_materials`
- `output_contract`
- `assumptions_and_constraints`

Material filenames must exactly match `material_id`, for example
`technology.json` for `material_id: "technology"`.

Material source JSON is strict. Do not add ad hoc source-only keys: the
compiler rejects unknown fields instead of silently dropping them. Successful
compilation still emits the canonical runtime five-field material structure
without teaching runtime packages to read the authoring tree directly.

Compiler-enforced material constraints:

- `material_id` must match `^[a-z][a-z0-9_]*$`
- `title` must be non-empty after trimming
- `domain` must match `^[a-z][a-z0-9_]*$`
- `roleplay_prompt`, `discussion_question`, `supplementary_materials`,
  `output_contract`, and `assumptions_and_constraints` must all be present and
  non-empty after trimming
- `materials/` may contain only `.json` files; duplicate `material_id` values
  are rejected, and the source tree must contain at least one material file

Material template placeholders must match the current prepare-stage context.
Use:

- `{{.PersonaID}}`
- `{{.SourcePath}}`
- `{{.Fields.<compiled_field_name>}}`

For persona content, reference compiled fields through `.Fields`, for example
`{{.Fields.display_name}}`, `{{.Fields.core_lens}}`, and
`{{.Fields.domain_technology}}`.

Current compiled persona field inventory is:

- `display_name` from `meta.json.display_name`
- `core_lens` from `meta.json.core_lens`
- `decision_style` from `meta.json.decision_style`
- `tone` from `meta.json.tone`
- `default_bias` from `meta.json.default_bias`
- `priority_stack` as a single string joined with ` | ` from `meta.json.priority_stack`
- `strong_domains` as a single string joined with ` | ` from `meta.json.strong_domains`
- `reference_anchors` as a single string joined with ` | ` from `meta.json.reference_anchors`
- `profile_long` from `profile.md`
- `psychology_summary` from `psychology.md`
- `anti_patterns` from `anti_patterns.md`
- `domain_<domain>` from `domains/<domain>.md` for every declared `strong_domains` entry

The compiled runtime persona JSON exposes those names directly as top-level
keys, and material templates receive the same values through `.Fields.<name>`.

## Distinctness evaluation question sets

Task 4 adds a separate evaluator input at
`content/src/evals/distinctness/questions.json`.

The file is strict JSON with:

- `schema_version`
- `questions`

Each `questions[]` entry must contain:

- `question_id`
- `material_id`
- `anchor_domain`
- `question`

Authoring rules:

- `schema_version` must be `distinctness_questions_v1`
- `questions` must contain at least one entry
- `question_id` must match `^[a-z][a-z0-9_]*$`
- `material_id` must point at an existing compiled runtime material
- `anchor_domain` must match the compiled persona field suffix you want to score against, for example `technology` for `domain_technology`
- `question` must be non-empty after trimming
- duplicate `question_id` values are rejected
- for the compiled bundle under evaluation, every persona must expose a non-empty `domain_<anchor_domain>` field or `make content-eval` fails before scoring
- keep the set intentionally small and deterministic; each question triggers a real panel run during `make content-eval`
- the current distinctness cohort is the live authored default bundle, not an analyst-only placeholder
- prefer prompts that expose persona anchors and decision style, not generic “what do you think” wording

The evaluator does not teach runtime stages about eval concepts. It reuses the
compiled bundle plus the existing `worldview-panel` CLI, then writes
`distinctness-report.json` and `distinctness-report.md` under the chosen eval
output root.

The default `make content-eval` target writes to
`out/content-eval/<bundle_id>/latest/` so later `make build-content
CONTENT_BUNDLE_ID=<bundle_id>` rebuilds can replace
`content/build/<bundle_id>/` without deleting retained eval history.

If you want a different location, override `CONTENT_EVAL_OUT`; for example use
`out/content-eval/<bundle_id>/no_review/`.
If you bypass `make content-eval` and invoke `eval_content_distinctness`
directly, you must still pass `-out` explicitly and that path must live outside
`content/build/<bundle_id>/` so evaluator artifacts do not get mixed back into
the compiled runtime bundle tree.
