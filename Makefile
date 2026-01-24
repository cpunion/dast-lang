SHELL := /bin/sh

STAGE0_DIR := compiler/bootstrap/stage0
STAGE0_BIN := $(STAGE0_DIR)/dast-stage0
STAGE2_FILES := $(shell find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' -not -path 'compiler/stage2/stdlib/*' -not -path 'compiler/stage2/backend/codegen-c/*' -not -path 'compiler/stage2/backend/interp/*' | sort) compiler/stage2/backend/interp/interp.dast compiler/stage2/backend/codegen-c/codegen-c.dast
STAGE2_COMPILER_DIRS := compiler/stage2 compiler/stage2/driver compiler/stage2/frontend compiler/stage2/middle compiler/stage2/backend compiler/stage2/backend/interp compiler/stage2/backend/codegen-c
STAGE2_COMPILER_OUT ?= compiler/stage2/target/dast-stage2
STAGE2_COMPILER_DEBUG ?=
STAGE2_COMPILER_DEBUG_FLAG := $(if $(STAGE2_COMPILER_DEBUG),--debug,)
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
IR_GEN_DIR := compiler/bootstrap/stage0/tests/ir-gen
IR_QBE_DIR := compiler/bootstrap/stage0/tests/ir-qbe
EXAMPLES := $(wildcard compiler/bootstrap/stage0/examples/*/main.dast)
STAGE0_RUN_PASS := $(wildcard compiler/bootstrap/stage0/tests/run-pass/*.dast)
STAGE0_COMPILE_FAIL := $(wildcard compiler/bootstrap/stage0/tests/compile-fail/*.dast)
STAGE0_MODULE_TEST_DIR := compiler/bootstrap/stage0/tests/module-basic
STAGE0_TEST_FAIL_DIR := compiler/bootstrap/stage0/tests/test-fail
STAGE0_TEST_FAIL_COMPILE_DIR := compiler/bootstrap/stage0/tests/test-fail-compile
STAGE0_DEPS_APP_DIR := compiler/bootstrap/stage0/tests/deps/app
STAGE0_WORKSPACE_APP_DIR := compiler/bootstrap/stage0/tests/workspace/app
STAGE2_RUN_PASS := $(wildcard compiler/stage2/tests/run-pass/*/main.dast)
STAGE2_COMPILE_FAIL := $(wildcard compiler/stage2/tests/compile-fail/*/main.dast)
STAGE2_TEST_CMD := $(wildcard compiler/stage2/tests/test-cmd/*)
STAGE2_BUILD := $(wildcard compiler/stage2/tests/build/*)
STAGE2_BUILD_FAIL := $(wildcard compiler/stage2/tests/build-fail/*)
STAGE2_WORKSPACE := $(wildcard compiler/stage2/tests/workspace/*)
STAGE2_EXAMPLES := $(wildcard compiler/stage2/tests/examples/*)
CC ?= cc
CFLAGS ?= -std=c11 -O2
NATIVE_PATH ?= compiler/stage2/tests/examples/native-full
NATIVE_BUILD_ARGS ?= --example hello
NATIVE_TARGET_DIR ?=

.PHONY: build-stage0 test-stage0 test-stage2 test-stage2-bootstrap test-stage2-parity test-ir test-ir-verify test-ir-opt test-ir-gen test-ir-qbe test clean stage2-native stage2-compiler vscode-ext vscode-ext-install vscode-ext-clean

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
	@echo "[stage0-build] $(STAGE0_MODULE_TEST_DIR)"; \
	./$(STAGE0_BIN) build --emit-ir $(STAGE0_MODULE_TEST_DIR) -o /tmp/dast-stage0-module.ir || exit 1; \
	rm -f /tmp/dast-stage0-module.ir
	@echo "[stage0-run] $(STAGE0_MODULE_TEST_DIR)"; \
	./$(STAGE0_BIN) run $(STAGE0_MODULE_TEST_DIR) || exit 1
	@echo "[stage0-test] $(STAGE0_MODULE_TEST_DIR)"; \
	./$(STAGE0_BIN) test $(STAGE0_MODULE_TEST_DIR) || exit 1
	@echo "[stage0-test-fail] $(STAGE0_TEST_FAIL_DIR)"; \
	out=$$(./$(STAGE0_BIN) test $(STAGE0_TEST_FAIL_DIR) 2>&1); \
	status=$$?; \
	echo "$$out"; \
	if [ $$status -eq 0 ]; then echo "expected test failure"; exit 1; fi; \
	if [ -z "$$out" ]; then echo "expected diagnostics"; exit 1; fi
	@echo "[stage0-test-fail-compile] $(STAGE0_TEST_FAIL_COMPILE_DIR)"; \
	out=$$(./$(STAGE0_BIN) test $(STAGE0_TEST_FAIL_COMPILE_DIR) 2>&1); \
	status=$$?; \
	echo "$$out"; \
	if [ $$status -eq 0 ]; then echo "expected test failure"; exit 1; fi; \
	if [ -z "$$out" ]; then echo "expected diagnostics"; exit 1; fi
	@echo "[stage0-deps-build] $(STAGE0_DEPS_APP_DIR)"; \
	./$(STAGE0_BIN) build --emit-ir $(STAGE0_DEPS_APP_DIR) -o /tmp/dast-stage0-deps.ir || exit 1; \
	rm -f /tmp/dast-stage0-deps.ir
	@echo "[stage0-deps-run] $(STAGE0_DEPS_APP_DIR)"; \
	./$(STAGE0_BIN) run $(STAGE0_DEPS_APP_DIR) || exit 1
	@echo "[stage0-deps-test] $(STAGE0_DEPS_APP_DIR)"; \
	./$(STAGE0_BIN) test $(STAGE0_DEPS_APP_DIR) || exit 1
	@echo "[stage0-workspace-run] $(STAGE0_WORKSPACE_APP_DIR)"; \
	./$(STAGE0_BIN) run $(STAGE0_WORKSPACE_APP_DIR) || exit 1
	@echo "[stage0-workspace-test] $(STAGE0_WORKSPACE_APP_DIR)"; \
	./$(STAGE0_BIN) test $(STAGE0_WORKSPACE_APP_DIR) || exit 1
	@$(MAKE) test-ir-gen
	@$(MAKE) test-ir-qbe

test-ir-gen: build-stage0
	@for f in $(IR_GEN_DIR)/*.dast; do \
		echo "[ir-gen] $$f"; \
		tmp="/tmp/dast-ir-gen-$$.ir"; \
		exp="$${f%.dast}.ir"; \
		if [ ! -f "$$exp" ]; then echo "missing $$exp"; exit 1; fi; \
		./$(STAGE0_BIN) ir $$f > $$tmp || exit 1; \
		diff -u "$$exp" "$$tmp" || exit 1; \
		rm -f "$$tmp"; \
	done

test-ir-qbe: build-stage0
	@for f in $(IR_QBE_DIR)/*.ir; do \
		echo "[ir-qbe] $$f"; \
		tmp="/tmp/dast-ir-qbe-$$.qbe"; \
		exp="$${f%.ir}.qbe"; \
		if [ ! -f "$$exp" ]; then echo "missing $$exp"; exit 1; fi; \
		./$(STAGE0_BIN) ir-qbe $$f > $$tmp || exit 1; \
		diff -u "$$exp" "$$tmp" || exit 1; \
		rm -f "$$tmp"; \
	done

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
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --emit-ir $$d 2>&1); \
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
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --emit-ir $$d 2>&1); \
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
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --emit-ir --package app $$d 2>&1); \
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
		out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --emit-ir --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done

test-stage2-bootstrap: build-stage0
	@tmp=$$(mktemp); \
	./$(STAGE0_BIN) ir $(STAGE2_FILES) > $$tmp || exit 1; \
	for f in $(STAGE2_RUN_PASS); do \
		echo "[stage2-run] $$f"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- run --bootstrap $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	for f in $(STAGE2_COMPILE_FAIL); do \
		echo "[stage2-fail] $$f"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- run --bootstrap $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	for d in $(STAGE2_TEST_CMD); do \
		echo "[stage2-test] $$d"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- test --bootstrap $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	for d in $(STAGE2_BUILD); do \
		echo "[stage2-build] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- build --bootstrap --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
		for f in $$d/target/*.ir; do \
			out2=$$(./$(STAGE0_BIN) ir-run $$tmp -- ir-verify $$f 2>&1); \
			status2=$$?; \
			echo "$$out2"; \
			if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
			if echo "$$out2" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		done; \
	done; \
	for d in $(STAGE2_BUILD_FAIL); do \
		echo "[stage2-build-fail] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- build --bootstrap --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-run] $$d"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- run --bootstrap --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[stage2-workspace-test] $$d"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- test --bootstrap --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[stage2-workspace-build] $$d"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- build --bootstrap --emit-ir --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done; \
	for d in $(STAGE2_EXAMPLES); do \
		echo "[stage2-example-run] $$d"; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- run --bootstrap --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[stage2-example-build] $$d"; \
		rm -rf $$d/target; \
		out=$$(./$(STAGE0_BIN) ir-run $$tmp -- build --bootstrap --emit-ir --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done; \
	rm -f $$tmp

test-stage2-parity: build-stage0
	@./scripts/test-stage2-parity.sh

stage2-native: build-stage0
	@set -e; \
	path="$(NATIVE_PATH)"; \
	if [ -d "$$path" ]; then root="$$path"; else root=$$(dirname "$$path"); fi; \
	if [ -n "$(NATIVE_TARGET_DIR)" ]; then target="$(NATIVE_TARGET_DIR)"; else target="$$root/target"; fi; \
	echo "[stage2-native] build $$path"; \
	rm -rf "$$target"; \
	./$(STAGE0_BIN) run $(STAGE2_FILES) -- build --emit-ir $(NATIVE_BUILD_ARGS) "$$path"; \
	ir=$$(ls "$$target"/*.ir 2>/dev/null | head -1); \
	if [ -z "$$ir" ]; then echo "missing build output in $$target"; exit 1; fi; \
	cfile="$${ir%.ir}.c"; \
	out="$${ir%.ir}"; \
	./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-c "$$ir" > "$$cfile"; \
	$(CC) $(CFLAGS) "$$cfile" compiler/stage2/backend/codegen-c/c_runtime.c -I compiler/stage2/backend/codegen-c -o "$$out"; \
	echo "native: $$out"

stage2-compiler: build-stage0
	@set -e; \
	root="compiler/stage2"; \
	target="$$root/target"; \
	stage2_bin="$(STAGE2_COMPILER_OUT)"; \
	use_native=0; \
	if [ -x "$$stage2_bin" ]; then \
		use_native=1; \
		for f in $(STAGE2_FILES) compiler/stage2/backend/codegen-c/c_runtime.c compiler/stage2/backend/codegen-c/c_runtime.h; do \
			if [ "$$f" -nt "$$stage2_bin" ]; then use_native=0; break; fi; \
		done; \
	fi; \
	mkdir -p "$$target"; \
	rm -f "$$target"/*.ir "$$target"/*.c; \
	echo "[stage2-compiler] emit IR"; \
	if [ $$use_native -eq 1 ]; then \
		if ! "$$stage2_bin" build $(STAGE2_COMPILER_DEBUG_FLAG) --bootstrap --emit-ir $(STAGE2_COMPILER_DIRS); then \
			echo "[stage2-compiler] native build failed, falling back to stage0"; \
			./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $(STAGE2_COMPILER_DEBUG_FLAG) --bootstrap --emit-ir $(STAGE2_COMPILER_DIRS); \
		fi; \
	else \
		./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $(STAGE2_COMPILER_DEBUG_FLAG) --bootstrap --emit-ir $(STAGE2_COMPILER_DIRS); \
	fi; \
	ir=$$(ls "$$target"/*.ir 2>/dev/null | head -1); \
	if [ -z "$$ir" ]; then echo "missing build output in $$target"; exit 1; fi; \
	cfile="$${ir%.ir}.c"; \
	out="$(STAGE2_COMPILER_OUT)"; \
	mkdir -p $$(dirname "$$out"); \
	echo "[stage2-compiler] ir -> c: $$cfile"; \
	if [ $$use_native -eq 1 ]; then \
		echo "[stage2-compiler] ir-c using native stage2"; \
		"$$stage2_bin" ir-c "$$ir" > "$$cfile"; \
	else \
		./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-c "$$ir" > "$$cfile"; \
	fi; \
	echo "[stage2-compiler] clang -> $$out"; \
	$(CC) $(CFLAGS) "$$cfile" compiler/stage2/backend/codegen-c/c_runtime.c -I compiler/stage2/backend/codegen-c -o "$$out"; \
	echo "native: $$out"

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
	echo "$$out" | grep -q 't0 = + 1, 2' || { echo "expected inline const binop"; echo "$$out"; exit 1; }; \
	echo "$$out" | grep -q 't1 = == true, false' || { echo "expected inline const cmp"; echo "$$out"; exit 1; }
	@out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-opt $(IR_OPT) 2>&1); \
	echo "$$out" | grep -q 'jump then' || { echo "expected jump then"; echo "$$out"; exit 1; }; \
	if echo "$$out" | grep -q 'block else'; then echo "expected else block removed"; echo "$$out"; exit 1; fi
	@out=$$(./$(STAGE0_BIN) run $(STAGE2_FILES) -- ir-opt $(IR_OPT_CONST) 2>&1); \
	echo "$$out" | grep -q 't0 = + 1, 2' || { echo "expected inline const binop"; echo "$$out"; exit 1; }; \
	echo "$$out" | grep -q 't1 = == true, false' || { echo "expected inline const cmp"; echo "$$out"; exit 1; }

test:
	@echo "[test] start"
	@$(MAKE) verify-stage0
	@$(MAKE) test-stage2
	@$(MAKE) test-ir
	@$(MAKE) test-ir-verify
	@$(MAKE) test-ir-opt
	@echo "[test] done"

verify-stage0:
	@./scripts/verify-stage0.sh

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
