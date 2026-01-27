SHELL := /bin/sh

STAGE0_DIR := compiler/bootstrap/stage0
STAGE0_BIN := $(STAGE0_DIR)/dast-stage0
STAGE2V2_DRIVER := compiler/stage2v2-min/driver/main.dast
STAGE2V2_FILES := $(shell find compiler/stage2v2-min -name '*.dast' -not -path 'compiler/stage2v2-min/stdlib/prelude/*' | sort)
STAGE2V2_OUT ?= compiler/stage2v2-min/target/dast-stage2v2-min
STAGE2V2_ALLOC_MAX_MB ?= 128
STAGE2V2_RUN_ENV := $(if $(STAGE2V2_ALLOC_MAX_MB),DAST_ALLOC_TOTAL_MAX_MB=$(STAGE2V2_ALLOC_MAX_MB),)
STAGE2_FILES := $(shell find compiler/stage2 -name '*.dast' -not -path 'compiler/stage2/tests/*' -not -path 'compiler/stage2/stdlib/*' -not -path 'compiler/stage2/backend/codegen-c/*' -not -path 'compiler/stage2/backend/interp/*' | sort) compiler/stage2/backend/interp/interp.dast compiler/stage2/backend/interp/quote.dast
STAGE2_COMPILER_DIRS := compiler/stage2 compiler/stage2/driver compiler/stage2/frontend compiler/stage2/middle compiler/stage2/backend compiler/stage2/backend/interp compiler/stage2/backend/qbe compiler/stage2/backend/cg
STAGE2_COMPILER_OUT ?= compiler/stage2/target/dast-stage2
STAGE2_COMPILER_DEBUG ?=
STAGE2_COMPILER_DEBUG_FLAG := $(if $(STAGE2_COMPILER_DEBUG),--debug,)
UNAME_S := $(shell uname -s)
STAGE2_MEM_LIMIT_KB ?= 8388608
STAGE2_GOMEMLIMIT ?= 8GiB
STAGE2_MEM_LIMIT_MB ?= 8192
STAGE2_MEM_LIMIT_CMD := $(if $(filter Linux,$(UNAME_S)),$(if $(STAGE2_MEM_LIMIT_KB),ulimit -v $(STAGE2_MEM_LIMIT_KB);,),)
STAGE2_RUN_ENV := $(if $(STAGE2_GOMEMLIMIT),GOMEMLIMIT=$(STAGE2_GOMEMLIMIT),) $(if $(STAGE2_MEM_LIMIT_MB),DAST_MEM_LIMIT_MB=$(STAGE2_MEM_LIMIT_MB),)
STAGE2_RUNNER := $(STAGE2_RUN_ENV) ./$(STAGE0_BIN)
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
IR_GEN_DIRS := compiler/tests/ir-gen compiler/bootstrap/stage0/tests/ir-gen
IR_QBE_DIRS := compiler/tests/ir-qbe compiler/bootstrap/stage0/tests/ir-qbe
EXAMPLES := $(wildcard compiler/bootstrap/stage0/examples/*/main.dast)
STAGE0_RUN_PASS := $(wildcard compiler/tests/run-pass/*.dast)
STAGE0_COMPILE_FAIL := $(wildcard compiler/tests/compile-fail/*.dast)
# Ordered stage0 phases: simple -> combo -> integration.
STAGE0_SIMPLE_RUN_PASS_BASENAMES := \
	010_char_lit \
	020_const_expr \
	030_if_while_let \
	040_loop_break_continue \
	050_borrow_basic \
	060_array_slice_autoborrow \
	070_array_borrow_reuse \
	080_string_autoborrow_str \
	090_string_borrow_reuse \
	100_drop_basic \
	110_type-alias-basic
STAGE0_SIMPLE_RUN_PASS := $(addprefix compiler/tests/run-pass/,$(addsuffix .dast,$(STAGE0_SIMPLE_RUN_PASS_BASENAMES)))
STAGE0_COMBO_RUN_PASS := $(filter-out $(STAGE0_SIMPLE_RUN_PASS),$(sort $(STAGE0_RUN_PASS)))
SHARED_RUN_PASS := $(STAGE0_SIMPLE_RUN_PASS) $(STAGE0_COMBO_RUN_PASS)
SHARED_RUN_PASS_TOTAL := $(words $(SHARED_RUN_PASS))
STAGE2V2_RUN_PASS_BASENAMES := \
	010_char_lit \
	020_const_expr \
	030_if_while_let \
	040_loop_break_continue \
	050_borrow_basic \
	060_array_slice_autoborrow \
	070_array_borrow_reuse \
	080_string_autoborrow_str \
	090_string_borrow_reuse \
	120_return_i64 \
	130_return_bool \
	140_enum_basic \
	150_drop_overwrite_struct \
	160_drop_return_struct \
	170_drop_control_flow \
	180_drop_loop_return_continue \
	190_drop_array_struct \
	200_borrow_field_index \
	210_if_match_expr \
	220_let_pattern \
	230_match_patterns \
	240_type_alias_basic \
	250_enum_tag_i32
STAGE2V2_SHARED_SIMPLE := $(addprefix compiler/tests/run-pass/,$(addsuffix .dast,$(STAGE2V2_RUN_PASS_BASENAMES)))
STAGE2V2_SHARED_TOTAL := $(words $(STAGE2V2_SHARED_SIMPLE))
STAGE0_MODULE_TEST_DIR := compiler/tests/integration/module-basic
STAGE0_TEST_FAIL_DIR := compiler/tests/integration/test-fail
STAGE0_TEST_FAIL_COMPILE_DIR := compiler/tests/integration/test-fail-compile
STAGE0_DEPS_APP_DIR := compiler/tests/integration/deps/app
STAGE0_WORKSPACE_APP_DIR := compiler/tests/integration/workspace/app
INTEGRATION_TEST_DIRS := $(STAGE0_MODULE_TEST_DIR) $(STAGE0_DEPS_APP_DIR) $(STAGE0_WORKSPACE_APP_DIR)
INTEGRATION_FAIL_DIRS := $(STAGE0_TEST_FAIL_DIR) $(STAGE0_TEST_FAIL_COMPILE_DIR)
SHARED_PKG_RUN_PASS_DIRS := $(wildcard compiler/tests/run-pass-pkg/*)
SHARED_PKG_SIMPLE_BASENAMES := \
	010_char-literal \
	020_array-string-builtins \
	030_implicit-return \
	040_loop-break-continue \
	050_refs-basic \
	060_if-let \
	070_if-let-expr \
	080_type-alias-simple
SHARED_PKG_SIMPLE_DIRS := $(addprefix compiler/tests/run-pass-pkg/,$(SHARED_PKG_SIMPLE_BASENAMES))
SHARED_PKG_COMBO_DIRS := $(filter-out $(SHARED_PKG_SIMPLE_DIRS),$(sort $(SHARED_PKG_RUN_PASS_DIRS)))
SHARED_PKG_TOTAL := $(words $(SHARED_PKG_SIMPLE_DIRS) $(SHARED_PKG_COMBO_DIRS))
STAGE2_RUN_TEST_DIRS := $(wildcard compiler/stage2/tests/run-pass/*)
STAGE2_COMPILE_FAIL := $(wildcard compiler/stage2/tests/compile-fail/*/main.dast)
STAGE2_TEST_CMD := $(wildcard compiler/stage2/tests/test-cmd/*)
STAGE2_BUILD := $(wildcard compiler/stage2/tests/build/*)
STAGE2_BUILD_FAIL := $(wildcard compiler/stage2/tests/build-fail/*)
STAGE2_WORKSPACE := $(wildcard compiler/stage2/tests/workspace/*)
STAGE2_EXAMPLES := $(wildcard compiler/stage2/tests/examples/*)
# Ordered IR generation phases for easier debugging.
IR_GEN_ALL := $(sort $(foreach d,$(IR_GEN_DIRS),$(wildcard $(d)/*.dast)))
IR_GEN_SIMPLE_BASENAMES := control vars
IR_GEN_SIMPLE_CANDIDATES := $(foreach d,$(IR_GEN_DIRS),$(addprefix $(d)/,$(addsuffix .dast,$(IR_GEN_SIMPLE_BASENAMES))))
IR_GEN_SIMPLE := $(sort $(wildcard $(IR_GEN_SIMPLE_CANDIDATES)))
IR_GEN_COMBO := $(filter-out $(IR_GEN_SIMPLE),$(IR_GEN_ALL))
IR_QBE_ALL := $(sort $(foreach d,$(IR_QBE_DIRS),$(wildcard $(d)/*.ir)))
IR_QBE_SIMPLE_BASENAMES := control vars
IR_QBE_SIMPLE_CANDIDATES := $(foreach d,$(IR_QBE_DIRS),$(addprefix $(d)/,$(addsuffix .ir,$(IR_QBE_SIMPLE_BASENAMES))))
IR_QBE_SIMPLE := $(sort $(wildcard $(IR_QBE_SIMPLE_CANDIDATES)))
IR_QBE_COMBO := $(filter-out $(IR_QBE_SIMPLE),$(IR_QBE_ALL))
# Ordered stage2 phases: simple -> combo -> integration.
STAGE2_SIMPLE_RUN_TEST_DIR_BASENAMES := \
	010_char-literal \
	020_array-string-builtins \
	030_implicit-return \
	040_loop-break-continue \
	050_refs-basic \
	060_if-let \
	070_if-let-expr \
	080_type-alias-simple
STAGE2_SIMPLE_RUN_TEST_DIRS := $(addprefix compiler/stage2/tests/run-pass/,$(STAGE2_SIMPLE_RUN_TEST_DIR_BASENAMES))
STAGE2_COMBO_RUN_TEST_DIRS := $(filter-out $(STAGE2_SIMPLE_RUN_TEST_DIRS),$(sort $(STAGE2_RUN_TEST_DIRS)))
STAGE2_SIMPLE_BUILD_BASENAMES := basic
STAGE2_SIMPLE_BUILD := $(addprefix compiler/stage2/tests/build/,$(STAGE2_SIMPLE_BUILD_BASENAMES))
STAGE2_COMBO_BUILD := $(filter-out $(STAGE2_SIMPLE_BUILD),$(sort $(STAGE2_BUILD)))
# Shared run-pass tests: ordered by numeric filename prefixes.
CC ?= cc
CFLAGS ?= -std=c11 -O2
NATIVE_PATH ?= compiler/stage2/tests/examples/native-full
NATIVE_BUILD_ARGS ?= --example hello
NATIVE_TARGET_DIR ?=

.PHONY: build-stage0 test-stage0 test-stage0-drop test-stage2 test-stage2v2 test-stage2-bootstrap test-stage2-parity test-shared-stage0 test-ir test-ir-verify test-ir-opt test-ir-gen test-ir-gen-simple test-ir-gen-combo test-ir-qbe test-ir-qbe-simple test-ir-qbe-combo test clean stage2-native stage2-compiler vscode-ext vscode-ext-install vscode-ext-clean

build-stage0:
	@cd $(STAGE0_DIR) && go build -o dast-stage0 ./cmd/dast

test-stage0: build-stage0
	@echo "[stage0-phase1] simple syntax compile"; \
	for f in $(STAGE0_SIMPLE_RUN_PASS); do \
		echo "[stage0-simple] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
	done; \
	for f in $(STAGE0_COMPILE_FAIL); do \
		echo "[stage0-fail] $$f"; \
		out=$$(./$(STAGE0_BIN) run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		if [ -z "$$out" ]; then echo "expected diagnostics"; exit 1; fi; \
	done
	@echo "[stage0-phase1b] shared simple package tests"; \
	for d in $(SHARED_PKG_SIMPLE_DIRS); do \
		echo "[stage0-simple-pkg] $$d"; \
		./$(STAGE0_BIN) test $$d || exit 1; \
	done
	@echo "[stage0-phase2] simple syntax generate"; \
	$(MAKE) test-ir-gen-simple; \
	$(MAKE) test-ir-qbe-simple
	@echo "[stage0-phase3] combo syntax compile+generate"; \
	for f in $(STAGE0_COMBO_RUN_PASS); do \
		echo "[stage0-combo] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
	done; \
	for d in $(SHARED_PKG_COMBO_DIRS); do \
		echo "[stage0-combo-pkg] $$d"; \
		./$(STAGE0_BIN) test $$d || exit 1; \
	done; \
	$(MAKE) test-ir-gen-combo; \
	$(MAKE) test-ir-qbe-combo
	@echo "[stage0-phase4] integration"; \
	for f in $(EXAMPLES); do \
		echo "[stage0-example] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
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

test-shared-stage0: build-stage0
	@i=0; total=$(SHARED_RUN_PASS_TOTAL); \
	for f in $(SHARED_RUN_PASS); do \
		i=$$((i+1)); \
		echo "[shared-stage0 $$i/$$total] $$f"; \
		./$(STAGE0_BIN) run $$f || exit 1; \
	done

test-stage0-drop: build-stage0
	@DAST_DROP_DEBUG=1 $(MAKE) test-stage0

test-ir-gen-simple: build-stage0
	@for f in $(IR_GEN_SIMPLE); do \
		echo "[ir-gen-simple] $$f"; \
		tmp="/tmp/dast-ir-gen-$$.ir"; \
		exp="$${f%.dast}.ir"; \
		if [ ! -f "$$exp" ]; then echo "missing $$exp"; exit 1; fi; \
		./$(STAGE0_BIN) ir $$f > $$tmp || exit 1; \
		diff -u "$$exp" "$$tmp" || exit 1; \
		rm -f "$$tmp"; \
	done

test-ir-gen-combo: build-stage0
	@for f in $(IR_GEN_COMBO); do \
		echo "[ir-gen-combo] $$f"; \
		tmp="/tmp/dast-ir-gen-$$.ir"; \
		exp="$${f%.dast}.ir"; \
		if [ ! -f "$$exp" ]; then echo "missing $$exp"; exit 1; fi; \
		./$(STAGE0_BIN) ir $$f > $$tmp || exit 1; \
		diff -u "$$exp" "$$tmp" || exit 1; \
		rm -f "$$tmp"; \
	done

test-ir-gen: build-stage0
	@$(MAKE) test-ir-gen-simple
	@$(MAKE) test-ir-gen-combo

test-ir-qbe-simple: build-stage0
	@for f in $(IR_QBE_SIMPLE); do \
		echo "[ir-qbe-simple] $$f"; \
		tmp="/tmp/dast-ir-qbe-$$.qbe"; \
		exp="$${f%.ir}.qbe"; \
		if [ ! -f "$$exp" ]; then echo "missing $$exp"; exit 1; fi; \
		./$(STAGE0_BIN) ir-qbe $$f > $$tmp || exit 1; \
		diff -u "$$exp" "$$tmp" || exit 1; \
		rm -f "$$tmp"; \
	done

test-ir-qbe-combo: build-stage0
	@for f in $(IR_QBE_COMBO); do \
		echo "[ir-qbe-combo] $$f"; \
		tmp="/tmp/dast-ir-qbe-$$.qbe"; \
		exp="$${f%.ir}.qbe"; \
		if [ ! -f "$$exp" ]; then echo "missing $$exp"; exit 1; fi; \
		./$(STAGE0_BIN) ir-qbe $$f > $$tmp || exit 1; \
		diff -u "$$exp" "$$tmp" || exit 1; \
		rm -f "$$tmp"; \
	done

test-ir-qbe: build-stage0
	@$(MAKE) test-ir-qbe-simple
	@$(MAKE) test-ir-qbe-combo

test-stage2: build-stage0
	@set -e; \
	stage2_bin="$(STAGE2_COMPILER_OUT)"; \
	mkdir -p "$$(dirname "$$stage2_bin")"; \
	echo "[stage2-build] $$stage2_bin"; \
	$(STAGE2_MEM_LIMIT_CMD) ./$(STAGE0_BIN) build -o "$$stage2_bin" $(STAGE2_FILES); \
	$(STAGE2_MEM_LIMIT_CMD) i=0; total=$(SHARED_RUN_PASS_TOTAL); \
	for f in $(SHARED_RUN_PASS); do \
		i=$$((i+1)); \
		echo "[shared-stage2 $$i/$$total] $$f"; \
		out=$$($$stage2_bin run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	echo "[stage2-phase1] simple syntax compile"; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(SHARED_PKG_SIMPLE_DIRS); do \
		echo "[shared-simple-pkg] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_SIMPLE_RUN_TEST_DIRS); do \
		echo "[stage2-simple] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	echo "[stage2-phase2] simple syntax generate"; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_SIMPLE_BUILD); do \
		echo "[stage2-simple-build] $$d"; \
		rm -rf $$d/target; \
		out=$$($$stage2_bin build --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
		for f in $$d/target/*.ir; do \
			out2=$$($$stage2_bin ir-verify $$f 2>&1); \
			status2=$$?; \
			echo "$$out2"; \
			if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
			if echo "$$out2" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		done; \
	done; \
	echo "[stage2-phase3] combo syntax compile"; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(SHARED_PKG_COMBO_DIRS); do \
		echo "[shared-combo-pkg] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_COMBO_RUN_TEST_DIRS); do \
		echo "[stage2-combo] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	echo "[stage2-phase3] combo syntax generate"; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_COMBO_BUILD); do \
		echo "[stage2-combo-build] $$d"; \
		rm -rf $$d/target; \
		out=$$($$stage2_bin build --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
		for f in $$d/target/*.ir; do \
			out2=$$($$stage2_bin ir-verify $$f 2>&1); \
			status2=$$?; \
			echo "$$out2"; \
			if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
			if echo "$$out2" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		done; \
	done; \
	echo "[stage2-phase4] shared-integration"; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(INTEGRATION_TEST_DIRS); do \
		echo "[shared-integration-run] $$d"; \
		out=$$($$stage2_bin run $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[shared-integration-test] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	echo "[shared-integration-build] $(STAGE0_MODULE_TEST_DIR)"; \
	$(STAGE2_MEM_LIMIT_CMD) $$stage2_bin build --emit-ir $(STAGE0_MODULE_TEST_DIR) -o /tmp/dast-stage2-module.ir || exit 1; \
	rm -f /tmp/dast-stage2-module.ir; \
	echo "[shared-integration-build] $(STAGE0_DEPS_APP_DIR)"; \
	$(STAGE2_MEM_LIMIT_CMD) $$stage2_bin build --emit-ir $(STAGE0_DEPS_APP_DIR) -o /tmp/dast-stage2-deps.ir || exit 1; \
	rm -f /tmp/dast-stage2-deps.ir; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(INTEGRATION_FAIL_DIRS); do \
		echo "[shared-integration-fail] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	echo "[stage2-phase4] integration"; \
	$(STAGE2_MEM_LIMIT_CMD) for f in $(STAGE2_COMPILE_FAIL); do \
		echo "[stage2-fail] $$f"; \
		out=$$($$stage2_bin run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_TEST_CMD); do \
		echo "[stage2-test] $$d"; \
		out=$$($$stage2_bin test $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_BUILD_FAIL); do \
		echo "[stage2-build-fail] $$d"; \
		rm -rf $$d/target; \
		out=$$($$stage2_bin build --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-run] $$d"; \
		out=$$($$stage2_bin run --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-test] $$d"; \
		out=$$($$stage2_bin test --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-build] $$d"; \
		rm -rf $$d/app/target; \
		out=$$($$stage2_bin build --emit-ir --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/app/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_EXAMPLES); do \
		echo "[stage2-example-run] $$d"; \
		out=$$($$stage2_bin run --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	$(STAGE2_MEM_LIMIT_CMD) for d in $(STAGE2_EXAMPLES); do \
		echo "[stage2-example-build] $$d"; \
		rm -rf $$d/target; \
		out=$$($$stage2_bin build --emit-ir --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done

test-stage2v2: build-stage0
	@set -e; \
	stage2v2_bin="$(STAGE2V2_OUT)"; \
	mkdir -p "$$(dirname "$$stage2v2_bin")"; \
	echo "[stage2v2-build] $$stage2v2_bin"; \
	$(STAGE2V2_RUN_ENV) ./$(STAGE0_BIN) build -o "$$stage2v2_bin" $(STAGE2V2_FILES); \
	echo "[stage2v2-smoke] $$stage2v2_bin"; \
	out=$$($(STAGE2V2_RUN_ENV) $$stage2v2_bin 2>&1); \
	status=$$?; \
	echo "$$out"; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	echo "[stage2v2-drop] $$stage2v2_bin --drop-self-test"; \
	out=$$($(STAGE2V2_RUN_ENV) $$stage2v2_bin --drop-self-test 2>&1); \
	status=$$?; \
	echo "$$out"; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	echo "[stage2v2-drop-pass] $$stage2v2_bin --drop-pass-self-test"; \
	out=$$($(STAGE2V2_RUN_ENV) $$stage2v2_bin --drop-pass-self-test 2>&1); \
	status=$$?; \
	echo "$$out"; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	echo "[stage2v2-phase1] shared simple run-pass"; \
	i=0; total=$(words $(STAGE2V2_SHARED_SIMPLE)); \
	for f in $(STAGE2V2_SHARED_SIMPLE); do \
		i=$$((i+1)); \
		echo "[stage2v2-shared $$i/$$total] $$f"; \
		out=$$($(STAGE2V2_RUN_ENV) $$stage2v2_bin run $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
	done

test-stage2-bootstrap: build-stage0
	@tmp=$$(mktemp); $(STAGE2_MEM_LIMIT_CMD) \
	$(STAGE2_RUNNER) ir $(STAGE2_FILES) > $$tmp || exit 1; \
	for d in $(STAGE2_RUN_TEST_DIRS); do \
		echo "[stage2-test] $$d"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- test --bootstrap $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	for f in $(STAGE2_COMPILE_FAIL); do \
		echo "[stage2-fail] $$f"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- run --bootstrap $$f 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	for d in $(STAGE2_TEST_CMD); do \
		echo "[stage2-test] $$d"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- test --bootstrap $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
	done; \
	for d in $(STAGE2_BUILD); do \
		echo "[stage2-build] $$d"; \
		rm -rf $$d/target; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- build --bootstrap --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
		for f in $$d/target/*.ir; do \
			out2=$$($(STAGE2_RUNNER) ir-run $$tmp -- ir-verify $$f 2>&1); \
			status2=$$?; \
			echo "$$out2"; \
			if [ $$status2 -ne 0 ]; then exit $$status2; fi; \
			if echo "$$out2" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		done; \
	done; \
	for d in $(STAGE2_BUILD_FAIL); do \
		echo "[stage2-build-fail] $$d"; \
		rm -rf $$d/target; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- build --bootstrap --emit-ir $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -eq 0 ]; then echo "expected failure"; exit 1; fi; \
		echo "$$out" | grep -q '^stage[0-9]:' || exit 1; \
	done; \
	for d in $(STAGE2_WORKSPACE); do \
		echo "[stage2-workspace-run] $$d"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- run --bootstrap --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[stage2-workspace-test] $$d"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- test --bootstrap --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[stage2-workspace-build] $$d"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- build --bootstrap --emit-ir --package app $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if ! ls $$d/target/*.ir >/dev/null 2>&1; then echo "missing build output"; exit 1; fi; \
	done; \
	for d in $(STAGE2_EXAMPLES); do \
		echo "[stage2-example-run] $$d"; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- run --bootstrap --example hello $$d 2>&1); \
		status=$$?; \
		echo "$$out"; \
		if [ $$status -ne 0 ]; then exit $$status; fi; \
		if echo "$$out" | grep -q '^stage[0-9]:'; then exit 1; fi; \
		echo "[stage2-example-build] $$d"; \
		rm -rf $$d/target; \
		out=$$($(STAGE2_RUNNER) ir-run $$tmp -- build --bootstrap --emit-ir --example hello $$d 2>&1); \
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
	./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $(NATIVE_BUILD_ARGS) "$$path"; \
	out=$$(find "$$target" -maxdepth 1 -type f -perm -111 2>/dev/null | head -1); \
	if [ -z "$$out" ]; then echo "missing native output in $$target"; exit 1; fi; \
	echo "native: $$out"

stage2-compiler: build-stage0
	@set -e; \
	root="compiler/stage2"; \
	target="$$root/target"; \
	stage2_bin="$(STAGE2_COMPILER_OUT)"; \
	use_native=0; \
	if [ -x "$$stage2_bin" ]; then \
		use_native=1; \
		for f in $(STAGE2_FILES) compiler/stage2/backend/codegen-c/c_runtime.c compiler/stage2/backend/codegen-c/c_runtime.h compiler/stage2/backend/qbe/qbe_runtime.c; do \
			if [ "$$f" -nt "$$stage2_bin" ]; then use_native=0; break; fi; \
		done; \
	fi; \
	mkdir -p "$$target"; \
	rm -rf "$$target"/*; \
	echo "[stage2-compiler] build native"; \
	if [ $$use_native -eq 1 ]; then \
		if ! "$$stage2_bin" build $(STAGE2_COMPILER_DEBUG_FLAG) --bootstrap $(STAGE2_COMPILER_DIRS); then \
			echo "[stage2-compiler] native build failed, falling back to stage0"; \
			./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $(STAGE2_COMPILER_DEBUG_FLAG) --bootstrap $(STAGE2_COMPILER_DIRS); \
		fi; \
	else \
		./$(STAGE0_BIN) run $(STAGE2_FILES) -- build $(STAGE2_COMPILER_DEBUG_FLAG) --bootstrap $(STAGE2_COMPILER_DIRS); \
	fi; \
	out_src=$$(find "$$target" -maxdepth 1 -type f -perm -111 2>/dev/null | head -1); \
	if [ -z "$$out_src" ]; then echo "missing native output in $$target"; exit 1; fi; \
	out="$(STAGE2_COMPILER_OUT)"; \
	mkdir -p $$(dirname "$$out"); \
	cp "$$out_src" "$$out"; \
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
