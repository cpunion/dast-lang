# C++ 风格借鉴与安全改进

## 设计理念

**借鉴 C++ 的优点**:
- RAII 资源管理
- 零成本抽象
- 值语义与移动语义
- 模板/泛型编程
- 操作符重载
- 确定性析构

**摒弃 C++ 的不安全**:
- 未初始化变量
- 空指针解引用
- 悬垂指针
- 缓冲区溢出
- 数据竞争
- 未定义行为 (大部分)

---

## 从 C++ 借鉴的特性

### 1. RAII (已采用)

```cpp
// C++ 风格
class File {
    int fd;
public:
    File(const char* path) { fd = open(path); }
    ~File() { close(fd); }  // 析构函数
};

void use_file() {
    File f("data.txt");
    // 使用 f
}  // 自动调用 ~File()
```

```dast
// Dast (相同理念)
struct File {
    fd: i32,
}

impl Drop for File {
    fn drop(self: &mut Self) {
        close(self.fd)
    }
}

fn use_file() {
    let f = File.open("data.txt")
    // 使用 f
}  // 自动调用 drop()
```

---

### 2. 值语义与移动语义

```cpp
// C++11 移动语义
class Buffer {
    char* data;
    size_t size;
public:
    // 移动构造
    Buffer(Buffer&& other) noexcept
        : data(other.data), size(other.size) {
        other.data = nullptr;
        other.size = 0;
    }

    // 移动赋值
    Buffer& operator=(Buffer&& other) noexcept {
        if (this != &other) {
            delete[] data;
            data = other.data;
            size = other.size;
            other.data = nullptr;
            other.size = 0;
        }
        return *this;
    }
};

Buffer b1 = create_buffer();
Buffer b2 = std::move(b1);  // 显式移动
```

```dast
// Dast (自动移动)
struct Buffer {
    data: *mut u8,
    size: usize,
}

impl Drop for Buffer {
    fn drop(self: &mut Self) {
        if self.data != null {
            free(self.data)
        }
    }
}

let b1 = create_buffer()
let b2 = b1  // 自动移动，b1 不可再用
```

**改进**: 无需显式 `std::move`，编译器自动判断

---

### 3. 智能指针

```cpp
// C++11 智能指针
#include <memory>

std::unique_ptr<Node> node = std::make_unique<Node>();
std::shared_ptr<Data> data = std::make_shared<Data>();
std::weak_ptr<Data> weak = data;

// 使用
node->process();
data->value = 42;
```

```dast
// Dast (统一接口)
let node: Box<Node> = Box.new(Node{})
let data: Shared<Data> = Shared.new(Data{})
let weak: Weak<Data> = data.downgrade()

// 使用 (相同语法)
node.process()
data.value = 42
```

**改进**:
- 统一的 `Box`/`Shared`/`Weak` 命名
- 无需 `make_unique`/`make_shared`
- 自动选择单线程/多线程版本

---

### 4. 模板/泛型

```cpp
// C++ 模板
template<typename T>
class Vec {
    T* data;
    size_t len;
    size_t cap;
public:
    void push(T value);
    T& operator[](size_t index);
};

Vec<int> numbers;
numbers.push(42);
```

```dast
// Dast 泛型
struct Vec<T> {
    data: *mut T,
    len: usize,
    cap: usize,
}

impl<T> Vec<T> {
    fn push(self: &mut Self, value: T) { ... }
    fn get(self: &Self, index: usize) -> &T { ... }
}

let numbers: Vec<i32> = Vec.new()
numbers.push(42)
```

**改进**:
- 更清晰的语法 (`<T>` vs `template<typename T>`)
- 无需前置声明
- 更好的错误信息

---

### 5. 操作符重载

```cpp
// C++ 操作符重载
class Vec2 {
    float x, y;
public:
    Vec2 operator+(const Vec2& other) const {
        return Vec2{x + other.x, y + other.y};
    }

    Vec2& operator+=(const Vec2& other) {
        x += other.x;
        y += other.y;
        return *this;
    }
};

Vec2 a{1, 2}, b{3, 4};
Vec2 c = a + b;
a += b;
```

```dast
// Dast 操作符重载
struct Vec2 {
    x: f32,
    y: f32,
}

impl Add for Vec2 {
    fn add(self: Self, other: Self) -> Self {
        return Vec2 { x: self.x + other.x, y: self.y + other.y }
    }
}

impl AddAssign for Vec2 {
    fn add_assign(self: &mut Self, other: Self) {
        self.x += other.x
        self.y += other.y
    }
}

let a = Vec2{1.0, 2.0}
let b = Vec2{3.0, 4.0}
let c = a + b
a += b
```

**改进**:
- Trait 系统更清晰
- 无需记忆 `operator+` 语法

---

### 6. 构造函数/析构函数

```cpp
// C++ 构造/析构
class Resource {
    int* data;
public:
    // 构造函数
    Resource(int size) : data(new int[size]) {}

    // 析构函数
    ~Resource() { delete[] data; }

    // 禁用拷贝
    Resource(const Resource&) = delete;
    Resource& operator=(const Resource&) = delete;
};
```

```dast
// Dast (更显式)
struct Resource {
    data: *mut i32,
}

impl Resource {
    // 构造函数 (普通函数)
    fn new(size: usize) -> Self {
        return Resource {
            data: alloc(size * size_of::<i32>())
        }
    }
}

impl Drop for Resource {
    fn drop(self: &mut Self) {
        free(self.data)
    }
}

// 默认不可复制 (除非实现 Copy trait)
```

**改进**:
- 构造函数是普通函数 (`new`)，更灵活
- 默认移动语义，无需 `= delete`
- 析构函数通过 Drop trait，更统一

---

## 摒弃 C++ 的不安全特性

### 1. 未初始化变量

```cpp
// ❌ C++: 未初始化变量
int x;  // 未定义值
int* ptr;  // 悬垂指针
std::cout << x;  // UB
```

```dast
// ✅ Dast: 强制初始化
let x: i32  // 编译错误: 必须初始化
let x: i32 = 0  // OK

// 或显式标记未初始化 (unsafe)
let x: i32 = undefined
unsafe {
    // 程序员保证在使用前初始化
    x = 42
}
```

---

### 2. 空指针解引用

```cpp
// ❌ C++: 空指针可以隐式解引用
int* ptr = nullptr;
*ptr = 42;  // 崩溃，但编译通过
```

```dast
// ✅ Dast: 类型系统区分可空/非空
let ptr: *mut i32 = null  // 裸指针可空

unsafe {
    *ptr = 42  // unsafe 块中才能解引用
}

// 安全方式: 使用 Option
let opt: Option<Box<i32>> = .none
match opt {
    .some(val) => *val = 42,  // 安全
    .none => println("null"),
}
```

---

### 3. 悬垂指针

```cpp
// ❌ C++: 悬垂指针
int* create_dangling() {
    int x = 42;
    return &x;  // 返回栈上变量的地址
}

int* ptr = create_dangling();
*ptr = 100;  // UB
```

```dast
// ✅ Dast: 编译器禁止
fn create_dangling() -> &i32 {
    let x = 42
    return &x  // 编译错误: x 在函数返回时销毁
}

// 正确方式
fn create_valid() -> Box<i32> {
    return Box.new(42)  // 堆上分配
}
```

---

### 4. 缓冲区溢出

```cpp
// ❌ C++: 无边界检查
int arr[10];
arr[100] = 42;  // 缓冲区溢出，UB
```

```dast
// ✅ Dast: 边界检查
let arr: [i32; 10] = [0; 10]
arr[100] = 42  // Debug: panic, Release: 可选检查

// 或使用 unsafe 绕过检查 (性能关键路径)
unsafe {
    arr.get_unchecked_mut(5) = 42
}
```

---

### 5. 数据竞争

```cpp
// ❌ C++: 数据竞争
int counter = 0;

void increment() {
    for (int i = 0; i < 1000; ++i) {
        ++counter;  // 数据竞争
    }
}

std::thread t1(increment);
std::thread t2(increment);
t1.join();
t2.join();
// counter 的值不确定
```

```dast
// ✅ Dast: 编译器检测或运行时工具
let counter = 0

fn increment() {
    for i in 0..1000 {
        counter += 1  // 编译错误: 跨线程访问需要同步
    }
}

// 正确方式
let counter = Atomic.new(0)
fn increment() {
    for i in 0..1000 {
        counter.fetch_add(1)
    }
}
```

---

### 6. 多重继承的菱形问题

```cpp
// ❌ C++: 菱形继承
class A { public: int value; };
class B : public A {};
class C : public A {};
class D : public B, public C {};  // D 有两个 A::value

D d;
d.value = 42;  // 编译错误: 歧义
d.B::value = 42;  // 需要显式指定
```

```dast
// ✅ Dast: 无继承，使用组合 + Trait
trait Display {
    fn display(self: &Self)
}

struct A { value: i32 }
struct B { a: A }
struct C { a: A }
struct D { b: B, c: C }

// 无歧义，显式访问
let d = D { ... }
d.b.a.value = 42
d.c.a.value = 100
```

---

### 7. 隐式类型转换

```cpp
// ❌ C++: 危险的隐式转换
void process(bool flag) { ... }

process(42);  // 隐式 int -> bool，总是 true

unsigned int a = 10;
int b = -5;
if (a > b) { ... }  // b 隐式转为 unsigned，变成很大的正数
```

```dast
// ✅ Dast: 禁止隐式转换
fn process(flag: bool) { ... }

process(42)  // 编译错误: 类型不匹配
process(42 != 0)  // OK: 显式转换

let a: u32 = 10
let b: i32 = -5
if a > b { ... }  // 编译错误: 无法比较 u32 和 i32
if a > b as u32 { ... }  // OK: 显式转换
```

---

### 8. 宏的文本替换

```cpp
// ❌ C++: 不安全的宏
#define MAX(a, b) ((a) > (b) ? (a) : (b))

int x = 5;
int y = MAX(++x, 10);  // x 被递增两次！
```

```dast
// ✅ Dast: 类型安全的泛型函数
fn max<T: Ord>(a: T, b: T) -> T {
    if a > b { a } else { b }
}

let x = 5
let y = max(x + 1, 10)  // 无副作用问题

// 或编译期函数
comptime fn max(a: i32, b: i32) -> i32 {
    if a > b { a } else { b }
}
```

---

## C++ 现代特性的 Dast 对应

| C++ 特性 | Dast 对应 | 改进 |
|---------|----------|------|
| `std::unique_ptr` | `Box<T>` | 更简洁 |
| `std::shared_ptr` | `Shared<T>` | 自动选择 Rc/Arc |
| `std::optional` | `Option<T>` | 统一语法 |
| `std::variant` | `enum` | 更强大的模式匹配 |
| `std::move` | 自动移动 | 无需显式标记 |
| `constexpr` | `comptime` | 更灵活 |
| `template` | 泛型 | 更清晰的语法 |
| `concept` (C++20) | Trait bound | 更成熟的系统 |
| `auto` | 类型推断 | 相同 |
| `lambda` | 闭包 | 更简洁 |

---

## 语法对比示例

### 函数定义

```cpp
// C++
template<typename T>
std::optional<T> find(const std::vector<T>& vec, const T& target) {
    for (const auto& item : vec) {
        if (item == target) {
            return item;
        }
    }
    return std::nullopt;
}
```

```dast
// Dast
fn find<T: Eq>(vec: &[T], target: &T) -> Option<T> {
    for item in vec {
        if item == target {
            return .some(*item)
        }
    }
    return .none
}
```

### 类/结构体

```cpp
// C++
class Point {
    float x, y;
public:
    Point(float x, float y) : x(x), y(y) {}

    float distance() const {
        return std::sqrt(x * x + y * y);
    }
};
```

```dast
// Dast
struct Point {
    x: f32,
    y: f32,
}

impl Point {
    fn new(x: f32, y: f32) -> Self {
        return Point { x, y }
    }

    fn distance(self: &Self) -> f32 {
        return (self.x * self.x + self.y * self.y).sqrt()
    }
}
```

### 错误处理

```cpp
// C++ (异常)
int divide(int a, int b) {
    if (b == 0) {
        throw std::runtime_error("division by zero");
    }
    return a / b;
}

try {
    int result = divide(10, 0);
} catch (const std::exception& e) {
    std::cerr << e.what() << std::endl;
}
```

```dast
// Dast (Result)
fn divide(a: i32, b: i32) -> Result<i32, Error> {
    if b == 0 {
        return .err("division by zero")
    }
    return .ok(a / b)
}

match divide(10, 0) {
    .ok(result) => println("{}", result),
    .err(e) => eprintln("{}", e),
}

// 或使用 ?
fn calculate() -> Result<i32, Error> {
    let result = divide(10, 2)?
    return .ok(result * 2)
}
```

---

## 总结

### 保留的 C++ 优点
- ✅ RAII 自动资源管理
- ✅ 零成本抽象
- ✅ 值语义与移动语义
- ✅ 泛型编程
- ✅ 操作符重载
- ✅ 确定性析构

### 摒弃的 C++ 缺陷
- ❌ 未初始化变量
- ❌ 空指针/悬垂指针
- ❌ 缓冲区溢出
- ❌ 数据竞争
- ❌ 多重继承
- ❌ 隐式类型转换
- ❌ 不安全的宏

### Dast 的改进
- 🎯 更简洁的语法
- 🎯 更好的类型推断
- 🎯 编译期内存安全检查
- 🎯 统一的错误处理
- 🎯 现代化的模块系统
- 🎯 更友好的错误信息

**核心理念**: C++ 的表达力 + Rust 的安全性 - 复杂度
