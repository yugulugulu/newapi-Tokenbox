# Claude Code 配置

本节介绍如何安装 Claude Code，并让它使用 TokenBox 作为 Anthropic 兼容接口。

## 安装 Claude Code

macOS 和 Windows 都可以通过 npm 安装：

```bash
npm install -g @anthropic-ai/claude-code
```

安装完成后检查版本：

```bash
claude --version
```

## 准备 API Key

在 [TokenBox API Keys](https://tokenbox.you/keys) 页面创建一个 Key。

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

## 配置 TokenBox 端点

Claude Code 使用 Anthropic 格式，需要指定兼容的 Base URL 和认证 Token。

::: code-group

```bash [macOS / zsh]
export ANTHROPIC_BASE_URL="https://tokenbox.you"
export ANTHROPIC_AUTH_TOKEN="$TOKENBOX_API_KEY"
export ANTHROPIC_MODEL="claude-opus-4-8"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

```powershell [Windows / PowerShell]
$env:ANTHROPIC_BASE_URL="https://tokenbox.you"
$env:ANTHROPIC_AUTH_TOKEN=$env:TOKENBOX_API_KEY
$env:ANTHROPIC_MODEL="claude-opus-4-8"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

:::

`ANTHROPIC_MODEL` 可以替换为 [TokenBox 模型价格页](https://tokenbox.you/pricing) 中实际可用的 Claude 模型名称。

## 验证配置

```bash
claude -p "回复 OK，不要执行任何命令"
```

如果能正常返回，说明 Claude Code 已通过 TokenBox 调用模型。

::: warning 注意
如果请求一直失败，优先确认 `ANTHROPIC_BASE_URL` 没有额外追加 `/v1`，并检查 API Key 和模型名称是否有效。
:::
