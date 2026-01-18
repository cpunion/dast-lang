# 热更新

## 设计目标

1. **模块级热更新** - 运行时替换模块代码
2. **多版本共存** - 允许同一模块的多个版本同时运行
3. **零成本可选** - 不使用时无额外开销
4. **双模式执行** - 字节码（灵活）+ 机器码（性能）

---

## 架构设计

### 函数指针表

```dast
// 模块函数通过指针表间接调用
struct ModuleTable {
    version: u32,
    functions: HashMap[String, *const ()],
}

// 调用示例
fn call_module_function(name: &str, args: &[Value]) -> Value {
    let table = get_current_module_table()
    let func_ptr = table.functions.get(name).unwrap()
    unsafe {
        // 通过函数指针调用
        call_dynamic(func_ptr, args)
    }
}
```

### 多版本共存

```dast
// 版本管理
struct ModuleRegistry {
    modules: HashMap[String, Vec[ModuleVersion]],
}

struct ModuleVersion {
    version: u32,
    state: VersionState,  // Active, Draining, Unloaded
    code: CodeHandle,     // Bytecode or Native
}

enum VersionState {
    Active,    // 新调用使用此版本
    Draining,  // 等待旧调用完成
    Unloaded,  // 已卸载
}
```

---

## 双模式执行

### 字节码模式（开发）

```bash
# 生成字节码
$ dast build --mode=dev --hot-reload

# 输出: target/dev/module.dbc
```

**优势**:
- 编译快速
- 热更新无缝
- 调试友好

### 机器码模式（生产）

```bash
# 生成优化的机器码
$ dast build --mode=release

# 输出: target/release/libmodule.so
```

**优势**:
- 性能最优
- 无解释器开销

### 混合模式

```bash
# 核心模块机器码，插件字节码
$ dast build --mode=hybrid \
    --native=core,render \
    --bytecode=plugins/*
```

---

## 状态迁移

### 方案 1: 显式钩子

```dast
@hot_reload
module GameLogic {
    var state: GameState

    // 卸载前: 导出状态
    fn __on_unload__() -> Vec[u8] {
        return serialize(state)
    }

    // 加载后: 导入状态
    fn __on_load__(data: Option[Vec[u8]]) {
        if let .Some(data) = data {
            state = deserialize(data)
        } else {
            state = GameState.default()
        }
    }
}
```

### 方案 2: 共享内存

```dast
// 状态定义在独立的共享区，布局固定
@shared
@abi_stable(1)
struct GameState {
    score: i32,
    level: i32,
    // 新版本可追加字段，但不能修改已有字段
}

// 状态分配在共享区
@shared
static mut STATE: GameState = GameState { score: 0, level: 1 }
```

### 方案 3: 无状态函数

```dast
// 纯函数，无状态，可自由替换
@hot_reload
fn calculate_damage(atk: i32, def: i32) -> i32 {
    return max(0, atk - def)
}

// 状态保留在 host 侧，不参与热更
```

---

## 使用示例

### 游戏逻辑热更新

```dast
// main.dast (host)
fn main() {
    let mut registry = ModuleRegistry.new()

    // 加载初始版本
    registry.load_module("game_logic", "v1.0")

    // 游戏循环
    loop {
        // 检查是否有新版本
        if registry.has_update("game_logic") {
            registry.reload_module("game_logic")
        }

        // 调用模块函数
        registry.call("game_logic", "update", &[delta_time])
        registry.call("game_logic", "render", &[])

        sleep(16)  // 60 FPS
    }
}
```

### 插件系统

```dast
// 插件接口
trait Plugin {
    fn init(self: &mut Self)
    fn update(self: &mut Self, dt: f32)
    fn shutdown(self: &mut Self)
}

// 动态加载插件
fn load_plugin(path: &str) -> Box[dyn Plugin] {
    let module = load_bytecode(path)
    let plugin = module.instantiate[dyn Plugin]()
    plugin.init()
    return plugin
}
```

---

## 编译配置

### 开发模式

```toml
# dast.toml
[profile.dev]
hot-reload = true
bytecode = true
opt-level = 0
```

### 生产模式

```toml
[profile.release]
hot-reload = false
bytecode = false
opt-level = 3
lto = true
```

### 混合模式

```toml
[profile.hybrid]
hot-reload = true

[profile.hybrid.module.core]
bytecode = false
opt-level = 3

[profile.hybrid.module.plugins]
bytecode = true
opt-level = 1
```

---

## 限制与注意事项

### 不支持热更新的场景

```dast
// ❌ 全局静态变量
static mut GLOBAL: i32 = 0

// ❌ 复杂的类型布局变更
struct OldVersion {
    x: i32,
}

struct NewVersion {
    x: i64,  // 类型改变
    y: i32,  // 新增字段
}
```

### 最佳实践

```dast
// ✅ 使用版本化的 ABI
@abi_stable(1)
struct Config {
    // 只追加，不修改
}

// ✅ 使用序列化迁移
fn migrate_state(old: &[u8], version: u32) -> GameState {
    match version {
        1 => deserialize_v1(old),
        2 => deserialize_v2(old),
        _ => panic("unsupported version"),
    }
}

// ✅ 无状态函数优先
@hot_reload
fn pure_function(input: &Data) -> Result {
    // 无副作用，易于热更新
}
```

---

## 与无 GC 的兼容

### 模块引用计数

```dast
// 模块级引用计数（非对象级）
struct ModuleHandle {
    module: *const Module,
    ref_count: Atomic[usize],
}

impl Drop for ModuleHandle {
    fn drop(self: &mut Self) {
        if self.ref_count.fetch_sub(1) == 1 {
            // 最后一个引用，可以卸载模块
            unload_module(self.module)
        }
    }
}
```

---

## 调试支持

### 热更新日志

```bash
$ dast run --hot-reload --log-level=debug

[INFO] Module 'game_logic' loaded (v1.0)
[INFO] Detected code change in game_logic.dast
[INFO] Compiling game_logic v1.1...
[INFO] Module 'game_logic' reloaded (v1.0 -> v1.1)
[DEBUG] v1.0 draining (2 active calls)
[DEBUG] v1.0 unloaded
```

### 版本查询

```dast
// 运行时查询模块版本
fn get_module_version(name: &str) -> Option[u32] {
    registry.get_version(name)
}

// 调试信息
fn print_module_info() {
    for (name, versions) in registry.modules {
        println("{}: {} versions", name, versions.len())
        for v in versions {
            println("  v{} - {:?}", v.version, v.state)
        }
    }
}
```

---

## 总结

| 特性 | 支持 |
|------|------|
| 模块级热更新 | ✅ |
| 多版本共存 | ✅ |
| 字节码模式 | ✅ |
| 机器码模式 | ✅ |
| 混合模式 | ✅ |
| 状态迁移 | ✅ 多种方案 |
| 零成本可选 | ✅ |
| 调试支持 | ✅ |

**适用场景**:
- 游戏开发
- 实时系统
- 长时间运行的服务
- 插件系统
