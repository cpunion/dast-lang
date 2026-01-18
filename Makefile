SHELL := /bin/sh

STAGE0_DIR := compiler/bootstrap/stage0
STAGE0_BIN := $(STAGE0_DIR)/dast-stage0
STAGE1_FILES := compiler/bootstrap/stage1/token.dast compiler/bootstrap/stage1/lexer.dast compiler/bootstrap/stage1/ast.dast compiler/bootstrap/stage1/parser.dast compiler/bootstrap/stage1/typecheck.dast compiler/bootstrap/stage1/ir.dast compiler/bootstrap/stage1/compile.dast compiler/bootstrap/stage1/interp.dast compiler/bootstrap/stage1/main.dast
EXAMPLES := $(wildcard compiler/bootstrap/stage0/examples/*.dast)

.PHONY: build-stage0 test-stage0 test-stage1 test-stage1-full test clean

build-stage0:
	@cd $(STAGE0_DIR) && go build -o dast-stage0 ./cmd/dast

test-stage0: build-stage0
	@for f in $(EXAMPLES); do \
		echo "[stage0] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
	done

test-stage1: build-stage0
	@for f in $(EXAMPLES); do \
		echo "[stage1] $$f"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		echo "$$out" | grep -q '^error' && exit 1 || true; \
	done

test-stage1-full: test-stage1

test: test-stage0 test-stage1

clean:
	@rm -f $(STAGE0_BIN)
