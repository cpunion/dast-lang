# GPU 与图形支持

## 设计目标

1. **广泛 API 支持**: OpenGL, Vulkan, DirectX, Metal, WebGPU
2. **GPU 计算**: CUDA, OpenCL, Metal Compute, WebGPU Compute
3. **着色器策略**: 当前使用外部着色器文件，未来支持语言内 DSL
4. **跨平台抽象**: 统一上层 API，按平台选择后端

---

## 图形 API 平台映射

```
┌─────────────────────────────────────────────────────────────┐
│                    图形 API 平台矩阵                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  平台          │ 首选 API      │ 备选 API     │ 着色器语言  │
│  ──────────────┼───────────────┼──────────────┼───────────  │
│  Windows       │ DirectX 12    │ Vulkan       │ HLSL/SPIR-V │
│  macOS         │ Metal         │ -            │ MSL         │
│  Linux         │ Vulkan        │ OpenGL 4.5   │ SPIR-V/GLSL │
│  iOS           │ Metal         │ -            │ MSL         │
│  Android       │ Vulkan        │ OpenGL ES 3  │ SPIR-V/GLSL │
│  Web (WASM)    │ WebGPU        │ WebGL 2      │ WGSL/GLSL   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 架构层次

```
┌─────────────────────────────────────────────────────────────┐
│                   Dast 图形架构                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Layer 3: 高级抽象 (可选)                                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  场景图 │ 材质系统 │ 后处理 │ 粒子 │ UI 渲染        │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                 │
│                           ▼                                 │
│  Layer 2: 跨平台渲染接口 (核心)                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Device │ Buffer │ Texture │ Shader │ Pipeline │ ...│    │
│  │  统一类型，平台无关                                  │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                 │
│                           ▼                                 │
│  Layer 1: 平台后端实现                                       │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐  │
│  │ Vulkan   │ DirectX  │ Metal    │ WebGPU   │ OpenGL   │  │
│  │ 后端     │ 后端     │ 后端     │ 后端     │ 后端     │  │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘  │
│                                                             │
│  Layer 0: 着色器                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  SPIR-V (通用) → 转译为 HLSL/MSL/WGSL/GLSL           │    │
│  │  或: 各平台原生着色器文件                            │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 着色器管理 (当前阶段)

### 外部着色器文件方式

```
project/
├── src/
│   └── main.dast
└── shaders/
    ├── vertex.glsl
    ├── fragment.glsl
    ├── compute.glsl
    └── vertex.spv        # 预编译 SPIR-V
```

```
// Dast 代码中加载着色器
const shader = gpu.load_shader(
    vertex: "shaders/vertex.glsl",
    fragment: "shaders/fragment.glsl",
)

// 或预编译的 SPIR-V
const shader = gpu.load_spirv(
    vertex: embed_file("shaders/vertex.spv"),
    fragment: embed_file("shaders/fragment.spv"),
)
```

### 着色器编译工作流

```
┌────────────────────────────────────────────────────────────┐
│                  着色器编译管线                             │
├────────────────────────────────────────────────────────────┤
│                                                            │
│   开发时:                                                  │
│   ┌──────────┐    ┌──────────┐    ┌──────────────────┐    │
│   │ .glsl    │───▶│ glslc    │───▶│ .spv (SPIR-V)    │    │
│   │ .hlsl    │    │ dxc      │    │                  │    │
│   └──────────┘    └──────────┘    └──────────────────┘    │
│                                                            │
│   运行时 (按需):                                          │
│   ┌──────────┐    ┌────────────────────────────────────┐  │
│   │ .spv     │───▶│ SPIRV-Cross 转译                   │  │
│   │ (SPIR-V) │    │  ├─▶ MSL (Metal)                   │  │
│   └──────────┘    │  ├─▶ HLSL (DirectX)                │  │
│                   │  ├─▶ WGSL (WebGPU)                 │  │
│                   │  └─▶ GLSL (OpenGL/WebGL)           │  │
│                   └────────────────────────────────────┘  │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

---

## GPU 计算

```
// 计算着色器加载
const compute = gpu.load_compute("shaders/particle_sim.glsl")

// 创建计算管线
let pipeline = gpu.compute_pipeline(compute)

// 绑定资源
pipeline.bind(0, particle_buffer)
pipeline.bind(1, output_buffer)

// 调度执行
gpu.dispatch(pipeline,
    groups_x: particle_count / 64,
    groups_y: 1,
    groups_z: 1
)
```

---

## 未来: 语言内 DSL (设计草案)

> ⚠️ 未来功能，当前不实现

```
// 设想 1: 宏展开生成着色器代码
@shader(type: .vertex)
fn vertex_main(
    @location(0) pos: vec3,
    @location(1) uv: vec2,
) -> VertexOutput {
    var out: VertexOutput
    out.position = uniforms.mvp * vec4(pos, 1.0)
    out.uv = uv
    return out
}

// 设想 2: 内联着色器块 (类似 CUDA)
fn render() {
    @gpu kernel {
        let idx = global_id.x
        output[idx] = input[idx] * 2.0
    }
}
```

**DSL 设计考量**:
- 需要元编程能力成熟后
- 需要支持 GPU 特殊类型 (vec2, mat4, sampler2D)
- 需要跨平台着色器生成器

---

## 与其他模块的集成

| 模块 | 集成方式 |
|------|---------|
| 内存管理 | GPU 缓冲区使用专用分配器 (arena/pool) |
| Hot Reload | 着色器热更新 (重新编译并替换管线) |
| 异步 | GPU 命令提交为异步操作 |
| WASM | WebGPU 后端，JS 互操作 |

---

## 待讨论

- [ ] 统一渲染接口的具体 API 设计
- [ ] 着色器反射 (自动生成绑定代码)
- [ ] GPU 资源生命周期管理
- [ ] 渲染图 (Render Graph) 抽象
- [ ] 调试/性能分析工具集成
