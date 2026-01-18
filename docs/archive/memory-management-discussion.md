# 内存管理模型

## 核心原则

1. **无 GC** - 不使用垃圾回收器
2. **无引用计数** - 不使用 RC/ARC
3. **RAII** - 资源获取即初始化，退出作用域即释放
4. **完整借用检查** - Rust 级别安全，无需生命周期标注
5. **栈优先** - 默认栈分配，堆分配显式 (Box)

---

## 核心决策: RAII 自动释放

采用 C++/Rust 的 RAII 模式：**值离开作用域时自动调用析构函数释放资源**。

```
┌─────────────────────────────────────────────────────────────┐
│                    RAII 生命周期                             │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  fn example() {                                             │
│      let file = File.open("data.txt")  // 1. 资源获取       │
│      let buffer = Buffer.new(1024)     // 2. 资源获取       │
│                                                             │
│      process(file, buffer)              // 3. 使用资源      │
│                                                             │
│  }  // ← 作用域结束                                         │
│     //   buffer.drop() 自动调用                             │
│     //   file.drop() 自动调用                               │
│     //   (逆序析构，后创建先释放)                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Drop Trait (析构接口)

```
// 定义析构行为
trait Drop {
    fn drop(self: &mut Self)
}

// 实现示例
struct File {
    fd: i32,
}

impl Drop for File {
    fn drop(self: &mut Self) {
        close(self.fd)  // 释放系统资源
    }
}

// 使用: 自动调用，无需手动
fn read_file() {
    let file = File.open("x.txt")
    // ... 使用 file
}  // file.drop() 自动调用
```

### 与手动 delete 的区别

```
// ❌ 旧设计 (需要 defer 或手动)
let ptr = new Node{...}
defer delete ptr

// ✅ 新设计 (RAII 自动)
let node = Box.new(Node{...})  // Box 是 RAII 包装器
// 作用域结束自动释放，无需 defer
```

---

## 智能指针类型

### 类型层次

```
┌─────────────────────────────────────────────────────────────┐
│                    所有权类型层次                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. 值类型 (栈上，自动复制或移动)                            │
│     let point = Point{x: 1, y: 2}                           │
│                                                             │
│  2. Box<T> (唯一所有权堆分配，RAII 释放)                     │
│     let node = Box.new(Node{...})                           │
│     // 作用域结束自动释放                                   │
│                                                             │
│  3. Unique<T> (类似 Box，但可空)                             │
│     let opt: Unique<Node> = Unique.new(node)                │
│     opt = null  // 立即释放                                 │
│                                                             │
│  4. 裸指针 (手动管理，unsafe 场景)                           │
│     let ptr: *mut Node = alloc(...)                         │
│     // 必须手动 free                                        │
│                                                             │
│  5. Arena 批量分配 (整体释放)                                │
│     let arena = Arena.new(4096)                             │
│     let a = arena.alloc<Node>()                             │
│     let b = arena.alloc<Node>()                             │
│     // arena 释放时全部回收                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Box<T> 详解

```
// Box: 堆分配 + 唯一所有权 + RAII
struct Box<T> {
    ptr: *mut T,
}

impl<T> Box<T> {
    fn new(value: T) -> Box<T> {
        let ptr = alloc(size_of<T>())
        *ptr = value
        return Box{ptr}
    }
}

impl<T> Drop for Box<T> {
    fn drop(self: &mut Self) {
        (*self.ptr).drop()  // 先析构内容
        free(self.ptr)       // 再释放内存
    }
}

// 使用
fn example() {
    let b = Box.new(LargeStruct{...})
    process(&b)           // 借用访问
}  // 自动释放
```

---

## 所有权与移动语义

### 规则

```
┌─────────────────────────────────────────────────────────────┐
│                    所有权规则                                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  规则 1: 小型值类型 → 默认复制                               │
│  ┌─────────────────────────────────────────────┐            │
│  │ let a = Point{1, 2}                         │            │
│  │ let b = a           // 复制，a 仍可用       │            │
│  │ // Point 是 Copy 类型 (小且无资源)          │            │
│  └─────────────────────────────────────────────┘            │
│                                                             │
│  规则 2: 资源类型 → 默认移动                                 │
│  ┌─────────────────────────────────────────────┐            │
│  │ let file = File.open("x.txt")               │            │
│  │ let f2 = file       // 移动，file 不可用    │            │
│  │ // file.read()      // 编译错误             │            │
│  └─────────────────────────────────────────────┘            │
│                                                             │
│  规则 3: Box<T> 等智能指针 → 移动                            │
│  ┌─────────────────────────────────────────────┐            │
│  │ let b1 = Box.new(data)                      │            │
│  │ let b2 = b1         // 移动，b1 不可用      │            │
│  │ // 防止双重释放                             │            │
│  └─────────────────────────────────────────────┘            │
│                                                             │
│  规则 4: 借用不获取所有权                                    │
│  ┌─────────────────────────────────────────────┐            │
│  │ fn print(p: &Point) { ... }                 │            │
│  │ let a = Point{1, 2}                         │            │
│  │ print(&a)           // 借用                 │            │
│  │ // a 仍可用                                 │            │
│  └─────────────────────────────────────────────┘            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Copy vs Move 判定

```
// Copy trait: 可按位复制，无需 Drop
trait Copy {}

// 默认 Copy 的类型:
// - 基础类型: i8, i16, i32, i64, u8, ..., f32, f64, bool
// - 固定大小数组（元素是 Copy）: [i32; 10]
// - 小型结构体（所有字段是 Copy，无 Drop）

// 必须 Move 的类型:
// - 实现了 Drop 的类型
// - 包含非 Copy 字段的类型
// - Box<T>, Unique<T>, File 等资源类型

// 显式标记
@derive(Copy)  // 启用复制
struct Point { x: f32, y: f32 }

@move_only     // 强制移动（即使可以 Copy）
struct Handle { id: u64 }
```

---

## 生命周期 (简化版)

**设计**: 比 Rust 更简单，无需显式 `'a` 标注，依赖编译器推断 + 简单规则

```
// Dast: 自动推断，无需标注
fn longest(x: &str, y: &str) -> &str {
    if x.len() > y.len() { x } else { y }
}
// 编译器推断: 返回值生命周期 = min(x, y)

// 简化规则:
// 1. 引用不能逃逸其来源的作用域
// 2. 返回的引用生命周期 <= 所有输入引用
// 3. 无法推断时 → 编译错误，要求使用值返回
```

### 禁止悬垂引用

```
fn bad() -> &i32 {
    let x = 42
    return &x  // 编译错误: x 在函数返回时销毁
}

fn good() -> i32 {
    let x = 42
    return x   // OK: 返回值
}
```

---

## 分配层次

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 1: 栈分配 (默认)                                       │
│ ─────────────────────────────────────────────────────────── │
│ • 所有局部变量默认在栈上                                     │
│ • 退出作用域自动调用 Drop (如果有)                           │
│                                                             │
│ let point = Point{x: 1, y: 2}  // 栈上，Copy 类型           │
│ let file = File.open("x.txt")  // 栈上，有 Drop             │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Layer 2: 静态分配                                            │
│ ─────────────────────────────────────────────────────────── │
│ • 全局常量 → .rodata section                                │
│ • 静态变量 → .data/.bss section                             │
│                                                             │
│ const TABLE: [u8; 256] = init_table()  // 编译期计算        │
│ static counter: Atomic<i32> = Atomic.new(0)                 │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Layer 3: 堆分配 (RAII 智能指针)                              │
│ ─────────────────────────────────────────────────────────── │
│ • 使用 Box<T> 等智能指针                                     │
│ • 作用域结束自动释放                                         │
│                                                             │
│ let data = Box.new(LargeData{...})  // 堆上，自动释放       │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Layer 4: Arena 批量分配                                      │
│ ─────────────────────────────────────────────────────────── │
│ • 大量小对象一起分配一起释放                                 │
│ • 适合编译器 AST、游戏帧数据等                               │
│                                                             │
│ let arena = Arena.new(64 * 1024)                            │
│ for item in items {                                         │
│     let node = arena.alloc<Node>()                          │
│     // 不单独释放                                           │
│ }                                                           │
│ // arena 离开作用域，一次性回收全部                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 分配器设计

### 默认分配器

```
// 全局默认分配器 (可配置)
static default_allocator: Allocator = system_allocator()

// 类型可指定分配器
let arena = Arena.new(4096)
let box: Box<Node, arena> = Box.new_in(node, arena)
```

### Arena 分配器

```
struct Arena {
    chunks: List<Chunk>,
    current: *mut u8,
    end: *mut u8,
}

impl Arena {
    fn alloc<T>(self: &mut Self) -> *mut T {
        // 无单独释放，返回裸指针
        // 整个 Arena drop 时释放所有 chunks
    }
}

impl Drop for Arena {
    fn drop(self: &mut Self) {
        for chunk in self.chunks {
            free(chunk.ptr)
        }
    }
}
```

---

## 特殊场景

### 嵌入式无堆模式

```bash
$ dast build --no-heap
```

```
// 编译检查: 任何使用 Box/alloc 的路径 → 编译错误
fn process() {
    let buf = Box.new(data)  // 编译错误: no-heap 模式禁用堆
}

// 替代: 使用静态缓冲区
static buf: Buffer<1024> = Buffer.new()
```

### 栈使用静态分析

```bash
$ dast analyze-stack main.dast

Function stack usage:
  main:         128 bytes
    ├── init:    32 bytes
    └── process: 64 bytes
        └── helper: 16 bytes

Worst-case total: 240 bytes
Warning: Recursive calls in 'traverse' - stack unbounded
```

---

## 与 Rust 的关键区别

| 特性 | Rust | Dast |
|------|------|------|
| 生命周期标注 | 显式 `'a` | 隐式推断 |
| 借用检查复杂度 | 完整 | 简化核心规则 |
| 自引用结构 | Pin + unsafe | 禁止或 unsafe |
| 分配器传递 | 泛型参数 | 上下文/默认 |
| RAII | ✓ Drop trait | ✓ Drop trait |
| Copy/Move | ✓ 显式 | ✓ 自动推断 |

---

## 待讨论

- [x] RAII vs 手动释放 → **决定: RAII**
- [ ] Box/Unique 命名是否采用其他关键字
- [ ] Drop 调用顺序在异常/错误时的行为
- [ ] unsafe 块的具体语法
- [ ] 与 Hot Reload 状态迁移的结合
