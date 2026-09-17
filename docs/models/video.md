# 视频模型调用

TokenBox 支持文生视频、图生视频等异步任务。视频生成通常需要先提交任务，再轮询任务状态，最后获取视频地址。

详细的上游参数和模型能力可参考飞书文档：

[视频模型接入文档](https://tcn5lhyjit4a.feishu.cn/wiki/D2vgwBDhMirM24k19UMcGv5mnGb)

## 调用前准备

1. 在 [API Keys](https://tokenbox.you/keys) 页面创建 API Key。
2. Base URL 使用：

```text
https://tokenbox.you/v1
```

3. TokenBox 主要提供两类入口：

| 接口 | 用途 |
| --- | --- |
| `POST /v1/video/generations` | TokenBox 通用异步视频任务 |
| `GET /v1/video/generations/:task_id` | 查询任务状态与结果 |
| `POST /v1/videos` | OpenAI 兼容的视频任务接口 |
| `GET /v1/videos/:task_id` | OpenAI 兼容的任务查询接口 |

## 整体流程

```text
提交任务 -> 获取 task_id -> 定时查询状态 -> 成功或失败
```

典型状态包括：

- `queued`
- `processing`
- `succeeded`
- `failed`

具体状态字段和返回格式可能因接口不同而略有差异。

## 文生视频示例

下面示例以 TokenBox 通用接口为例。模型名、分辨率、时长都需要以控制台或飞书文档中的可用配置为准。

### cURL

```bash
curl https://tokenbox.you/v1/video/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKENBOX_API_KEY" \
  -d '{
    "model": "你的视频模型名称",
    "prompt": "一只卡通小机器人在木箱旁挥手",
    "duration": 5,
    "resolution": "720p"
  }'
```

提交成功后记录返回的 `task_id`：

```json
{
  "task_id": "task_xxxxxxxx",
  "status": "queued"
}
```

查询任务：

```bash
curl https://tokenbox.you/v1/video/generations/task_xxxxxxxx \
  -H "Authorization: Bearer $TOKENBOX_API_KEY"
```

任务成功时，返回数据中通常包含视频地址或 `result_url`，例如：

```json
{
  "code": "success",
  "data": {
    "task_id": "task_xxxxxxxx",
    "status": "SUCCESS",
    "result_url": "https://..."
  }
}
```

## Python

```python
import os
import time
import requests

base_url = "https://tokenbox.you/v1"
headers = {
    "Authorization": f"Bearer {os.getenv('TOKENBOX_API_KEY')}",
    "Content-Type": "application/json",
}

submit = requests.post(
    f"{base_url}/video/generations",
    headers=headers,
    json={
        "model": "你的视频模型名称",
        "prompt": "一只卡通小机器人在木箱旁挥手",
        "duration": 5,
        "resolution": "720p",
    },
    timeout=30,
)
submit.raise_for_status()
task_id = submit.json()["task_id"]

while True:
    result = requests.get(
        f"{base_url}/video/generations/{task_id}",
        headers=headers,
        timeout=30,
    )
    result.raise_for_status()
    data = result.json().get("data", {})

    status = data.get("status", "")
    if status in {"SUCCESS", "succeeded", "completed"}:
        print("视频地址:", data.get("result_url") or data.get("url"))
        break
    if status in {"FAILURE", "failed"}:
        raise SystemExit(f"任务失败: {data}")
    time.sleep(3)
```

## Node.js

```javascript
const baseUrl = 'https://tokenbox.you/v1'
const headers = {
  Authorization: `Bearer ${process.env.TOKENBOX_API_KEY}`,
  'Content-Type': 'application/json'
}

const submit = await fetch(`${baseUrl}/video/generations`, {
  method: 'POST',
  headers,
  body: JSON.stringify({
    model: '你的视频模型名称',
    prompt: '一只卡通小机器人在木箱旁挥手',
    duration: 5,
    resolution: '720p'
  })
})
const submitted = await submit.json()
const taskId = submitted.task_id

while (true) {
  const response = await fetch(
    `${baseUrl}/video/generations/${taskId}`,
    { headers }
  )
  const payload = await response.json()
  const data = payload.data ?? payload

  if (['SUCCESS', 'succeeded', 'completed'].includes(data.status)) {
    console.log('视频地址:', data.result_url || data.url)
    break
  }
  if (['FAILURE', 'failed'].includes(data.status)) {
    throw new Error(`任务失败: ${JSON.stringify(data)}`)
  }
  await new Promise((resolve) => setTimeout(resolve, 3000))
}
```

## 图生视频

图生视频通常额外传入首帧图片。可以尝试使用 `image` 或 `images` 字段：

```bash
curl https://tokenbox.you/v1/video/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKENBOX_API_KEY" \
  -d '{
    "model": "你的图生视频模型名称",
    "prompt": "让画面中的角色自然微笑并挥手",
    "image": "https://example.com/first-frame.png",
    "duration": 5,
    "resolution": "720p"
  }'
```

不同供应商对图生视频字段的命名可能不同。如果 `image` 不生效，请对照飞书文档使用 `images`、`content` 或 `input_reference` 等字段。

## 获取视频内容

任务成功后的视频地址可能是一个远程 URL。如果返回的是 TokenBox 代理地址，也可以通过：

```text
GET /v1/videos/:task_id/content
```

下载视频内容。请以实际返回字段和平台配置为准。

## 常见问题

- 长时间处于 `queued`：可能是上游排队，稍后继续轮询。
- 状态变为 `failed`：查看返回中的 `fail_reason` 或 `error`。
- 提示参数错误：确认 `model`、`duration`、`resolution` 是否符合当前模型限制。
- 提示额度不足：确认 API Key 对应账号有足够额度。
