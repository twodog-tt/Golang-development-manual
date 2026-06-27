---
id: S-MEM-07
title: interface 底层 eface/iface 与断言成本
module: memory-gc
level: senior
frequency: 4
go_version: "1.22+"
tags: [interface, eface, iface, type-assertion]
status: published
code_refs: []
sources:
  - https://go.dev/ref/spec#Interface_types
  - https://research.swtch.com/interfaces
  - https://pkg.go.dev/internal/abi#Type
---

# interface 底层 eface/iface 与断言成本

## 30 秒版（开场）

> 空接口 **`interface{}`/`any` 是 eface**（type, data）；带方法的接口是 **iface**（itab, data）。**类型断言**需查 itab 或 type 相等，失败走 slow path；**频繁 dynamic dispatch** 阻碍内联。生产关键词：**少 interface 热点、type switch、具体类型 API**。

## 3 分钟版（一面深度）

1. **是什么**：interface 是指针大小的类型描述 + 数据指针；iface 的 itab 缓存 `(动态类型, 接口类型)` 的方法表。
2. **为什么**：实现多态与解耦；代价是装箱、间接调用、逃逸与 GC 扫描指针。
3. **怎么做**：热路径用泛型/代码生成/具体类型；断言用 `v, ok := x.(T)`；避免层层 `interface{}` 传递。

## 10 分钟版（原理 + 图示）

**内存布局**

```
eface { *_type, unsafe.Pointer data }
iface { *itab, unsafe.Pointer data }
itab  { inter *InterfaceType, _type *Type, hash uint32, fun [1]uintptr }
```

**赋值装箱**：具体值赋给 interface 时，小值可能直接存在 data（word 内），大值则 data 指向堆拷贝。

```mermaid
flowchart LR
  Concrete[具体类型 T] -->|装箱| IF[iface/eface]
  IF -->|方法调用| ITAB[itab.fun[i]]
  ITAB --> Fn[实际函数]
```

**断言成本**

| 形式 | 成本 |
|------|------|
| `x.(T)` 成功 | 比较 itab/type，O(1) |
| `x.(T)` 失败 panic | 同上 + panic |
| type switch | 多次比较，编译器可能优化为跳转表 |
| 反射 `reflect.Value` | 远高于断言 |

**nil interface 陷阱**：`var p *T=nil; var i interface{}=p` → i 非 nil（type 非空）。

## 生产场景

- **中间件链**：`func(ctx, interface{})` 层层传递，allocs 与 CPU 双高。
- **ORM/JSON**：大量 `map[string]interface{}`，GC 与 CPU profile 顶部常驻。
- **可观测**：`pprof` 见 `runtime.convT*`、`runtime.assert*`。

## 排查与工具

| 工具 | 用途 |
|------|------|
| `pprof allocs` | convT 系列 |
| `-gcflags=-m` | interface 实参导致逃逸 |
| bench | 具体类型 vs interface 对比 |

路径：allocs 高 → 查 interface 传参 → 改泛型/代码生成 → 复测 B/op。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| 泛型 API（1.18+） | 容器、算法 | 异构插件系统 |
| 小 interface 面 | io.Reader 等标准 | 上帝 interface |
| 类型注册表 + ID | RPC 多类型 | 少量固定类型 |
| 代码生成 | 序列化热点 | 小项目维护成本 |

## 追问链

1. **eface 与 iface 区别？** → 有无 itab/方法集。
2. **为何 nil 指针赋 interface 不是 nil？** → data 与 type 二元组，type 已设置。
3. **itab 何时生成？** → 首次 (T, I) 组合，全局缓存。
4. **接口比较？** → 可比较 type+value；slice/map 作 dynamic type 不可比。
5. **泛型消除 interface 吗？** → 单态化减少 boxing，但非零成本抽象仍有。

## 反模式与事故

- 热路径 `map[string]interface{}` 做业务模型。
- `if err != nil` 里 err 已是 typed nil 仍判非 nil 的逻辑 bug。
- 为「灵活」全项目 `Any`，性能回归难定位。

## 代码示例

```go
// 反模式：热路径 interface{}
func Process(v any) { /* reflect or type switch */ }

// 改进：泛型或分类型 API
func ProcessInt(v int) { /* ... */ }

// 正确判断 typed nil
func IsNilInterface(i any) bool {
    if i == nil {
        return true
    }
    rv := reflect.ValueOf(i)
    switch rv.Kind() {
    case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Chan, reflect.Func:
        return rv.IsNil()
    }
    return false
}
```

## 延伸阅读

- [Russ Cox: Go Data Structures: Interfaces](https://research.swtch.com/interfaces)
- [Go spec: Interface types](https://go.dev/ref/spec#Interface_types)
- [Go 1.18 Generics](https://go.dev/doc/go1.18)
