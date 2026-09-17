# API 文档

TokenBox 提供完全兼容 OpenAI 格式的 API 接口。

## 基础信息

**API 地址：** `https://api.tokenbox.you/v1`

**认证方式：** Bearer Token

```http
Authorization: Bearer YOUR_API_KEY
```

## 支持的模型

### 对话模型

| 提供商 | 模型 | 上下文长度 |
|--------|------|-----------|
| OpenAI | gpt-5.6-terra | 128K |
| OpenAI | gpt-4-turbo | 128K |
| OpenAI | gpt-3.5-turbo | 16K |
| Anthropic | claude-opus-4-8 | 200K |
| Anthropic | claude-3-opus | 200K |
| Google | gemini-1.5-pro | 2M |
| Google | gemini-1.5-flash | 1M |

### 图片生成模型

| 提供商 | 模型 | 分辨率 |
|--------|------|--------|
| OpenAI | dall-e-3 | 1024x1024, 1792x1024, 1024x1792 |
| OpenAI | dall-e-2 | 1024x1024, 512x512, 256x256 |

## 快速开始

查看 [快速开始指南](/quickstart) 了解如何使用 API。


- 请求速率：每分钟最多 60 次（可联系升级）
- 最大请求体积：10 MB
- 超时时间：60 秒

## 最佳实践

### 1. 错误处理

```python
from openai import OpenAI, APIError

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://api.tokenbox.you/v1"
)

try:
    response = client.chat.completions.create(
        model="gpt-5.6-terra",
        messages=[{"role": "user", "content": "Hello"}]
    )
except APIError as e:
    print(f"API 错误: {e}")
```

### 2. 流式响应

```python
stream = client.chat.completions.create(
    model="gpt-5.6-terra",
    messages=[{"role": "user", "content": "写一首诗"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### 3. 设置超时

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://api.tokenbox.you/v1",
    timeout=30.0  # 30 秒超时
)
```
