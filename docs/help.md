# 常见错误与帮助

遇到调用失败时，先按下面顺序快速定位：

1. 确认 Base URL 是否正确：OpenAI 兼容接口使用 `https://tokenbox.you/v1`，Anthropic 兼容接口使用 `https://tokenbox.you`。
2. 确认请求头是否携带 `Authorization: Bearer $TOKENBOX_API_KEY`。
3. 确认模型名称与 [模型价格页](https://tokenbox.you/pricing) 中完全一致。
4. 确认 API Key 所属分组是否包含该模型。
5. 确认控制台钱包中仍有可用额度。

## 401 认证失败

**常见原因**

- 请求没有携带 `Authorization` 请求头。
- `Bearer` 后多写了空格、逗号或其他字符。
- API Key 已删除、已失效或被复制时遗漏字符。

**处理方式**

重新从 [API Keys](https://tokenbox.you/keys) 复制完整 Key，并检查环境变量：

```bash
echo "$TOKENBOX_API_KEY"
```

确认输出以 `sk-` 开头且没有多余引号。

## 404 模型不存在

**常见原因**

- 模型名拼写错误。
- API Key 使用了不包含该模型的分组。
- 该模型尚未在你当前分组中开启。

**处理方式**

打开 [模型价格页](https://tokenbox.you/pricing)，找到可调用模型名，直接复制到代码中。注意不要自行猜测模型版本号。

## 额度不足

错误信息中通常包含 `insufficient quota`、`余额不足`、`quota exceeded` 等关键词。

**处理方式**

1. 打开控制台钱包或额度页面，查看剩余额度。
2. 如果使用的是刚注册账号，先确认是否已领取新手额度。
3. 如果使用多个 API Key，确认当前 Key 对应的是同一个账号。

## 分组不可用

**常见表现**

- 提示 `分组不存在`、`分组未启用`、`当前分组不可用`。
- 同一个模型在其他 Key 上可用，但当前 Key 不可用。

**处理方式**

检查该 API Key 创建时选择的分组。必要时重新创建一个 Key，并显式选择你需要的分组。

## 400 参数错误

400 表示请求已到达服务端，但参数不合法。

先检查：

- JSON 是否完整，是否缺少必填字段。
- `messages` 是否为空，`content` 类型是否符合模型要求。
- 图片模型是否使用了正确的 `size`、`n` 等参数。
- 视频模型是否填写了当前模型支持的 `duration`、`resolution` 或图片字段。

建议先复制文档中的最小示例，跑通后再逐步加入自己的参数。

## 502 / 504 网关错误

502 和 504 通常表示 TokenBox 已收到请求，但上游模型服务没有及时返回。

**可以先做**

- 等待几秒后重试。
- 换一个模型测试，判断是个别模型还是整体服务问题。
- 检查请求是否过大，例如超长文本、超大图片或超长视频时长。

如果多个模型持续出现 502/504，建议联系客服并提供请求时间与模型名。

## Codex CLI / Claude Code 连接失败

### Codex CLI

检查 `~/.codex/config.toml`：

```toml
[model_providers.tokenbox]
name = "TokenBox"
base_url = "https://tokenbox.you/v1"
wire_api = "responses"
env_key = "TOKENBOX_API_KEY"
```

重点确认：

- `base_url` 末尾不要同时拼接其他版本路径。
- `env_key` 对应的环境变量已经生效。
- 模型名称在 TokenBox 中确实存在。

### Claude Code

检查当前终端环境变量：

```bash
echo "$ANTHROPIC_BASE_URL"
echo "$ANTHROPIC_AUTH_TOKEN"
echo "$ANTHROPIC_MODEL"
```

重点确认：

- `ANTHROPIC_BASE_URL` 为 `https://tokenbox.you`，不要额外追加 `/v1`。
- `ANTHROPIC_AUTH_TOKEN` 与 TokenBox API Key 一致。
- `ANTHROPIC_MODEL` 是可用的 Claude 模型名。

## 联系客服

如果以上步骤仍无法解决，请尽量提供以下信息，方便快速定位：

```text
1. 发生时间
2. 使用模型
3. 请求 Base URL
4. 使用的 API Key 名称或分组
5. 报错 HTTP 状态码
6. 完整错误信息
7. 最小复现代码或 curl 示例
```

请勿在反馈中直接粘贴完整 API Key。
