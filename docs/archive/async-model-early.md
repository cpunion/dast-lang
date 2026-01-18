# 异步编程模型

## 设计目标

1. **async/await 语法**: 现代协程语法
2. **Pull 模式 (惰性)**: 只在 await 点推进执行
3. **零成本抽象**: 无运行时分配，状态机内联

---

## 执行模型

**Pull 模式** (如 Rust Future):
- 消费者主动 poll → 按需执行 → 惰性求值
- 特点: 无背压，可内联，零成本

---

## 语法设计

```
async fn fetch_data(url: &str) -> Result(Data, Error) {
    let response = await http.get(url)?
    let body = await response.read_body()?
    return parse(body)
}

// 并发组合
async fn fetch_all() {
    let (a, b) = await join(fetch("url1"), fetch("url2"))
}
```

---

## 编译实现

编译器将 async 函数转换为状态机:
- 每个 await 点是状态边界
- 跨 await 变量保存在状态结构中
- Future 大小编译期静态计算，栈上分配

---

## 运行时选项

| 类型 | 适用场景 |
|------|---------|
| 单线程事件循环 | 嵌入式/简单应用 |
| 多线程工作窃取 | 高并发服务器 |
| 无运行时 block_on | 同步调用 async |

---

## 待讨论

- [ ] Future trait 设计
- [ ] 取消语义
- [ ] 结构化并发
