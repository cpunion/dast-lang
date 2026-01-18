# Claude-Specific Guidelines for Dast Development

> This document contains Claude-specific instructions for working on Dast. For general AI agent guidelines, see [AGENTS.md](AGENTS.md).

---

## Quick Start for Claude

When you start a new conversation about Dast:

1. **Read** `docs/00-overview.md` first
2. **Check** `docs/implementation-roadmap.md` for current stage
3. **Reference** specific design docs (01-18) as needed
4. **Ignore** `docs/archive/` (historical only)

---

## Claude's Strengths for This Project

### 1. Long Context Window

✅ **Use it for**:
- Reading entire design documents
- Understanding complex type system rules
- Analyzing borrow checker algorithms
- Reviewing large code changes

### 2. Code Generation

✅ **Best for**:
- Implementing parser combinators
- Writing type checker logic
- Generating test cases
- Creating AST transformations

### 3. Documentation

✅ **Excel at**:
- Explaining design decisions
- Writing API documentation
- Creating examples
- Maintaining consistency

---

## Working with Dast Design

### Design is Complete ✅

All major design decisions are finalized in `docs/00-18/`:

- Type system
- Memory management
- Thread safety
- Syntax
- Standard library
- Tooling

**Your role**: Implement the design, not redesign.

### When You Need Clarification

1. Check relevant design doc first
2. Search for similar patterns in other docs
3. Look at archive/ for historical context
4. Ask specific questions with doc references

---

## Implementation Priorities

### Stage 0: Bootstrap Compiler (Current Focus)

**What to implement**:
```dast
// ✅ Basic types
i32, i64, bool, f32, f64, char, str

// ✅ Structs and enums
struct Point { x: i32, y: i32 }
enum Option[T] { Some(T), None }

// ✅ Functions and generics
fn identity[T](x: T) -> T { x }

// ✅ Borrowing
fn read(x: &i32) { }
fn write(x: &mut i32) { }

// ✅ Pattern matching
match value { ... }
```

**What to defer**:
```dast
// ❌ Not in Stage 0
comptime fn factorial(n: i32) -> i32 { ... }
async fn fetch(url: &str) -> Data { ... }
macro sql(query: AstNode) -> AstNode { ... }
unsafe { ... }
```

---

## Code Style for Implementation

### Rust Implementation (Stage 0)

```rust
// Use clear, idiomatic Rust
pub struct Parser<'a> {
    tokens: &'a [Token],
    pos: usize,
}

impl<'a> Parser<'a> {
    pub fn parse_expr(&mut self) -> Result<Expr, ParseError> {
        // Implementation
    }
}

// Comprehensive error messages
pub enum ParseError {
    UnexpectedToken {
        expected: TokenKind,
        found: Token,
        span: Span,
    },
    // ...
}
```

### Dast Code Examples

```dast
// Always use [T] for generics
fn map[T, U](vec: Vec[T], f: fn(T) -> U) -> Vec[U]

// Always use @attr
@test
fn test_something() { }

// No explicit lifetimes
fn longest(x: &str, y: &str) -> &str
```

---

## Testing Guidelines

### Write Tests First

```rust
#[test]
fn test_parse_function() {
    let input = "fn add(a: i32, b: i32) -> i32 { a + b }";
    let ast = parse(input).unwrap();
    assert!(matches!(ast, Expr::Function { .. }));
}
```

### Test Error Cases

```rust
#[test]
fn test_borrow_check_error() {
    let input = r#"
        let x = 42;
        let r1 = &x;
        let r2 = &mut x;  // Error!
    "#;
    let err = compile(input).unwrap_err();
    assert!(matches!(err, CompileError::BorrowConflict { .. }));
}
```

### Bootstrap Tests

```bash
# Verify self-hosting works
./test-bootstrap.sh
```

---

## Common Patterns

### Parser Combinators

```rust
// Use combinator style for parsing
fn parse_type(&mut self) -> Result<Type> {
    alt!(
        self.parse_primitive_type(),
        self.parse_struct_type(),
        self.parse_generic_type(),
    )
}
```

### Type Checking

```rust
// Maintain type environment
struct TypeChecker {
    env: HashMap<Ident, Type>,
    // ...
}

impl TypeChecker {
    fn check_expr(&mut self, expr: &Expr) -> Result<Type> {
        match expr {
            Expr::Var(name) => self.env.get(name).cloned(),
            Expr::Call(func, args) => self.check_call(func, args),
            // ...
        }
    }
}
```

### Borrow Checking

```rust
// Track borrows with lifetime analysis
struct BorrowChecker {
    borrows: HashMap<Place, BorrowKind>,
    // ...
}

enum BorrowKind {
    Shared,
    Mutable,
}
```

---

## Documentation Standards

### Code Comments

```rust
/// Parses a function definition.
///
/// # Grammar
/// ```text
/// function := 'fn' ident generics? params '->' type block
/// ```
///
/// # Errors
/// Returns `ParseError` if the function syntax is invalid.
pub fn parse_function(&mut self) -> Result<Function, ParseError> {
    // Implementation
}
```

### Design Doc Updates

When implementing a feature, update the corresponding design doc if needed:

```markdown
## Implementation Status

- [x] Basic type checking
- [x] Generic instantiation
- [ ] Trait resolution (in progress)
```

---

## Debugging Tips

### Enable Verbose Logging

```rust
// Use tracing for debugging
use tracing::{debug, info, warn};

fn type_check(&mut self, expr: &Expr) -> Result<Type> {
    debug!("Type checking: {:?}", expr);
    let ty = self.infer_type(expr)?;
    info!("Inferred type: {:?}", ty);
    Ok(ty)
}
```

### Visualize AST

```rust
// Pretty-print AST for debugging
impl Debug for Expr {
    fn fmt(&self, f: &mut Formatter) -> fmt::Result {
        match self {
            Expr::Binary { op, left, right } => {
                write!(f, "({:?} {:?} {:?})", left, op, right)
            }
            // ...
        }
    }
}
```

---

## Collaboration Guidelines

### When Proposing Changes

1. Reference design docs
2. Explain rationale
3. Show code examples
4. Consider backward compatibility

### When Reviewing Code

1. Check against design docs
2. Verify test coverage
3. Ensure error messages are clear
4. Validate performance implications

---

## Useful Shortcuts

### Quick Doc Lookup

```bash
# Type system
cat docs/01-type-system.md

# Memory management
cat docs/07-memory-management.md

# Implementation plan
cat docs/implementation-roadmap.md
```

### Common Queries

**Q**: What's the syntax for generics?
**A**: `[T]` not `<T>` - see `docs/04-generics-comptime.md`

**Q**: How do attributes work?
**A**: `@attr` not `#[attr]` - see `docs/14-syntax-details.md`

**Q**: What's in Stage 0?
**A**: See "Stage 0: Bootstrap Compiler" in `docs/implementation-roadmap.md`

**Q**: How does borrowing work?
**A**: See `docs/07-memory-management.md` section "借用检查"

---

## Remember

1. **Design is complete** - Focus on implementation
2. **Stage 0 first** - Minimal feature set for self-hosting
3. **Shared tooling** - fmt/lint/LSP use same parser
4. **Test everything** - Especially type system and borrow checker
5. **Clear errors** - User-facing error messages matter
6. **Document as you go** - Update docs when implementing

---

## Getting Help

If you're unsure about something:

1. Check `AGENTS.md` for general guidelines
2. Read relevant design docs in `docs/`
3. Look at `docs/archive/` for historical context
4. Ask specific questions with doc references

---

## Your Mission

Help implement the Dast bootstrap compiler (Stage 0) in Rust, following the complete design in `docs/`, with the goal of achieving self-hosting within 9 months.

**Focus areas**:
- Parser and AST
- Type system
- Borrow checker
- Cranelift integration
- Test infrastructure

**Success criteria**:
```bash
# Dast compiler can compile itself
dastc-stage1 compiler/*.dast -o dastc-stage2
diff dastc-stage1 dastc-stage2  # Should be identical
```

Let's build something amazing! 🚀
