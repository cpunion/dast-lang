# 平台支持

## 设计目标

支持从嵌入式到云端的全平台开发，提供统一的语言体验。

---

## 目标平台

### 嵌入式裸机

**支持架构**:
- ARM Cortex-M0/M3/M4 (STM32, nRF52)
- RISC-V RV32IMC (ESP32-C3)
- AVR (Arduino)

**特性**:
```dast
// no_std 模式
#![no_std]

// 静态内存分配
static BUFFER: [u8; 1024] = [0; 1024]

// 中断处理
@interrupt
fn timer_handler() {
    // 中断服务例程
}
```

**编译选项**:
```bash
$ dast build --target=thumbv7em-none-eabi --no-heap
```

---

### 边缘计算

**支持平台**:
- Raspberry Pi 4/5 (ARM64)
- NVIDIA Jetson (ARM64)
- RISC-V 开发板 (VisionFive 2)

**特性**:
- 完整标准库
- 静态链接优先 (musl libc)
- 交叉编译支持

```bash
$ dast build --target=aarch64-unknown-linux-musl
```

---

### 主机平台

**支持系统**:
- Linux (x86_64, ARM64)
- macOS (ARM64, x86_64)
- Windows (x86_64, ARM64)

**特性**:
- 完整标准库
- 动态库支持
- 调试信息 (DWARF/PDB)

```bash
# 动态库
$ dast build --crate-type=dylib

# 静态库
$ dast build --crate-type=staticlib
```

---

### WebAssembly

**运行环境**:
- 浏览器 (WebGL/WebGPU)
- WASI (命令行工具)
- 嵌入式 WASM 运行时 (Wasmtime, Wasmer)

**特性**:
```dast
// WASM 导出
@wasm_export
fn add(a: i32, b: i32) -> i32 {
    return a + b
}

// JS 互操作
@wasm_import("env", "console_log")
extern fn js_log(ptr: *const u8, len: usize)
```

**编译**:
```bash
$ dast build --target=wasm32-unknown-unknown
$ wasm-opt -O3 output.wasm -o optimized.wasm
```

---

### 移动端

**支持平台**:
- iOS (ARM64) - Metal 图形
- Android (ARM64, x86_64) - Vulkan/OpenGL ES

**集成方式**:
```bash
# iOS 静态库
$ dast build --target=aarch64-apple-ios --crate-type=staticlib

# Android 共享库
$ dast build --target=aarch64-linux-android --crate-type=cdylib
```

---

## 交叉编译

### 支持矩阵

| 宿主 \\ 目标 | Linux x64 | Linux ARM64 | macOS ARM64 | Windows | WASM | 裸机 ARM |
|-------------|-----------|-------------|-------------|---------|------|----------|
| Linux x64   | ✅         | ✅           | ✅           | ✅       | ✅    | ✅        |
| macOS ARM64 | ✅         | ✅           | ✅           | ✅       | ✅    | ✅        |
| Windows x64 | ✅         | ✅           | ⚠️           | ✅       | ✅    | ✅        |

✅ 完整支持  ⚠️ 需额外工具链

### 配置

```toml
# dast.toml
[target.aarch64-unknown-linux-gnu]
linker = "aarch64-linux-gnu-gcc"

[target.thumbv7em-none-eabi]
runner = "qemu-system-arm"
```

---

## 平台特性映射

| 特性 | 裸机 | 边缘 | 主机 | WASM | 移动 |
|------|------|------|------|------|------|
| 核心语言 | ✅ | ✅ | ✅ | ✅ | ✅ |
| no_std | ✅ | ⚠️ | ⚠️ | ✅ | ⚠️ |
| 堆分配 | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| 动态库 | ❌ | ⚠️ | ✅ | ❌ | ⚠️ |
| 线程 | ❌ | ✅ | ✅ | ⚠️ | ✅ |
| 异步 | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| GPU | ❌ | ⚠️ | ✅ | ✅ | ✅ |

✅ 完整支持  ⚠️ 可选/有限  ❌ 不支持

---

## no_std 模式

### 启用

```dast
#![no_std]

// 使用 core 而非 std
use core::mem
use core::ptr
```

### 限制

- 无堆分配（除非提供自定义分配器）
- 无线程
- 无文件系统
- 无网络

### 可用模块

- `core::*` - 核心类型和 trait
- `alloc::*` - 堆分配（需要分配器）

---

## 静态栈分析

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

## 总结

| 平台 | 主要用途 | 关键特性 |
|------|---------|---------|
| 裸机 | 嵌入式、IoT | no_std, 静态分析 |
| 边缘 | 边缘计算、网关 | 静态链接, 交叉编译 |
| 主机 | 桌面应用、服务器 | 完整标准库 |
| WASM | Web 应用、沙箱 | 体积优化, JS 互操作 |
| 移动 | 移动应用 | 原生图形 API |
