# 图片模型调用

TokenBox 提供 OpenAI 兼容的图片生成接口。常见模型包括 `gpt-image-2`、`gpt-image-2-edit` 等，具体可用模型、图片尺寸、计费方式请以 [TokenBox 模型价格页](https://tokenbox.you/pricing) 为准。

## 调用前准备

1. 在 [API Keys](https://tokenbox.you/keys) 页面创建 API Key。
2. Base URL 使用：

```text
https://tokenbox.you/v1
```

3. 生成图片使用：

```text
POST /v1/images/generations
```

## cURL

```bash
curl https://tokenbox.you/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKENBOX_API_KEY" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只可爱的卡通小机器人，站在木箱旁边，暖色背景",
    "n": 1,
    "size": "1024x1024"
  }'
```

成功后，返回内容通常包含：

```json
{
  "data": [
    {
      "url": "https://..."
    }
  ]
}
```

如果希望返回 Base64，可以追加：

```json
{
  "response_format": "b64_json"
}
```

## Python

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.getenv("TOKENBOX_API_KEY"),
    base_url="https://tokenbox.you/v1",
)

response = client.images.generate(
    model="gpt-image-2",
    prompt="一只可爱的卡通小机器人，站在木箱旁边，暖色背景",
    n=1,
    size="1024x1024",
)

for image in response.data:
    print(image.url or image.b64_json[:60])
```

## Node.js

```javascript
import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: process.env.TOKENBOX_API_KEY,
  baseURL: 'https://tokenbox.you/v1'
})

const response = await client.images.generate({
  model: 'gpt-image-2',
  prompt: '一只可爱的卡通小机器人，站在木箱旁边，暖色背景',
  n: 1,
  size: '1024x1024'
})

for (const image of response.data) {
  console.log(image.url || image.b64_json?.slice(0, 60))
}
```

## 常用参数

| 参数 | 说明 |
| --- | --- |
| `model` | 模型名称，例如 `gpt-image-2`，以控制台可调用模型为准 |
| `prompt` | 生成图片的描述文本 |
| `n` | 生成数量，通常为 `1`，具体上限以 TokenBox 配置为准 |
| `size` | 图片尺寸，例如 `1024x1024`、`1536x1024`、`1024x1536` |
| `quality` | 质量参数，具体取值以模型能力为准 |
| `response_format` | `url` 或 `b64_json` |

::: warning 模型与价格会变化
图片模型名称、支持尺寸和计费规则可能随渠道配置调整。接入前先在 [模型价格页](https://tokenbox.you/pricing) 确认，避免硬编码不可用的模型或尺寸。
:::

## 下载返回图片

当返回 `url` 时，可以直接下载：

```bash
curl -L "返回的图片URL" -o tokenbox-image.png
```

当返回 `b64_json` 时，需要先将 Base64 解码为图片文件：

```python
import base64

with open("tokenbox-image.png", "wb") as f:
    f.write(base64.b64decode(response.data[0].b64_json))
```

## 常见问题

- 返回 `401`：检查 `Authorization: Bearer $TOKENBOX_API_KEY` 是否遗漏或 Key 是否正确。
- 返回 `404` 或模型不存在：确认模型名称和 API Key 所属分组可用。
- 返回尺寸错误：确认 `size` 是当前模型支持的组合。
- 返回额度不足：到控制台钱包或额度页面确认剩余额度。
