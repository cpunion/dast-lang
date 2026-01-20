# 测试框架设计

## 单元测试

### 基础语法（Go 风格）

```
src/
├── math.dast       # 正式代码
└── math_test.dast  # 测试代码
```

```dast
// math_test.dast
// 同包文件，无需 import

// test_ 前缀自动识别为测试函数
fn test_addition() {
    assert_eq!(2 + 2, 4)
}

fn test_string_concat() {
    let s = "hello" + " world"
    assert_eq!(s, "hello world")
}
```

**规则**:
- `*_test.dast` 文件中的 `test_*` 函数自动识别为测试
- 同包文件无需 `import`
- 目前 stage2 仅支持 `test_*` 发现规则（暂不要求 `@test`）

### 断言宏

```dast
// 基础断言
assert!(condition)
assert!(x > 0, "x must be positive")

// 相等断言
assert_eq!(left, right)
assert_eq!(result, expected, "calculation failed")

// 不等断言
assert_ne!(a, b)

// 浮点数比较
assert_approx_eq!(3.14, pi, epsilon: 0.01)

// 错误断言
assert_err!(result)
assert_ok!(result)
```

### 编译模型

**编写时**: `*_test.dast` 文件与正式代码在同一个模块中（目录=模块）
- 可以访问私有函数和类型
- 便于白盒测试

**编译后**: 测试文件被自动排除
- 正式构建忽略 `*_test.dast` 文件
- `dast test` 会把 `*_test.dast` 仅加入入口目录对应的模块（依赖模块不包含测试文件）
- 零运行时开销

```bash
$ dast build  # 自动忽略 *_test.dast
$ dast test   # 包含 *_test.dast
```

### 预期失败

```dast
// 使用属性标记
@should_panic
fn test_divide_by_zero() {
    let _ = 1 / 0  // 应该 panic
}

@should_panic(expected: "division by zero")
fn test_specific_panic() {
    divide(10, 0)
}
```

### 忽略测试

```dast
@ignore
fn test_expensive() {
    // 默认不运行，除非 --ignored
}

@ignore(reason: "waiting for bug fix")
fn test_broken() {
    // ...
}
```

---

## Fuzz 测试

### 基础语法

```dast
// fuzz_ 前缀自动识别为 fuzz 测试
fn fuzz_parse_input(data: &[u8]) {
    // 测试不应该 panic
    let _ = parse(data)
}

fn fuzz_json_parser(input: &str) {
    if let .Ok(parsed) = parse_json(input) {
        // 验证往返一致性
        let serialized = serialize_json(parsed)
        let reparsed = parse_json(&serialized)
        assert_eq!(parsed, reparsed)
    }
}
```

### 运行 Fuzz 测试

```bash
# 运行 fuzz 测试
$ dast fuzz fuzz_parse_input

# 指定运行时间
$ dast fuzz fuzz_parse_input --time=60s

# 使用语料库
$ dast fuzz fuzz_parse_input --corpus=testdata/corpus

# 最小化失败案例
$ dast fuzz fuzz_parse_input --minimize
```

### Fuzz 测试目录结构

```
my_project/
├── src/
│   ├── parser.dast
│   └── parser_test.dast
└── fuzz/
    ├── fuzz_parse_input/
    │   └── corpus/
    │       ├── input1.txt
    │       └── input2.txt
    └── fuzz_json_parser/
        └── corpus/
```

---

## 基准测试

### 基础语法

```dast
// bench_ 前缀自动识别为基准测试
fn bench_addition(b: &mut Bencher) {
    b.iter(|| {
        let _ = 2 + 2
    })
}

fn bench_string_concat(b: &mut Bencher) {
    b.iter(|| {
        let s = "hello".to_string() + " world"
        s
    })
}
```

### 高级基准测试

```dast
fn bench_with_setup(b: &mut Bencher) {
    let data = setup_large_dataset()

    b.iter(|| {
        process(data.clone())
    })
}

fn bench_with_input(b: &mut Bencher) {
    b.iter_with_setup(
        || generate_input(),  // setup
        |input| process(input)  // benchmark
    )
}
```

---

## 异步测试

```dast
async fn test_async_function() {
    let result = fetch_data("url").await
    assert_ok!(result)
}

async fn test_timeout() {
    let result = timeout(
        Duration.seconds(1),
        slow_operation()
    ).await

    assert_err!(result)  // 应该超时
}
```

---

## 测试运行

```bash
# 运行所有测试
$ dast test

# 运行特定测试
$ dast test test_addition

# 运行特定模块
$ dast test tests::integration

# 运行被忽略的测试
$ dast test -- --ignored

# 并行运行
$ dast test -- --test-threads=4

# 显示输出
$ dast test -- --nocapture

# 运行基准测试
$ dast bench

# 运行特定基准测试
$ dast bench bench_addition
```

---

## 测试覆盖率

```bash
# 生成覆盖率报告
$ dast test --coverage

# 生成 HTML 报告
$ dast test --coverage --coverage-format=html

# 设置最小覆盖率
$ dast test --coverage --min-coverage=80
```

---

## 与其他语言对比

| 特性 | Rust | Go | Dast |
|------|------|-----|------|
| 单元测试 | `#[test]` | `func Test*` | `test_*` 函数 |
| 测试文件 | 同文件 `#[cfg(test)]` | `*_test.go` | `*_test.dast` |
| 断言 | `assert!` | `t.Assert` | `assert!` |
| 基准测试 | `#[bench]` | `func Benchmark*` | `bench_*` 函数 |
| Fuzz 测试 | `#[fuzz]` | `func Fuzz*` | `fuzz_*` 函数 |
| 异步测试 | `#[tokio::test]` | goroutine | `async fn test_*` |
| 覆盖率 | tarpaulin | go test -cover | `--coverage` |

---

## 最终设计

| 特性 | 决策 |
|------|------|
| 测试文件 | `*_test.dast` (Go 风格) |
| 测试函数 | `test_*` 前缀 |
| 基准测试 | `bench_*` 前缀 |
| Fuzz 测试 | `fuzz_*` 前缀 |
| 断言 | `assert!` 宏 |
| 覆盖率 | 内置支持 |

**核心**: 完全采用 Go 风格，简洁直观
