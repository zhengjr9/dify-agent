# dify-agent API 文档

本项目只有一个二进制入口 `cmd/server`，通过 `--a2a` flag 同时启动 Proxy Server 和 A2A Server。

---

## 一、启动方式

### 仅启动 Proxy Server（默认）

```bash
go run ./cmd/server/main.go \
  --dify-base-url https://your-dify-host/v1/chat-messages \
  --listen-addr :8080 \
  --default-user your-username \
  --request-timeout 120s
```

### 同时启动 Proxy + A2A Server

```bash
go run ./cmd/server/main.go \
  --dify-base-url https://your-dify-host/v1/chat-messages \
  --dify-api-key app-xxxxxxxxxxxxxxxxxxxx \
  --listen-addr :8080 \
  --default-user your-username \
  --a2a \
  --a2a-port 8000 \
  --agent-name "my-agent" \
  --agent-desc "A Dify-backed intelligent assistant"
```

### 环境变量方式

```bash
DIFY_BASE_URL=https://your-dify-host/v1/chat-messages \
DIFY_API_KEY=app-xxxxxxxxxxxxxxxxxxxx \
LISTEN_ADDR=:8080 \
DEFAULT_USER=your-username \
A2A_ENABLED=true \
A2A_PORT=8000 \
AGENT_NAME=my-agent \
AGENT_DESC="A Dify-backed intelligent assistant" \
go run ./cmd/server/main.go
```

### 全部启动参数

| Flag | 环境变量 | 默认值 | 说明 |
|------|----------|--------|------|
| `--dify-base-url` | `DIFY_BASE_URL` | `http://localhost` | Dify 端点（完整 URL 或 base URL）|
| `--dify-api-key` | `DIFY_API_KEY` | *(空)* | Dify API Key（启用 A2A 时必填）|
| `--listen-addr` | `LISTEN_ADDR` | `:8080` | Proxy 监听地址 |
| `--default-user` | `DEFAULT_USER` | `dify-agent` | 传给 Dify 的 user 字段及 AIGC-USER 头 |
| `--request-timeout` | `REQUEST_TIMEOUT` | `120s` | Dify 请求超时 |
| `--a2a` | `A2A_ENABLED` | `false` | 是否同时启动 A2A Server |
| `--a2a-port` | `A2A_PORT` | `8000` | A2A Server 监听端口 |
| `--agent-name` | `AGENT_NAME` | `dify-agent` | A2A AgentCard 名称 |
| `--agent-desc` | `AGENT_DESC` | `Dify-backed agent...` | A2A AgentCard 描述 |

---

## 二、Proxy Server（`:8080`）

Proxy Server 接收 OpenAI / Anthropic / Gemini 格式请求，转发给 Dify。

> API Key 由调用方通过 `Authorization: Bearer <key>` 透传，不在服务端配置。

---

### 2.1 OpenAI 兼容接口

#### POST /v1/chat/completions

**Blocking 模式**

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dify",
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": false
  }'
```

**响应：**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "dify",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！我是基于 Dify 的智能助手，可以回答你的问题。"
      },
      "finish_reason": "stop"
    }
  ]
}
```

**Streaming 模式**

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dify",
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": true
  }'
```

**SSE 响应：**
```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1700000000,"model":"dify","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1700000000,"model":"dify","choices":[{"index":0,"delta":{"content":"！"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1700000000,"model":"dify","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

---

### 2.2 Anthropic 兼容接口

#### POST /v1/messages

**Blocking 模式**

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": false
  }'
```

**响应：**
```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "你好！我是基于 Dify 的智能助手，可以回答你的问题。"}
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 10,
    "output_tokens": 200
  }
}
```

**Streaming 模式**

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好，请介绍一下你自己"}
    ],
    "stream": true
  }'
```

**SSE 响应：**
```
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"！"}}

event: message_stop
data: {"type":"message_stop"}
```

---

### 2.3 Gemini 兼容接口

#### POST /v1beta/models/{model}:generateContent

**Blocking 模式**

```bash
curl -X POST "http://localhost:8080/v1beta/models/gemini-1.5-pro:generateContent" \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "你好，请介绍一下你自己"}]
      }
    ]
  }'
```

**响应：**
```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [{"text": "你好！我是基于 Dify 的智能助手，可以回答你的问题。"}]
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 10,
    "candidatesTokenCount": 200,
    "totalTokenCount": 210
  }
}
```

#### POST /v1beta/models/{model}:streamGenerateContent

**Streaming 模式**

```bash
curl -X POST "http://localhost:8080/v1beta/models/gemini-1.5-pro:streamGenerateContent" \
  -H "Authorization: Bearer app-xxxxxxxxxxxxxxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "你好，请介绍一下你自己"}]
      }
    ]
  }'
```

**SSE 响应：**
```
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"你好"}]},"finishReason":"","index":0}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"！"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":200,"totalTokenCount":210}}
```

---

## 三、A2A Server（`:8000`）

启动时加 `--a2a` 即可在独立端口启动 A2A Server，遵循 A2A 协议（JSON-RPC 2.0）。

---

### 3.1 获取 AgentCard

#### GET /.well-known/agent-card.json

```bash
curl http://localhost:8000/.well-known/agent-card.json
```

**响应：**
```json
{
  "name": "my-agent",
  "description": "A Dify-backed intelligent assistant",
  "url": "http://localhost:8000/",
  "version": "1.0.0",
  "capabilities": {
    "streaming": true
  }
}
```

---

### 3.2 发送消息（同步）

#### POST /

```bash
curl -X POST http://localhost:8000/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [
          {"kind": "text", "text": "你好，请介绍一下你自己"}
        ],
        "messageId": "msg-001"
      }
    }
  }'
```

**响应：**
```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "result": {
    "kind": "task",
    "id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "contextId": "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy",
    "status": {
      "state": "completed",
      "timestamp": "2024-01-01T00:00:00.000000+08:00"
    },
    "artifacts": [
      {
        "artifactId": "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
        "parts": [
          {
            "kind": "text",
            "text": "你好！我是基于 Dify 的智能助手..."
          }
        ]
      }
    ],
    "history": [
      {
        "kind": "message",
        "messageId": "msg-001",
        "role": "user",
        "parts": [{"kind": "text", "text": "你好，请介绍一下你自己"}]
      }
    ]
  }
}
```

---

### 3.3 流式接收

#### POST /（method: message/stream）

```bash
curl -X POST http://localhost:8000/ \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": "2",
    "method": "message/stream",
    "params": {
      "message": {
        "role": "user",
        "parts": [
          {"kind": "text", "text": "请详细说明一下"}
        ],
        "messageId": "msg-002"
      }
    }
  }'
```

**SSE 响应（逐 token 推送）：**
```
data: {"jsonrpc":"2.0","id":"2","result":{"kind":"task","id":"...","status":{"state":"working"},"artifacts":[{"parts":[{"kind":"text","text":"好的","metadata":{"adk_partial":true}}]}]}}

data: {"jsonrpc":"2.0","id":"2","result":{"kind":"task","id":"...","status":{"state":"working"},"artifacts":[{"parts":[{"kind":"text","text":"，","metadata":{"adk_partial":true}}]}]}}

data: {"jsonrpc":"2.0","id":"2","result":{"kind":"task","id":"...","status":{"state":"completed"},"artifacts":[{"parts":[{"kind":"text","text":"好的，详细说明如下..."}]}]}}
```

---

### 3.4 多轮对话

使用第一次响应中的 `contextId` 保持上下文：

```bash
# 第一轮
curl -X POST http://localhost:8000/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"kind": "text", "text": "你好"}],
        "messageId": "msg-001"
      }
    }
  }'

# 第二轮（带上 contextId）
curl -X POST http://localhost:8000/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "2",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "parts": [{"kind": "text", "text": "继续上面的话题"}],
        "messageId": "msg-002",
        "contextId": "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy"
      }
    }
  }'
```

---

## 四、错误响应

### Proxy Server

```json
{"error": {"code": 401, "message": "missing or invalid Authorization header"}}
{"error": {"code": 400, "message": "invalid request body"}}
{"error": {"code": 502, "message": "dify 404: ..."}}
```

### A2A Server（JSON-RPC 错误）

```json
{
  "jsonrpc": "2.0",
  "id": "1",
  "error": {
    "code": -32603,
    "message": "dify streaming request failed: dify 401: ..."
  }
}
```

---

## 五、Dify 请求透传说明

两个服务最终都向 Dify 发送如下请求：

```http
POST /v1/chat-messages
Authorization: Bearer <api-key>
AIGC-USER: <user>
Content-Type: application/json

{
  "inputs": {},
  "query": "<用户消息>",
  "response_mode": "streaming" | "blocking",
  "conversation_id": "",
  "user": "<user>",
  "files": []
}
```

| 字段 | Proxy Server | A2A Server |
|------|-------------|------------|
| `api-key` | 调用方 `Authorization` 头透传 | `--dify-api-key` 配置 |
| `user` / `AIGC-USER` | `--default-user` | `--default-user` |
