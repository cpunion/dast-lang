# 宏系统设计

## 设计目标

1. **卫生宏** - 避免名称冲突
2. **类型安全** - 编译期检查
3. **简洁** - 比 Rust 过程宏更易用
4. **强大** - 支持代码生成

---

## 宏的层次

### 1. 声明宏 (Declarative Macros)

类似 Rust 的 `macro_rules!`，基于模式匹配

```dast
macro vec {
    () => { Vec.new() },
    ($($x:expr),+ $(,)?) => {
        {
            let mut temp = Vec.new()
            $(temp.push($x))*
            temp
        }
    }
}

// 使用
let v = vec![1, 2, 3]
```

### 2. 过程宏 (Procedural Macros)

编译期执行的函数，操作 AST

```dast
// 派生宏
@derive(Clone, Debug, Serialize)
struct Point { x: f32, y: f32 }

// 属性宏
@route(GET, "/users/:id")
fn get_user(id: i32) -> User { ... }

// 函数式宏
let sql = sql!("SELECT * FROM users WHERE id = ?")
```

---

## 与 Comptime 的关系

### 关键区别

| 特性 | Comptime | 宏 |
|------|---------|-----|
| 执行时机 | 编译期 | 编译期 |
| 输入 | 值、类型 | AST/Token |
| 输出 | 值、类型 | AST/Token |
| 用途 | 计算、反射 | 代码生成、DSL |

### 结合使用

```dast
// Comptime 生成数据
comptime const TABLE = generate_table()

// 宏生成代码
macro generate_accessors {
    ($struct_name:ident, $($field:ident: $ty:ty),*) => {
        impl $struct_name {
            $(
                fn get_$field(self: &Self) -> $ty {
                    self.$field
                }
            )*
        }
    }
}
```

---

## 方案对比

### 方案 A: Rust 风格（过程宏 + 声明宏）

```dast
// 声明宏
macro vec { ... }

// 过程宏（需要单独 crate）
// proc_macro crate
fn derive_serialize(input: TokenStream) -> TokenStream {
    // 解析 AST，生成代码
}
```

**优点**: 成熟、强大
**缺点**: 过程宏复杂（需要单独 crate）

---

### 方案 B: Comptime 宏（Zig 风格）

```dast
// 用 comptime 实现宏
comptime fn derive_serialize(T: type) -> type {
    // 编译期反射 + 代码生成
    return generated_impl
}

@derive_serialize
struct Point { x: f32, y: f32 }
```

**优点**: 统一、简洁
**缺点**: 需要强大的编译期反射

---

### 方案 C: 混合方案（推荐）

```dast
// 简单场景: 声明宏
macro vec { ... }

// 复杂场景: Comptime 宏
comptime fn derive[T](trait_name: &str) {
    comptime match trait_name {
        "Clone" => generate_clone_impl(T),
        "Debug" => generate_debug_impl(T),
        _ => @compile_error("unknown trait"),
    }
}

@derive("Clone")
struct Point { x: f32, y: f32 }
```

**优点**: 灵活、渐进
**缺点**: 两套系统

---

## 推荐设计

### 第一阶段: 声明宏

```dast
macro vec {
    () => { Vec.new() },
    ($($x:expr),+) => {
        {
            let mut v = Vec.new()
            $(v.push($x))*
            v
        }
    }
}
```

### 第二阶段: Comptime 宏

```dast
// 利用编译期反射实现派生
comptime fn auto_derive(T: type, trait_name: &str) {
    comptime if trait_name == "Clone" {
        // 生成 Clone 实现
        impl Clone for T {
            fn clone(self: &Self) -> Self {
                Self {
                    comptime for i in 0..@field_count(T) {
                        @field_name(T, i): self.@field_name(T, i).clone(),
                    }
                }
            }
        }
    }
}

@derive(Clone)
struct Point { x: f32, y: f32 }
```

### 第三阶段: 过程宏（可选）

```dast
// 如果 comptime 不够用，提供过程宏
@proc_macro
fn custom_derive(input: TokenStream) -> TokenStream {
    // 完全自定义的 AST 操作
}
```

---

## 内置宏

### 格式化宏

```dast
println!("x = {}, y = {}", x, y)
format!("Hello, {}!", name)
```

### 断言宏

```dast
assert!(x > 0)
assert_eq!(a, b)
debug_assert!(condition)
```

### 向量宏

```dast
vec![1, 2, 3]
vec![0; 10]  // [0, 0, ..., 0]
```

---

## 与其他语言对比

| 语言 | 宏系统 | 强度 |
|------|--------|------|
| **C/C++** | 文本替换 | ⭐⭐ |
| **Rust** | 声明宏 + 过程宏 | ⭐⭐⭐⭐⭐ |
| **Zig** | Comptime | ⭐⭐⭐⭐ |
| **Nim** | 模板 + 宏 | ⭐⭐⭐⭐⭐ |
| **Dast** | 声明宏 + Comptime | ⭐⭐⭐⭐⭐ |

---

## 最终设计：Comptime + AST 宏

### 两层宏系统

```dast
// 层次 1: Comptime - 类型和值的编译期操作
comptime fn derive(T: type, trait: &str) {
    comptime match trait {
        "Clone" => generate_clone_impl(T),
        "Debug" => generate_debug_impl(T),
        _ => @compile_error("unknown trait"),
    }
}

@derive(Clone, Debug)
struct Point { x: f32, y: f32 }

// 层次 2: AST 宏 - 语法扩展和 DSL
macro sql(query: AstNode) -> AstNode {
    comptime {
        let parsed = parse_sql(query.string_value())
        if !parsed.is_valid() {
            @compile_error("Invalid SQL")
        }
    }

    quote! {
        Query[#infer_type(parsed)] {
            sql: #query,
        }
    }
}

let users = sql!("SELECT id, name FROM users")
// 类型: Query<(i32, String)>
```

---

## 实际应用示例

### SQL 宏

```dast
macro sql(query: AstNode) -> AstNode {
    comptime {
        let parsed = parse_sql(query.string_value())
        validate_sql(parsed)
        let result_type = infer_result_type(parsed)
    }

    quote! {
        Query[#result_type] {
            sql: #query,
            params: vec![],
        }
    }
}

// 编译期类型安全
let query = sql!("SELECT id, name, email FROM users WHERE age > ?")
// 类型: Query<(i32, String, String)>
```

### HTML 宏

```dast
macro html(template: AstNode) -> AstNode {
    comptime {
        let dom = parse_html_ast(template)
        validate_html(dom)
    }

    generate_dom_builder(dom)
}

let page = html! {
    <div class="container">
        <h1>{"Hello"}</h1>
        <p>{user.name}</p>
    </div>
}
```

### Regex 宏

```dast
macro regex(pattern: AstNode) -> AstNode {
    comptime {
        let compiled = compile_regex(pattern.string_value())
        if let .Err(e) = compiled {
            @compile_error(format("Invalid regex: {}", e))
        }
    }

    quote! {
        Regex { pattern: #pattern, compiled: #compiled }
    }
}

let email = regex!(r"^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$")
```

---

## 优势总结

| 特性 | Comptime + AST 宏 |
|------|------------------|
| 覆盖率 | 100% |
| 复杂度 | 中（比 Rust 简单） |
| 类型安全 | ✅ |
| DSL 支持 | ✅ |
| 编译期验证 | ✅ |

**核心**:
- Comptime 处理类型和反射
- AST 宏处理语法扩展
- 两者结合覆盖所有场景

