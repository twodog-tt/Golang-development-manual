---
id: S-AI-08
title: 多模态与语音接入：图像、音频在 Go 服务中的工程实践
module: ai-engineering
level: senior
frequency: 4
go_version: "1.22+"
tags: [multimodal, speech, vision, whisper, tts, streaming]
status: published
code_refs: []
sources:
  - https://platform.openai.com/docs/guides/speech-to-text
  - https://platform.openai.com/docs/guides/text-to-speech
  - https://platform.openai.com/docs/guides/images
---

# 多模态与语音接入：图像、音频在 Go 服务中的工程实践

## 30 秒版（开场）

> 多模态 = 同一对话里传 **文本 + 图片 + 音频**；Go 后端负责 **上传校验、转码、流式转发、计费**。语音链路常见 **ASR（Whisper）→ LLM → TTS**。生产关键词：**multipart 限大小、base64 vs 对象存储 URL、实时语音 WebSocket**。

## 3 分钟版（精讲深度）

1. **是什么**：Vision 模型读图答问；ASR 把语音转文本；TTS 把回复合成语音；部分模型支持 **音频输入直接理解**（原生多模态）。
2. **为什么**：客服、质检、会议助手、电商以图搜款 — JD  increasingly 要求「接过大模型 + 语音/图像」。
3. **怎么做**：按 provider 限制选择 base64 或对象存储短期签名 URL；上传入口在解析 multipart 前用 `http.MaxBytesReader` 限制总大小，并实际解码/探测 MIME、像素、时长和压缩炸弹；流式链路传播客户端取消。

## 10 分钟版（原理 + 图示）

```mermaid
sequenceDiagram
  participant App as 客户端
  participant API as Go API
  participant OSS as 对象存储
  participant LLM as 多模态模型
  App->>API: 上传图片/音频
  API->>OSS: 存原文件(可选)
  API->>LLM: messages + image_url / input_audio
  LLM-->>API: 文本或音频流
  API-->>App: JSON / SSE / 音频流
```

**某些 Chat Completions 兼容 API 的消息结构（示意）**

```go
// 图像：推荐 URL 方式（省请求体）
msg := map[string]any{
    "role": "user",
    "content": []any{
        map[string]any{"type": "text", "text": "这张图里有什么？"},
        map[string]any{
            "type": "image_url",
            "image_url": map[string]any{
                "url":    signedURL,
                "detail": "low", // low/high 影响 token 与费用
            },
        },
    },
}
```

**语音 ASR（Whisper 类 API）**

```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/audio/transcriptions", body)
if err != nil {
    return err
}
req.Header.Set("Authorization", "Bearer "+apiKey)
// multipart: file + model=whisper-1 + language=zh
```

**TTS 流式播放**

- 请求 provider 支持的音频格式；有些 API 直接以 chunked HTTP body 返回音频，并不存在通用的 `stream: true` 请求字段
- Go 可边读上游 body 边写下游，并处理 flush、背压、写超时和客户端 `ctx.Done()`

| 能力 | 典型延迟 | Go 注意点 |
|------|----------|-----------|
| 图像理解 | 与模型、图片尺寸和 detail 相关 | 高 detail 通常增加 token/费用 |
| ASR | 与模型、音频长度、并发和实时模式相关 | 采样率/声道按来源与模型要求预处理 |
| TTS | 关注首字节与播放缓冲 | 流式可降低体感等待 |
| 实时语音 API | 双向流 | 独立 WebSocket，与文本 Chat 分离 |

## 生产场景

- **智能客服**：用户发截图 + 语音；ASR 失败时降级纯文字
- **工单质检**：录音 ASR → LLM 提取违规话术 → 结构化入库
- **会议助手**：长音频分片 ASR + 时间戳对齐

## 排查与工具

- 指标：`asr_duration_seconds`、`vision_request_bytes`、`tts_first_byte_ms`
- 抽样保存 **脱敏** 原文件用于 bad case（合规审批）
- `ffprobe` 检查客户端上传格式是否异常

## 架构取舍

| 方案 | 适用 |
|------|------|
| 云 API（OpenAI/Azure/阿里） | 快速上线 |
| 私有化 Whisper + vLLM 视觉 | 数据不出域 |
| 端侧 ASR + 云端 LLM | 降带宽，复杂度高 |

**何时不用端到端多模态**：仅需 OCR — 专用 OCR 更便宜更准。

## 深挖问答

1. **图片 base64 还是 URL？** → 由 provider 请求大小、隐私和网络路径决定；URL 使用 HTTPS、短 TTL、最小对象权限，并防止把内部任意 URL 暴露给 provider。
2. **实时语音和批处理 ASR？** → 实时用 WebSocket/专用 Realtime API；离线批处理用文件 API 更便宜。
3. **如何防恶意上传？** → 限制 MIME、尺寸、时长；病毒扫描；异步队列处理。
4. **和 [S-AI-01](./S-AI-01-llm-api-integration.md) 流式关系？** → 同一套 ctx 超时与连接池；TTS/Chat 共用 `http.Client` 要注意 body 类型不同。

## 反模式与事故

- **无限 multipart** → 内存 OOM
- 不经评估所有图片都用最高 detail → token、延迟和费用显著上升
- **ASR 结果直接执行 SQL** → 语音注入
- **TTS 全量缓存用户隐私对话** → 合规风险

## 代码示例

多模态请求体组装可与 `examples/senior/llmclient` 的 `Message` 扩展为 `Content []Part`；流式转发逻辑同 [S-AI-01](./S-AI-01-llm-api-integration.md) SSE 处理。

```go
type Part struct {
    Type     string // text | image_url | input_audio
    Text     string
    ImageURL string
    AudioURL string
}
```

## 延伸阅读

- [OpenAI Speech to text](https://platform.openai.com/docs/guides/speech-to-text)
- [OpenAI Text to speech](https://platform.openai.com/docs/guides/text-to-speech)
- [OpenAI Images & vision](https://platform.openai.com/docs/guides/images)
