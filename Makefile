SHELL := /bin/sh

STAGE0_DIR := compiler/bootstrap/stage0
STAGE0_BIN := $(STAGE0_DIR)/dast-stage0
STAGE1_FILES := compiler/bootstrap/stage1/token.dast compiler/bootstrap/stage1/lexer.dast compiler/bootstrap/stage1/ast.dast compiler/bootstrap/stage1/parser.dast compiler/bootstrap/stage1/typecheck.dast compiler/bootstrap/stage1/ir.dast compiler/bootstrap/stage1/compile.dast compiler/bootstrap/stage1/interp.dast compiler/bootstrap/stage1/main.dast
STAGE2_FILES := $(shell find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' | sort)
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
STAGE2_RUN_PASS := $(wildcard compiler/stage2/tests/run-pass/*/main.dast)
STAGE2_COMPILE_FAIL := $(wildcard compiler/stage2/tests/compile-fail/*/main.dast)
STAGE2_TEST_CMD := $(wildcard compiler/stage2/tests/test-cmd/*)
STAGE2_BUILD := $(wildcard compiler/stage2/tests/build/*)

.PHONY: build-stage0 build-stage1-ir test-stage0 test-stage1 test-stage2 test-stage1-ir test-stage1-full test-ir test-ir-verify test-ir-opt test clean

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
		if echo "$$out" | grep -q '^error'; then exit 1; fi; \
	done

test-stage1-full: test-stage1

build-stage1-ir: build-stage0
	@./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir $(STAGE2_FILES) > $(STAGE1_IR)

test-stage2: build-stage0
	@for f in $(STAGE2_RUN_PASS); do \
		echo "[stage2-run] $$f"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^error'; then exit 1; fi; \
	done
	@for f in $(STAGE2_COMPILE_FAIL); do \
		echo "[stage2-fail] $$f"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^error' || exit 1; \
	done
	@for d in $(STAGE2_TEST_CMD); do \
		echo "[stage2-test] $$d"; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^error'; then exit 1; fi; \
	done
	@for d in $(STAGE2_BUILD); do \
		echo "[stage2-build] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if [ ! -f $$d/target/build_basic.ir ]; then echo "missing build output"; exit 1; fi; \
		out2=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-verify $$d/target/build_basic.ir 2>&1); \
		status2=$$?; \
		echo "$$out2"; \
		if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
		if echo "$$out2" | grep -q '^error'; then exit 1; fi; \
	done

test-stage1-ir: build-stage0 build-stage1-ir
	@for f in $(EXAMPLES); do \
		echo "[stage1-ir] $$f"; \
		out=$$(./$(STAGE0_BIN) ir-run $(STAGE1_IR) -- run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^error'; then exit 1; fi; \
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
		if echo "$$out2" | grep -q '^error'; then exit 1; fi; \
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
	if echo "$$out" | grep -q '^error'; then exit 1; fi
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_INVALID) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_UNDEF) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_UNINIT) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_TERM) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_JUMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_BADTEMP) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_DUPBLOCK) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_DUPFIELD) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }
	@if ./$(STAGE0_BIN) run $(STAGE1_FILES) -- ir-verify $(IR_DUPFN) >/tmp/dast-ir-verify.out 2>&1; then \
		echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; \
	fi; \
	echo "$$(cat /tmp/dast-ir-verify.out)" | grep -q '^error' || { echo "expected ir-verify to fail"; cat /tmp/dast-ir-verify.out; exit 1; }

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

test: test-stage0 test-stage1 test-stage2 test-ir test-ir-verify test-ir-opt

clean:
	@rm -f $(STAGE0_BIN)
