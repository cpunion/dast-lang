SHELL := /bin/sh

STAGE0_DIR := compiler/bootstrap/stage0
STAGE0_BIN := $(STAGE0_DIR)/dast-stage0
STAGE1_FILES := compiler/bootstrap/stage1/token.dast compiler/bootstrap/stage1/lexer.dast compiler/bootstrap/stage1/ast.dast compiler/bootstrap/stage1/parser.dast compiler/bootstrap/stage1/typecheck.dast compiler/bootstrap/stage1/ir.dast compiler/bootstrap/stage1/compile.dast compiler/bootstrap/stage1/interp.dast compiler/bootstrap/stage1/main.dast
STAGE2_FILES := $(shell find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' -not -path 'compiler/stage2/stdlib/*' -not -path 'compiler/stage2/backend/codegen/*' | sort)
STAGE1_IR := compiler/bootstrap/stage1/stage1.ir
IR_TEST_DIR := compiler/bootstrap/stage0/tests/ir
IR_VALID := $(IR_TEST_DIR)/valid.ir
IR_INVALID := $(IR_TEST_DIR)/invalid_missing_term.ir
IR_OPT := $(IR_TEST_DIR)/opt_branch.ir
IR_UNDEF := $(IR_TEST_DIR)/invalid_undef_var.ir
IR_UNINIT := $(IR_TEST_DIR)/invalid_maybe_uninit.ir
IR_TERM := $(IR_TEST_DIR)/invalid_term_not_last.ir
IR_JUMP := $(IR_TEST_DIR)/invalid_jump_target.ir
IR_BADTEMP := $(IR_TEST_DIR)/invalid_bad_temp.ir
IR_DUPBLOCK := $(IR_TEST_DIR)/invalid_dup_block.ir
IR_DUPFIELD := $(IR_TEST_DIR)/invalid_dup_field.ir
IR_DUPFN := $(IR_TEST_DIR)/invalid_dup_fn.ir
IR_OPT_CONST := $(IR_TEST_DIR)/opt_const.ir
EXAMPLES := $(wildcard compiler/bootstrap/stage0/examples/*/main.dast)
STAGE0_RUN_PASS := $(wildcard compiler/bootstrap/stage0/tests/run-pass/*.dast)
STAGE0_COMPILE_FAIL := $(wildcard compiler/bootstrap/stage0/tests/compile-fail/*.dast)
STAGE2_RUN_PASS := $(wildcard compiler/stage2/tests/run-pass/*/main.dast)
STAGE2_COMPILE_FAIL := $(wildcard compiler/stage2/tests/compile-fail/*/main.dast)
STAGE2_TEST_CMD := $(wildcard compiler/stage2/tests/test-cmd/*)
STAGE2_BUILD := $(wildcard compiler/stage2/tests/build/*)
STAGE2_BUILD_FAIL := $(wildcard compiler/stage2/tests/build-fail/*)
STAGE2_WORKSPACE := $(wildcard compiler/stage2/tests/workspace/*)
STAGE2_EXAMPLES := $(wildcard compiler/stage2/tests/examples/*)

.PHONY: build-stage0 build-stage1-ir test-stage0 test-stage1 test-stage2 test-stage1-ir test-stage1-full test-ir test-ir-verify test-ir-opt test clean vscode-ext vscode-ext-install vscode-ext-clean

build-stage0:
	@cd $(STAGE0_DIR) && go build -o dast-stage0 ./cmd/dast

test-stage0: build-stage0
	@for f in $(EXAMPLES); do \
		echo "[stage0] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
	done
	@for f in $(STAGE0_RUN_PASS); do \
		echo "[stage0-run] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
	done
	@for f in $(STAGE0_COMPILE_FAIL); do \
		echo "[stage0-fail] $$f"; \
		out=$$(./$(STAGE0_BIN) run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		if [ -z "$$out" ]; then echo "expected diagnostics"; exit 1; fi; \
	done

test-stage1: build-stage0
	@for f in $(EXAMPLES); do \
		echo "[stage1] $$f"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done

test-stage1-full: test-stage1

test-stage1-self: build-stage0
	@echo "[stage1-self] stage1 self-host IR check"; \
	tmp0="/tmp/dast-stage1-self-$$.ir"; \
	tmp1="/tmp/dast-stage1-self-$$.ir1"; \
	tmp2="/tmp/dast-stage1-self-$$.ir2"; \
	./$(STAGE0_BIN) ir $(STAGE1_FILES) > $$tmp0 || exit 1; \
	./$(STAGE0_BIN) ir-verify $$tmp0 || exit 1; \
	./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir $(STAGE1_FILES) > $$tmp1 || exit 1; \
	./$(STAGE0_BIN) ir-verify $$tmp1 || exit 1; \
	./$(STAGE0_BIN) ir-run $$tmp0 -- ir $(STAGE1_FILES) > $$tmp2 || exit 1; \
	./$(STAGE0_BIN) ir-verify $$tmp2 || exit 1; \
	diff -q $$tmp1 $$tmp2 >/tmp/dast-stage1-self.out 2>&1 || { \
		echo "stage1 self-host IR mismatch"; \
		cat /tmp/dast-stage1-self.out; \
		exit 1; \
	}; \
	rm -f $$tmp0 $$tmp1 $$tmp2

build-stage1-ir: build-stage0
	@./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir $(STAGE2_FILES) > $(STAGE1_IR)

test-stage2: build-stage0
	@for f in $(STAGE2_RUN_PASS); do \
		echo "[stage2-run] $$f"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done
	@for f in $(STAGE2_COMPILE_FAIL); do \
		echo "[stage2-fail] $$f"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done
	@for d in $(STAGE2_TEST_CMD); do \
		echo "[stage2-test] $$d"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done
	@for d in $(STAGE2_BUILD); do \
		echo "[stage2-build] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
		for f in $$d/target/*.ir; do \
			out2=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $$f 2>&1); \
			status2=$$?; \
			echo "$$out2"; \
			if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
			if echo "$$out2" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		done; \
	done
	@for d in $(STAGE2_BUILD_FAIL); do \
		echo "[stage2-build-fail] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done
	@for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-run] $$d"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- run --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done
	@for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-test] $$d"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- test --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done
	@for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-build] $$d"; \
		rm -rf $$d/app/target; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/app/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done
	@for d in $(STAGE2_EXAMPLES); do \
		echo "[stage2-example-run] $$d"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- run --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done
	@for d in $(STAGE2_EXAMPLES); do \
		echo "[stage2-example-build] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done

test-stage1-ir: build-stage0 build-stage1-ir
	@for f in $(EXAMPLES); do \
		echo "[stage1-ir] $$f"; \
		out=$$(./$(STAGE0_BIN) ir-run $(STAGE1_IR) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done

test-ir: build-stage0
	@for f in $(EXAMPLES); do \
		echo "[ir0] $$f"; \
		tmp="/tmp/dast-ir-v0-$$.ir"; \
		./$(STAGE0_BIN) ir $$f > $$tmp || exit 1; \
		./$(STAGE0_BIN) ir-run $$tmp || exit 1; \
	done
	@for f in $(EXAMPLES); do \
		echo "[ir1] $$f"; \
		tmp="/tmp/dast-ir-v0-$$.ir"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		echo "$$out" > $$tmp; \
		out2=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-run $$tmp 2>&1); \
		status2=$$?; \
		echo "$$out2"; \
		if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
		if echo "$$out2" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done

test-ir-verify: build-stage0
	@./$(STAGE0_BIN) ir-verify $(IR_VALID)
	@if ./$(STAGE0_BIN) ir-verify $(IR_INVALID) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_UNDEF) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_UNINIT) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_TERM) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_JUMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_BADTEMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_DUPBLOCK) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_DUPFIELD) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@if ./$(STAGE0_BIN) ir-verify $(IR_DUPFN) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi
	@out=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_VALID) 2>&1); \
	status=$$?; echo "$$out"; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_INVALID) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_UNDEF) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_UNINIT) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_TERM) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_JUMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_BADTEMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_DUPBLOCK) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_DUPFIELD) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_DUPFN) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_VALID) 2>&1); \
	status=$$?; echo "$$out"; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_INVALID) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_UNDEF) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_UNINIT) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_TERM) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_JUMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_BADTEMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_DUPBLOCK) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_DUPFIELD) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $(IR_DUPFN) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^stage[0-9]:' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }

test-ir-opt: build-stage0
	@out=$$(./$(STAGE0_BIN) ir-opt $(IR_OPT)); \
	echo "$$out" | grep -q 'jump then' || { echo "expected jump then"; echo "$$out"; exit 1; }; \
	if echo "$$out" | grep -q 'block else'; then echo "expected else block removed"; echo "$$out"; exit 1; fi
	@out=$$(./$(STAGE0_BIN) ir-opt $(IR_OPT_CONST)); \
	echo "$$out" | grep -q 't2 = const 3' || { echo "expected const fold"; echo "$$out"; exit 1; }; \
	echo "$$out" | grep -q 't4 = const false' || { echo "expected const fold"; echo "$$out"; exit 1; }
	@out=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-opt $(IR_OPT) 2>&1); \
	echo "$$out" | grep -q 'jump then' || { echo "expected jump then"; echo "$$out"; exit 1; }; \
	if echo "$$out" | grep -q 'block else'; then echo "expected else block removed"; echo "$$out"; exit 1; fi
	@out=$$(./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-opt $(IR_OPT_CONST) 2>&1); \
	echo "$$out" | grep -q 't2 = const 3' || { echo "expected const fold"; echo "$$out"; exit 1; }; \
	echo "$$out" | grep -q 't4 = const false' || { echo "expected const fold"; echo "$$out"; exit 1; }
	@out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-opt $(IR_OPT) 2>&1); \
	echo "$$out" | grep -q 'jump then' || { echo "expected jump then"; echo "$$out"; exit 1; }; \
	if echo "$$out" | grep -q 'block else'; then echo "expected else block removed"; echo "$$out"; exit 1; fi
	@out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-opt $(IR_OPT_CONST) 2>&1); \
	echo "$$out" | grep -q 't2 = const 3' || { echo "expected const fold"; echo "$$out"; exit 1; }; \
	echo "$$out" | grep -q 't4 = const false' || { echo "expected const fold"; echo "$$out"; exit 1; }

test:
	@echo "[test] start"
	@$(MAKE) test-stage0
	@$(MAKE) test-stage1
	@$(MAKE) test-stage1-self
	@$(MAKE) test-stage2
	@$(MAKE) test-ir
	@$(MAKE) test-ir-verify
	@$(MAKE) test-ir-opt
	@echo "[test] done"

clean:
	@rm -f $(STAGE0_BIN)

# VSCode Extension targets
vscode-ext:
	@echo "Building VSCode extension..."
	cd compiler/stage3/vscode-ext && npm install
	cd compiler/stage3/vscode-ext && npm run compile
	cd compiler/stage3/vscode-ext && yes | npx -y @vscode/vsce package --allow-missing-repository --no-dependencies

vscode-ext-install: vscode-ext
	@echo "Installing VSCode extension..."
	@VSIX=$$(ls -t compiler/stage3/vscode-ext/*.vsix 2>/dev/null | head -1); \
	if [ -n "$$VSIX" ]; then \
		code --install-extension "$$VSIX"; \
		echo "Extension installed: $$VSIX"; \
	else \
		echo "Error: No .vsix file found"; \
		exit 1; \
	fi

vscode-ext-clean:
	@echo "Cleaning VSCode extension build artifacts..."
	rm -rf compiler/stage3/vscode-ext/out
	rm -rf compiler/stage3/vscode-ext/node_modules
	rm -f compiler/stage3/vscode-ext/*.vsix
