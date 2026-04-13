SHELL := /bin/sh

REPO_ROOT := $(CURDIR)
TMP_ROOT ?= $(or $(TMPDIR),$(HOME),/tmp)
LOCAL_TMP_TEMPLATE ?= $(TMP_ROOT)/view_panel_make_XXXXXX
BIN_PATH ?= $(or $(HOME),$(TMP_ROOT),/tmp)/worldview-panel_bin
MIRROR_TAR_EXCLUDES := --exclude=.git --exclude=out --exclude=runs --exclude=content/build --exclude=go_bin
CONTENT_BUNDLE_ID ?= default
CONTENT_BUNDLE_OUT := $(REPO_ROOT)/content/build/$(CONTENT_BUNDLE_ID)
CONTENT_BUNDLE_RUNTIME := $(CONTENT_BUNDLE_OUT)/runtime
CONTENT_EVAL_QUESTIONS := $(REPO_ROOT)/content/src/evals/distinctness/questions.json
CONTENT_EVAL_OUT ?= $(REPO_ROOT)/out/content-eval/$(CONTENT_BUNDLE_ID)/latest
CONTENT_FIXTURE_SOURCE := $(REPO_ROOT)/testdata/content/builder_fixture/src
SMOKE_OUT := $(REPO_ROOT)/out/smoke
SMOKE_RUNS_DIR := $(SMOKE_OUT)/runs
SMOKE_MATERIAL := $(REPO_ROOT)/testdata/smoke/materials/smoke.json
SMOKE_PERSONA_SET := $(REPO_ROOT)/testdata/smoke/runtime/persona-index.json
SMOKE_QUESTION ?= smoke test question
SMOKE_CONCURRENCY ?= 1

.PHONY: help build build-content content-eval content-smoke-build smoke smoke-check capture-protocol

help:
	@echo "Targets:"
	@echo "  make build"
	@echo "  make build-content"
	@echo "  make content-eval"
	@echo "  make capture-protocol"
	@echo "  make content-smoke-build"
	@echo "  make smoke"
	@echo "  make smoke-check RUN_ID=<run_id>"
	@echo "Overrides:"
	@echo "  BIN_PATH=/absolute/path/to/worldview-panel"
	@echo "  TMP_ROOT=/absolute/path/for-local-mirror-tempdirs"
	@echo "  CONTENT_BUNDLE_ID=<bundle_id>"
	@echo "  CONTENT_EVAL_OUT=/absolute/path/for-distinctness-eval-output"

build:
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	mkdir -p "$$(dirname "$(BIN_PATH)")"; \
	cd "$$tmpdir"; \
	go build -o "$(BIN_PATH)" ./cmd/worldview-panel

build-content:
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go run ./tools/build_content_bundle -source "$$tmpdir/content/src" -out "$(CONTENT_BUNDLE_OUT)" -bundle-id "$(CONTENT_BUNDLE_ID)"

content-eval: build-content
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go run ./tools/eval_content_distinctness -bundle "$(CONTENT_BUNDLE_OUT)" -questions "$(CONTENT_EVAL_QUESTIONS)" -out "$(CONTENT_EVAL_OUT)"

content-smoke-build:
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	smoke_out="$$tmpdir/out/content-smoke-build"; \
	go run ./tools/build_content_bundle -source "$$tmpdir/testdata/content/builder_fixture/src" -out "$$smoke_out" -bundle-id smoke; \
	test -f "$$smoke_out/bundle-manifest.json"; \
	test -f "$$smoke_out/runtime/persona-index.json"; \
	test -f "$$smoke_out/runtime/personas/analyst.json"; \
	test -f "$$smoke_out/runtime/materials/technology.json"

smoke:
	@set -eu; \
	rm -rf "$(SMOKE_OUT)"; \
	mkdir -p "$(SMOKE_OUT)"; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go build -o "$$tmpdir/go_bin" ./cmd/worldview-panel; \
	"$$tmpdir/go_bin" \
	  -question "$(SMOKE_QUESTION)" \
	  -materials "$$tmpdir/testdata/smoke/materials/smoke.json" \
	  -persona-set "$$tmpdir/testdata/smoke/runtime/persona-index.json" \
	  -outdir "$(SMOKE_OUT)" \
	  -concurrency "$(SMOKE_CONCURRENCY)"; \
	run_id="$$(ls -1 "$(SMOKE_RUNS_DIR)" 2>/dev/null | sort | tail -n 1)"; \
	if [ -z "$$run_id" ]; then \
	  echo "smoke failed: no run_id found under $(SMOKE_RUNS_DIR)" >&2; \
	  exit 1; \
	fi; \
	$(MAKE) -C "$(REPO_ROOT)" smoke-check RUN_ID="$$run_id"

smoke-check:
	@set -eu; \
	if [ -z "$(RUN_ID)" ]; then \
	  echo "RUN_ID is required, e.g. make smoke-check RUN_ID=<run_id>" >&2; \
	  exit 1; \
	fi; \
	run_root="$(SMOKE_RUNS_DIR)/$(RUN_ID)"; \
	if [ ! -d "$$run_root" ]; then \
	  echo "run root not found: $$run_root" >&2; \
	  exit 1; \
	fi; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go run ./tools/smokecheck --run-root "$$run_root"

capture-protocol:
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - $(MIRROR_TAR_EXCLUDES) . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	capture_output="$$(go run ./tools/capture_protocol_bundle)"; \
	printf '%s\n' "$$capture_output"; \
	bundle_root="$${capture_output##*at }"; \
	if [ -z "$$bundle_root" ]; then \
	  echo "capture-protocol failed: no bundle root found under $$tmpdir/third_party/codex-protocol" >&2; \
	  exit 1; \
	fi; \
	version="$$(basename "$$bundle_root")"; \
	dest_root="$(REPO_ROOT)/third_party/codex-protocol/$$version"; \
	mkdir -p "$$dest_root"; \
	for name in openapi.json messages.json schema-index.json smoke-transcript.jsonl; do \
	  cp "$$bundle_root/$$name" "$$dest_root/$$name"; \
	done
