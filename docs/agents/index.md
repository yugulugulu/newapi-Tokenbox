# Agent 配置指南

TokenBox 可以用于 Codex CLI、Claude Code 等 Agent 工具。这类工具通常会自动执行命令、读取文件、调用模型，因此建议单独创建一个 API Key，并给 Key 起一个明确名称，例如 `codex-cli` 或 `claude-code`。

## 配置前准备

无论使用哪一款 Agent，都建议先完成：

1. 登录 [TokenBox](https://tokenbox.you)。
2. 在 [API Keys](https://tokenbox.you/keys) 中创建 Key。
3. 安装 Node.js，用于通过 npm 安装命令行工具。

## 选择工具

| 工具 | 适合场景 | 配置入口 |
| --- | --- | --- |
| Codex CLI | 代码生成、自动化执行、多文件修改 | [Codex CLI 配置](/agents/codex) |
| Claude Code | 交互式编码、Claude 模型生态 | [Claude Code 配置](/agents/claude-code) |

如果你还没有安装 Node.js，请先查看 [安装 Node.js](/agents/nodejs)。

## 为什么建议单独创建 Key

- 方便单独撤销：某个 Agent 泄露或不再使用时，不会影响其他业务。
- 方便查看消耗：按 Key 名称区分不同工具的调用量。
- 权限更清晰：不需要给 Agent 分配与 Web 控制台相同的敏感权限。

## 下一步

- [安装 Node.js](/agents/nodejs)
- [配置 Codex CLI](/agents/codex)
- [配置 Claude Code](/agents/claude-code)
