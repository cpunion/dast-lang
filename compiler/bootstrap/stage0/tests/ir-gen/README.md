# IR v0 值语义测试

## 运行测试
```bash
./compiler/bootstrap/stage0/dast-stage0 ir compiler/bootstrap/stage0/tests/ir-gen/value_params.dast
```

## 期望：值参数直接使用，无 load

### value_params.dast
```dast
fn add(a: int, b: int) -> int {
    a + b
}
```

### 期望 IR
```
fn add(a: int, b: int) -> int
  block entry0:
    t0: int = + a, b
    return t0
```

### 错误 IR（不应生成）
```
fn add(a: int, b: int) -> int
  block entry0:
    t0 = load a        # 错误！a 是值参数
    t1 = load b        # 错误！b 是值参数
    t2 = + t0, t1
    return t2
```
