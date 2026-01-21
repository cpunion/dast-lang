# Dast IR 设计（纯底层）

> **核心理念**：IR 只关心内存和计算，所有高级概念在前端完全展开。

## 关键设计权衡

### 类型保留 vs 布局展开

| 方案 | 优点 | 缺点 |
|------|------|------|
| **保留类型名** `type Point = { x: i32, y: i32 }` | 可读、调试友好、保留语义 | 后端需计算布局 |
| **纯偏移** `offset(p, 4)` | 真正底层、后端简单 | 难读、丢失语义 |

### 字段访问 vs 偏移

| 方案 | 优点 | 缺点 |
|------|------|------|
| **字段名** `p.x` | 可读、平台无关 | 后端需计算偏移 |
| **偏移** `offset(p, 4)` | 后端直接使用 | 平台相关、难读 |

### 决策：**保留类型和字段名**

理由：
1. **跨平台**：偏移依赖平台 ABI（对齐、padding）
2. **可调试**：保留语义信息便于调试
3. **优化**：类型信息有助于别名分析等优化
4. **简单**：布局计算是后端职责，不复杂

## IR 设计

### 类型声明

```
type Point = { x: i32, y: i32 }
type Option = { tag: u8, payload: i64 }   // enum 展开
```

- 保留类型名和字段名
- enum 展开为 struct，但保留结构
- 后端负责计算实际偏移/大小

### 变量

```
fn foo(a: i32, b: *Point) -> i32 {
    var x: i32
    var p: Point
    ...
}
```

### 内存访问

```
%0 = load(ptr)              // 从指针读
store(ptr, val)             // 写入指针
%0 = field(ptr, name)       // 取字段地址（不是偏移）
%0 = index(ptr, idx)        // 数组索引
```

### 完整指令集（5 类）

```
// 1. 内存
%0 = load(ptr)
store(ptr, val)
%0 = field(ptr, name)       // 结构体字段
%0 = index(ptr, idx)        // 数组索引

// 2. 算术/逻辑
%0 = binop(op, a, b)
%0 = unop(op, a)

// 3. 调用
%0 = call(func, args...)

// 4. 分配
%0 = alloca(T)              // 动态栈分配
memcpy(dst, src, T)         // 按类型拷贝
memset(dst, val, T)         // 按类型填充

// 5. 原子/Intrinsic
%0 = atomic(op, ptr, val, ordering)
%0 = intrinsic(name, args...)
```

### 终结符（3 种）

```
return [val]
goto(block)
switch(val, [(v1, b1), ...], default)
```

## 示例

### 结构体

```dast
struct Point { x: i32, y: i32 }
let p = Point { x: 1, y: 2 }
let v = p.x
```

展开为：

```
type Point = { x: i32, y: i32 }

fn main() {
    var p: Point
    var v: i32
    
    block entry:
        %0 = field(p, x)           // &p.x
        store(%0, const 1)
        %1 = field(p, y)           // &p.y
        store(%1, const 2)
        %2 = load(%0)              // p.x
        store(v, %2)
        return
}
```

### enum

```dast
enum Option { Some(i32), None }
let x = Option.Some(42)
```

展开为：

```
type Option = { tag: u8, payload: i32 }   // enum 展开

fn main() {
    var x: Option
    
    block entry:
        %0 = field(x, tag)
        store(%0, const u8 0)      // Some = 0
        %1 = field(x, payload)
        store(%1, const 42)
        return
}
```

### 引用参数

```dast
fn inc(x: &mut i32) { *x = *x + 1 }
```

展开为：

```
fn inc(x: *i32) {
    var x: *i32                    // 参数也是 var
    
    block entry:
        %0 = load(x)               // 读指针参数
        %1 = load(%0)              // *x
        %2 = binop(+, %1, const 1)
        store(%0, %2)
        return
}
```

## 与 IR v0 对比

| 特性 | IR v0 (当前) | 新 IR |
|------|-------------|--------|
| 类型名 | ✓ 保留 | ✓ 保留 |
| 字段访问 | `p.x` | `field(p, x)` |
| enum | 特殊指令 | 展开为 struct |
| 闭包 | 特殊指令 | 展开为 struct |

**主要变化**：
- enum/closure 展开为普通 struct，减少特殊指令
- `p.x` 统一为 `field(p, x)`，语义更明确
- 保留类型信息，后端负责布局

## 总结

```
┌────────────────────────────────────────────┐
│  IR 设计原则                                 │
├────────────────────────────────────────────┤
│  ✓ 保留类型名和字段名（跨平台、可调试）      │
│  ✓ enum/closure 展开为 struct（减少特殊指令） │
│  ✓ 后端负责布局计算（不在 IR 中硬编码偏移）  │
├────────────────────────────────────────────┤
│  5 类指令 + 3 种终结符                       │
└────────────────────────────────────────────┘
```
