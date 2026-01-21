# Stage1 Dast Compiler Makefile
# Usage: make test-run   - Run Stage1 compiler
#        make test-ir    - Compile to IR
#        make test-full  - Full pipeline test

STAGE0 = ../stage0/dast-stage0

# Stage1 source files (must be in dependency order)
STAGE1_FILES = \
    irv0/ir.dast \
    interp/interp.dast \
    compiler/token.dast \
    compiler/lexer.dast \
    compiler/ast.dast \
    compiler/parser.dast \
    compiler/typecheck.dast \
    compiler/compile.dast \
    compiler/main.dast

.PHONY: all test-ir test-run test-hello clean

all: test-hello

# Test: compile simple hello world with Stage1
test-hello:
	@echo "=== Testing Stage1 Compiler ==="
	@echo 'fn main() { println("Hello from Stage1!") }' > /tmp/hello.dast
	$(STAGE0) run $(STAGE1_FILES) -- run /tmp/hello.dast
	@rm -f /tmp/hello.dast
	@echo "=== Test Passed ==="

# Generate IR without running
test-ir:
	@echo 'fn main() { println("Test") }' > /tmp/hello.dast
	$(STAGE0) run $(STAGE1_FILES) -- ir /tmp/hello.dast
	@rm -f /tmp/hello.dast

# Run Stage1 in interactive mode
test-run:
	$(STAGE0) run $(STAGE1_FILES) -- $(ARGS)

clean:
	rm -f /tmp/hello.dast /tmp/test.ir
