# 标准库设计

## 核心模块划分

### 模块层次结构

```
std/
├── core/           # 核心类型和 trait
├── collections/    # 集合类型
├── io/            # I/O 抽象
├── fs/            # 文件系统
├── net/           # 网络
├── sync/          # 同步原语
├── async/         # 异步运行时
├── string/        # 字符串处理
├── mem/           # 内存操作
├── ptr/           # 指针操作
├── os/            # 操作系统接口
├── time/          # 时间处理
├── math/          # 数学函数
└── fmt/           # 格式化
```

---

## 核心类型 (std.core)

### 基础类型

```dast
// 整数
i8, i16, i32, i64, i128, isize
u8, u16, u32, u64, u128, usize

// 浮点数
f32, f64

// 布尔
bool

// 字符
char  // Unicode scalar value

// 单元类型
()
```

### 核心 Trait

```dast
// 复制
trait Copy { }
trait Clone {
    fn clone(self: &Self) -> Self
}

// 比较
trait Eq {
    fn eq(self: &Self, other: &Self) -> bool
}

trait Ord {
    fn cmp(self: &Self, other: &Self) -> Ordering
}

// 运算符
trait Add[Rhs = Self] {
    type Output
    fn add(self: Self, rhs: Rhs) -> Self.Output
}

// 转换
trait From[T] {
    fn from(value: T) -> Self
}

trait Into[T] {
    fn into(self: Self) -> T
}
```

---

## 集合类型 (std.collections)

### Vec - 动态数组

```dast
struct Vec[T] {
    // 内部实现
}

impl[T] Vec[T] {
    fn new() -> Self
    fn with_capacity(capacity: usize) -> Self

    fn push(self: &mut Self, value: T)
    fn pop(self: &mut Self) -> Option[T]

    fn len(self: &Self) -> usize
    fn is_empty(self: &Self) -> bool

    fn get(self: &Self, index: usize) -> Option[&T]
    fn get_mut(self: &mut Self, index: usize) -> Option[&mut T]

    fn iter(self: &Self) -> Iter[T]
    fn iter_mut(self: &mut Self) -> IterMut[T]
}

// 使用
let mut v = Vec.new()
v.push(1)
v.push(2)
```

### HashMap - 哈希表

```dast
struct HashMap[K, V] {
    // 内部实现
}

impl[K: Eq + Hash, V] HashMap[K, V] {
    fn new() -> Self
    fn with_capacity(capacity: usize) -> Self

    fn insert(self: &mut Self, key: K, value: V) -> Option[V]
    fn get(self: &Self, key: &K) -> Option[&V]
    fn remove(self: &mut Self, key: &K) -> Option[V]

    fn contains_key(self: &Self, key: &K) -> bool
    fn len(self: &Self) -> usize

    fn iter(self: &Self) -> Iter[K, V]
}

// 使用
let mut map = HashMap.new()
map.insert("key", "value")
```

### HashSet - 哈希集合

```dast
struct HashSet[T] {
    // 内部实现
}

impl[T: Eq + Hash] HashSet[T] {
    fn new() -> Self

    fn insert(self: &mut Self, value: T) -> bool
    fn contains(self: &Self, value: &T) -> bool
    fn remove(self: &mut Self, value: &T) -> bool

    fn len(self: &Self) -> usize
}
```

### String - 字符串

```dast
struct String {
    // UTF-8 编码
}

impl String {
    fn new() -> Self
    fn from(s: &str) -> Self

    fn push(self: &mut Self, ch: char)
    fn push_str(self: &mut Self, s: &str)

    fn len(self: &Self) -> usize
    fn is_empty(self: &Self) -> bool

    fn as_str(self: &Self) -> &str
    fn chars(self: &Self) -> Chars
}

// 使用
let mut s = String.from("hello")
s.push_str(" world")
```

---

## I/O 抽象 (std.io)

### 核心 Trait

```dast
trait Read {
    fn read(self: &mut Self, buf: &mut [u8]) -> Result[usize, Error]

    fn read_to_end(self: &mut Self, buf: &mut Vec[u8]) -> Result[usize, Error]
    fn read_to_string(self: &mut Self, buf: &mut String) -> Result[usize, Error]
}

trait Write {
    fn write(self: &mut Self, buf: &[u8]) -> Result[usize, Error]
    fn flush(self: &mut Self) -> Result[(), Error]

    fn write_all(self: &mut Self, buf: &[u8]) -> Result[(), Error]
}

trait Seek {
    fn seek(self: &mut Self, pos: SeekFrom) -> Result[u64, Error]
}
```

### 缓冲 I/O

```dast
struct BufReader[R: Read] {
    inner: R,
}

impl[R: Read] BufReader[R] {
    fn new(inner: R) -> Self
    fn with_capacity(capacity: usize, inner: R) -> Self
}

impl[R: Read] Read for BufReader[R] {
    fn read(self: &mut Self, buf: &mut [u8]) -> Result[usize, Error]
}

// 使用
let file = File.open("data.txt")?
let reader = BufReader.new(file)
```

### 标准输入输出

```dast
fn stdin() -> Stdin
fn stdout() -> Stdout
fn stderr() -> Stderr

// 使用
let line = stdin().read_line()?
stdout().write_all(b"Hello\n")?
```

---

## 文件系统 (std.fs)

```dast
struct File {
    // 内部实现
}

impl File {
    fn open(path: &str) -> Result[Self, Error]
    fn create(path: &str) -> Result[Self, Error]

    fn read_to_string(path: &str) -> Result[String, Error]
    fn write(path: &str, contents: &[u8]) -> Result[(), Error]
}

impl Read for File { }
impl Write for File { }
impl Seek for File { }

// 使用
let content = File.read_to_string("config.toml")?
File.write("output.txt", b"data")?
```

---

## 同步原语 (std.sync)

### Mutex

```dast
struct Mutex[T] {
    // 内部实现
}

impl[T] Mutex[T] {
    fn new(value: T) -> Self
    fn lock(self: &Self) -> MutexGuard[T]
}

// 使用
let mutex = Mutex.new(0)
{
    let mut guard = mutex.lock()
    *guard += 1
}
```

### RwLock

```dast
struct RwLock[T] {
    // 内部实现
}

impl[T] RwLock[T] {
    fn new(value: T) -> Self
    fn read(self: &Self) -> RwLockReadGuard[T]
    fn write(self: &Self) -> RwLockWriteGuard[T]
}
```

### Channel

```dast
fn channel[T]() -> (Sender[T], Receiver[T])

struct Sender[T] {
    fn send(self: &Self, value: T) -> Result[(), Error]
}

struct Receiver[T] {
    fn recv(self: &Self) -> Result[T, Error]
}

// 使用
let (tx, rx) = channel()
tx.send(42)?
let value = rx.recv()?
```

---

## 与其他语言对比

| 模块 | Rust | Go | Dast |
|------|------|-----|------|
| 集合 | std::collections | container | std.collections |
| I/O | std::io | io | std.io |
| 文件 | std::fs | os | std.fs |
| 网络 | std::net | net | std.net |
| 同步 | std::sync | sync | std.sync |
| 字符串 | String | string | String |

---

## 设计原则

1. **零成本抽象** - trait 编译为静态分发
2. **明确所有权** - 所有 API 清晰表达所有权
3. **错误处理** - 使用 Result 而非异常
4. **模块化** - 清晰的模块边界
5. **最小化** - 核心库保持精简

---

## 最终设计

| 特性 | 决策 |
|------|------|
| 集合 | Vec, HashMap, HashSet, String |
| I/O | Read/Write trait |
| 同步 | Mutex, RwLock, Channel |
| 错误 | Result[T, Error] |
| 字符串 | UTF-8 String + &str |

**核心**: 类似 Rust，零成本抽象
