# Dast 编译器自举规划

## 概述

Dast 采用**分阶段自举**策略：

- **Stage0**：Go 实现的 bootstrap 编译器/运行时（IR v0 + 解释器）
- **Stage2**：Dast 实现的编译器（当前以 QBE 后端为主）
- **Stage3**：工具链与优化（fmt/lint/LSP/opt）

```
Stage 0 (Go)                 Stage 2 (Dast)
┌──────────────────────┐     ┌──────────────────────┐
│  compiler/stage0/    │  →  │  compiler/stage2/    │
└──────────────────────┘     └──────────────────────┘
         运行 Stage2                   完整编译器
```

## 目录结构

### Stage0（Go）

```
compiler/stage0/
├── cmd/dast/              # 主入口
├── internal/              # AST/Parser/Typecheck/IR/Interp
├── runtime/               # 运行时（C）
├── stdlib/                # 标准库（Stage0）
├── tests/                 # Stage0 测试
└── dast-stage0            # 编译后的二进制
```

### Stage2（Dast）

```
compiler/stage2/
├── driver/                # 入口
├── frontend/              # 词法/语法/AST/类型检查
├── middle/                # 中端/代码生成（QBE）
├── stdlib/                # 标准库
└── tests/                 # 测试
```

## Runtime ABI（C）

Stage0/Stage2 的代码生成依赖 C 运行时（`compiler/stage0/runtime/c_runtime.c`）。当前公开 ABI 主要包括：

### 内存/分配与对象释放
- `dast_alloc`
- `dast_free`
- `dast_mem_store`
- `dast_mem_load_s`
- `dast_mem_load_u`
- `dast_mem_field_addr`
- `dast_mem_field_addr_size`
- `dast_mem_copy_field`
- `dast_alloc_total_bytes`
- `dast_alloc_total_peak_bytes`

### 数组
- `dast_array_new`
- `dast_array_free`
- `dast_array_len`
- `dast_array_index_addr`
- `dast_array_push`
- `dast_array_get_s`
- `dast_array_get_u`
- `dast_array_get_s_unchecked`
- `dast_array_get_u_unchecked`
- `dast_array_set`
- `dast_array_set_unchecked`
- `dast_push`
- `dast_pop`

### 字符串
- `dast_string_len`
- `dast_string_eq`
- `dast_string_concat`
- `dast_string_clone`
- `dast_string_free`
- `dast_char_at`
- `dast_substr`
- `dast_int_to_string`
- `dast_parse_int`
- `dast_string_to_int`
- `dast_has_prefix`

### I/O 与系统
- `dast_read_file`
- `dast_write_file`
- `dast_read_dir`
- `dast_read_line`
- `dast_read_bytes`
- `dast_args`
- `dast_getenv`
- `dast_exec`
- `dast_mkdir`
- `dast_exit`
- `dast_set_args`

### 打印
- `dast_print_space`, `dast_print_newline`
- `dast_eprint_space`, `dast_eprint_newline`
- `dast_print_i64`, `dast_print_bool`, `dast_print_string`, `dast_print_struct`, `dast_print_array`, `dast_print_ptr`
- `dast_eprint_i64`, `dast_eprint_bool`, `dast_eprint_string`, `dast_eprint_struct`, `dast_eprint_array`, `dast_eprint_ptr`

### 宏/AST 支持
- `dast_ast_expr1`, `dast_ast_expr2`
- `dast_ast_stmt1`, `dast_ast_stmt2`
- `dast_ast_item1`, `dast_ast_item2`
- `dast_ast_block1`, `dast_ast_block2`
- `dast_ast_to_string`
- `dast_ast_eq`
- `dast_ast_assert_eq`
- `dast_gensym`
- `dast_bind`

## IR v0

- 规范：`docs/19-ir-spec.md`  
- 实现：Stage0 `compiler/stage0/internal/ir/`

当前 IR v0 仅用于 Stage0。Stage2 直接生成 QBE，不输出 IR v0。

## 使用方式

```bash
# 运行 Stage2（由 Stage0 驱动）
make test-stage2

# Stage0 编译/运行
./compiler/stage0/dast-stage0 build src/ -o app.out
./compiler/stage0/dast-stage0 run main.dast
```

## 关键设计原则

1. **永久保留 Stage0**：bootstrap 编译器永远可用  
2. **IR v0 稳定**：Stage0 依赖 IR v0  
3. **跨阶段一致**：Stage0/Stage2 的测试与行为尽量一致
