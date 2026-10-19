# 多模态请求、上传与语音恢复本地实战

对应 S-AI-08。Go 1.24.2+，标准库、loopback HTTP、静音 WAV 与固定转写结果，无密钥/麦克风/外部模型/ffmpeg。

```bash
go run ./examples/senior/mediaio/cmd/demo
go test -race -timeout 30s ./examples/senior/mediaio/...
```

CLI 串起：嵌套媒体消息 → 真实 multipart 上传校验 → 本地 ASR 请求 → 原始二进制 PCM 转发 → 重复分片合并 → 打断/迟到音频/新会话隔离。
预期 `passed=true`，`asr_calls=1`、`tts_calls=1`、`binary_bytes=6`、`merged_segments=1`、`truncate_ms=500`、`new_epoch=2`。

## 请求与上传边界

- `MarshalMessage([]Part)` 只序列化 Chat Completions 风格 user message：image_url={url,detail}；input_audio={data,format}，恰好一个 variant。
- 本例仅接受 HTTPS 图片引用（low/high/auto），不读取 URL、不验证对象归属、不产生签名 URL；可信宿主须先验证对象权限。它不是 Responses 的 input_image 格式。
- 音频 fixture 仅 WAV base64，实际探测内容必须是 PCM16 LE/单声道/24kHz；不把 AudioURL 当 input_audio，不把媒体塞进 llmclient.Message.Content。
- NewLocalClient 只接受字面量 loopback HTTP origin，禁用代理/重定向，不发送 key，使用完 Close；没有真实 provider 接入入口。
- Receive 在解析前 MaxBytesReader 限总请求，磁盘 multipart 临时文件在成功/失败/超时均清理；必须且仅有一个 file 字段，声明 MIME 与真实解码一致。
- 默认总请求256KiB、单文件128KiB、图像65536像素、音频3秒、上传/探测1秒；配置亦有硬上限。支持 PNG/JPEG 与限定 WAV，先像素检查再解码，WAV核对 RIFF/fmt/data/对齐/时长。
- 真实 HTTP 请求用 socket read deadline 约束慢上传；可注入探测器的合约是必须响应 ctx。Go 图像解码为有大小/像素边界的同步调用，不是可强制中止任意codec的沙箱。生产 ffprobe/转码需可信配置的子进程、CPU/内存/时长限制与隔离；本例没有恶意文件扫描、生产认证或对象存储权限验收。

## ASR 与分片

Transcribe 向本地 /asr 发 multipart，含实际 WAV、正确 MIME/boundary、model=fixture-asr-v1、language=zh、response_format=fixture-segments-v1；关闭 multipart writer 后才发送。
返回 JSON segments(start_ms,end_ms,text)，严格检查必填/非null/精确字段名、重复键（含转义同名）、范围、顺序、大小与错误状态，最多等2秒；失败不自动重试，不将未知生成/费用当免费。
这是本地请求/响应契约，不能只换URL就接入真实厂商；真实模型、流式能力和时间戳参数须按所选API验收。

Merge(totalMS,chunks) 按稳定 ID 识别相同结果重投；同ID不同内容拒绝。应用每片 OffsetMS，要求录音覆盖无缺口且时间戳合法；仅去重**相同全局起止时间+正文**的重叠段。
其他重叠文本冲突转人工核对，不能只按相同文字去重（重复措辞可能是真实发言）。这里不做强制对齐/说话人识别，也不提供持久重试账本或真实ASR准确率保证。

## PCM 转发与播放状态

本地 /tts 固定 audio/pcm，X-Audio-Format=pcm16le;rate=24000;channels=1，X-Audio-Bytes 声明偶数字节数（最多128KiB）。这些 X-* 是**本地长度夹具**，不是 OpenAI 通用字段。
RelayPCM 验证上游状态/格式后按原始字节转发，HTTP连接/读流总预算2秒，每次下游 Write 设置明确deadline并Flush；不支持deadline的 writer 拒绝。
不走文本 SSE。fixture 字节数与上游EOF检测截断/多余数据，清除继承的Content-Length，下游保持chunked并Flush，不能提前用Content-Length掩盖后续校验失败；长度吻合只证明本地传输，不证明语音语义完成或实际账单已确定。
开始输出后失败返回 RelayError{Started:true}，调用方必须中断响应（demo 用 http.ErrAbortHandler），不能追加 JSON 或拼接重试音频；所有路径都关闭上游并传播取消。

Voice 是**单个事件循环串行使用**的本地 WebSocket 风格控制/播放状态模型，不实现 WebSocket/WebRTC网络协议、设备播放器或真实服务会话。
- Append 绑定session/connection epoch/response/sequence，有界队列（96000字节）；生成结束与播放完毕分离。
- Played(session,response,epoch,n) 同样绑定会话/响应/epoch，拒绝旧播放器回调；计实际播放消耗字节，不能用收到字节数代替；Interrupt 清队列并返回匹配item的截断位置（已播放bytes/48，向下取整为毫秒）。
- 真实传输还须停止本地播放、取消在途生成、发送协议对应的truncate并核对ACK。本地示例在ACK前拒绝新生成；迟到音频、错误ACK、重复response ID均拒绝。
- Disconnect 不推断可恢复旧会话；只接受本句柄从未使用的新确认session（最多128个，耗尽后必须重新创建句柄并握手），提升epoch，绝不重播旧音频或工具。重启也须重新握手；没有持久恢复旧会话的声明。
- WebRTC媒体轨与控制通道、服务端自动截断应按具体API处理，不能直接照搬这里的WebSocket风格手动截断模型。

| 测试组 | 故障验证 |
|---|---|
| Wire* / WAV* | 嵌套schema/oneof、错误媒体类型/base64/外部端点、WAV大小/编码/声道/采样率/对齐/时长 |
| UploadValidAndTempCleanup / Multipart* | 正常上传、总大小/文件大小、MIME伪装、缺boundary/错误字段、探测失败/超时、取消、磁盘临时文件清理 |
| ImageDecodeAndPixelBudgets / SlowUpload* | 真PNG解码、像素超限/损坏媒体、真实慢上传读deadline |
| ASR* | 真实multipart请求字段/格式、非2xx/坏JSON/缺字段/null/重复键/大小写别名/越界时间戳/超大响应/取消；失败只请求一次 |
| RelayPCM* / RelayFailures* | 二进制保真、非2xx/错误MIME或音频格式/非法长度、截断/多余字节、失败不重试 |
| HTTPAbortMakesTruncatedAndExtraAudioObservableToClient | 真实HTTP下游在截断/多余音频时读流失败，不被提前Content-Length掩盖 |
| SlowDownstream* / RelayCancellation* | net.Pipe真实阻塞写超时、取消上游、无deadline writer拒绝 |
| Merge* | 重复分片、全局时间偏移/精确重叠去重、同ID冲突、缺口/缺片/时间越界/歧义重叠 |
| Generated* / Interrupt* / Disconnect* / VoiceLimits* / StalePlayback* | 生成/播放完成分离、已播放500ms截断、ACK丢失/错误、迟到音频、旧播放回调拒绝、历史session拒绝复用、新epoch、乱序/队列限额 |

成本/质量取舍应以具体模型、语料、计费与部署链路评估；本例不比较OCR准确率、云端费用或认证数据不出域。
