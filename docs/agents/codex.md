# Codex CLI 配置

本节介绍如何安装 Codex CLI，并让它使用 TokenBox 作为模型服务。

## 安装 Codex CLI

macOS 和 Windows 都可以通过 npm 安装：

```bash
npm install -g @openai/codex
```

安装完成后检查版本：

```bash
codex --version
```

## 准备 API Key

在 [TokenBox API Keys](https://tokenbox.you/keys) 页面创建一个 Key，并保存到本地环境变量。

### macOS / zsh

把下面内容追加到 `~/.zshrc`：

```bash
export TOKENBOX_API_KEY="sk-..."
```

然后执行：

```bash
source ~/.zshrc
```

### Windows / PowerShell

```powershell
[Environment]::SetEnvironmentVariable("TOKENBOX_API_KEY", "sk-...", "User")
```

设置后重新打开 PowerShell。

## 创建配置文件

在用户目录创建 `~/.codex/config.toml`：

::: code-group

```toml [~/.codex/config.toml]
model = "gpt-5.6-terra"
model_provider = "tokenbox"

[model_providers.tokenbox]
name = "TokenBox"
base_url = "https://tokenbox.you/v1"
wire_api = "responses"
env_key = "TOKENBOX_API_KEY"
```

:::

其中 `model` 可以替换为 [TokenBox 模型价格页](https://tokenbox.you/pricing) 中实际可用的模型名称。

## 验证配置

```bash
codex "回复 OK，不要执行任何命令"
```

如果能正常返回，说明 Codex CLI 已成功通过 TokenBox 调用模型。

::: warning 注意
如果模型名称不存在、分组不匹配或 Key 没有额度，会返回 404、401 或额度不足等错误。优先检查 API Key、模型名称和额度。
:::

## 常见问题

- 如果配置文件不被识别，先确认路径是否正确，或使用 `codex -c model_provider=tokenbox` 临时覆盖。
- 如果使用 `/v1/responses` 报错，可以尝试把 `wire_api` 改为 `chat`。
- 如果终端一直要求登录，确认 `env_key` 指向的环境变量已生效。
