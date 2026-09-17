# 新手快速开始

本文按「登录 TokenBox → 选择分组 → 创建 API Key → 发起第一次请求」的顺序，带你完成第一次调用。

## 登录 TokenBox

打开 [TokenBox 控制台](https://tokenbox.you)，注册或登录账号。

新用户可以先加入社群联系客服领取体验额度。领取后建议先熟悉控制台首页和左侧导航，再继续创建 API Key。

## 了解分组

TokenBox 通过「分组」来管理模型和渠道。不同分组可能对应不同的模型范围或计费倍率。

- 登录后，在模型或价格页面通常能看到当前账号可用的分组。
- 创建 API Key 时，可以选择继承默认分组，也可以指定一个分组。
- 如果你刚接触，保持默认分组即可；等熟悉模型和计费后，再按需切换。

::: tip 什么时候需要关心分组
如果你调用某个模型时提示模型不存在、无权限或分组不可用，先检查当前 API Key 是否选择了正确的分组。
:::

## 创建 API Key

1. 登录控制台后进入 [API Keys](https://tokenbox.you/keys) 页面。
2. 点击创建或新增 API Key。
3. 填写名称，方便识别用途，例如 `本地开发` 或 `Claude Code`。
4. 根据需求选择分组，默认使用继承配置即可。
5. 保存后立即复制 API Key。

::: warning 只显示一次
API Key 通常在创建完成后只完整展示一次。请立即保存到安全的密码管理器中，不要提交到 Git 仓库或公开分享。
:::

## 发起第一次请求

TokenBox 兼容 OpenAI 调用格式。下面的示例使用 `https://tokenbox.you/v1` 作为 Base URL。

### cURL

```bash
curl https://tokenbox.you/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKENBOX_API_KEY" \
  -d '{
    "model": "gpt-5.6-terra",
    "messages": [
      {"role": "user", "content": "你好，TokenBox！"}
    ]
  }'
```

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="TOKENBOX_API_KEY",
    base_url="https://tokenbox.you/v1"
)

response = client.chat.completions.create(
    model="gpt-5.6-terra",
    messages=[
        {"role": "user", "content": "你好，TokenBox！"}
    ]
)

print(response.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from 'openai'

const client = new OpenAI({
  apiKey: process.env.TOKENBOX_API_KEY,
  baseURL: 'https://tokenbox.you/v1'
})

const response = await client.chat.completions.create({
  model: 'gpt-5.6-terra',
  messages: [
    { role: 'user', content: '你好，TokenBox！' }
  ]
})

console.log(response.choices[0].message.content)
```

## 查看模型与价格

需要确认某个模型是否可用、如何计费时，打开 [模型价格页](https://tokenbox.you/pricing)。

模型详情中通常包含：

- 模型名称与支持的分组。
- 每百万 Token 或每张图片、每秒视频的计费方式。
- 可以直接复制的 cURL、Python、Node.js 调用示例。

## 下一步

- 配置 Codex CLI：查看 [Codex CLI 配置指南](/agents/codex)。
- 配置 Claude Code：查看 [Claude Code 配置指南](/agents/claude-code)。
- 图片模型：查看 [图片模型调用](/models/image)。
- 视频模型：查看 [视频模型调用](/models/video)。
