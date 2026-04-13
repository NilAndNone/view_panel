SHELL := /bin/sh

REPO_ROOT := $(CURDIR)
LOCAL_TMP_TEMPLATE := /data/data/com.termux/files/home/view_panel_make_XXXXXX
BIN_PATH := $(REPO_ROOT)/go_bin
SMOKE_OUT := $(REPO_ROOT)/out/smoke
SMOKE_RUNS_DIR := $(SMOKE_OUT)/runs
SMOKE_MATERIAL := $(REPO_ROOT)/testdata/smoke/materials/smoke.json
SMOKE_PERSONA_SET := $(REPO_ROOT)/testdata/smoke/runtime/persona-index.json
SMOKE_QUESTION ?= smoke test question
SMOKE_MODEL ?= gpt-5.4
SMOKE_CONCURRENCY ?= 1

.PHONY: help build smoke smoke-check capture-protocol

help:
	@echo "Targets:"
	@echo "  make build"
	@echo "  make capture-protocol"
	@echo "  make smoke"
	@echo "  make smoke-check RUN_ID=<run_id>"

build:
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - --exclude=.git . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go build -o "$(BIN_PATH)" ./cmd/worldview-panel

smoke: build
	@set -eu; \
	rm -rf "$(SMOKE_OUT)"; \
	mkdir -p "$(SMOKE_OUT)"; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - --exclude=.git . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go build -o "$$tmpdir/go_bin" ./cmd/worldview-panel; \
	"$$tmpdir/go_bin" \
	  -question "$(SMOKE_QUESTION)" \
	  -materials "$$tmpdir/testdata/smoke/materials/smoke.json" \
	  -persona-set "$$tmpdir/testdata/smoke/runtime/persona-index.json" \
	  -outdir "$(SMOKE_OUT)" \
	  -concurrency "$(SMOKE_CONCURRENCY)" \
	  -model "$(SMOKE_MODEL)"; \
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
	tar -cf - --exclude=.git . | (cd "$$tmpdir" && tar -xf -); \
	cd "$$tmpdir"; \
	go run ./tools/smokecheck --run-root "$$run_root"

capture-protocol:
	@set -eu; \
	tmpdir="$$(mktemp -d "$(LOCAL_TMP_TEMPLATE)")"; \
	trap 'rm -rf "$$tmpdir"' EXIT INT TERM; \
	tar -cf - --exclude=.git . | (cd "$$tmpdir" && tar -xf -); \
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
