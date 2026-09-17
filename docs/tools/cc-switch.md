# cc-switch 快速配置工具

cc-switch 是一个用于快速切换 Claude Code / Codex 等工具服务商配置的桌面工具。如果你经常在多个模型网关之间切换，可以用它减少手动编辑配置文件的麻烦。

## 下载

前往 [cc-switch Releases](https://github.com/farion1231/cc-switch/releases) 下载对应系统的安装包：

- macOS 通常选择 `.dmg` 文件。
- Windows 通常选择 `.exe` 或 `.msi` 文件。
- 如果不确定，优先选择带有 `latest` 标记的版本。

## 安装

### macOS

1. 打开下载的 `.dmg` 文件。
2. 将应用拖入 `Applications`。
3. 首次打开时，如果系统提示未验证开发者，请在「系统设置 → 隐私与安全性」中允许打开。

### Windows

1. 双击下载的 `.exe` 文件。
2. 按安装向导完成安装。
3. 首次启动时，如果 SmartScreen 提示风险，请确认安装包来源后再选择运行。

## 添加 TokenBox

打开 cc-switch 后，新增一个供应商配置，并按 TokenBox 填写：

| 配置项 | 推荐值 |
| --- | --- |
| 供应商名称 | `TokenBox` |
| Base URL | `https://tokenbox.you` |
| API Key | 在 [TokenBox API Keys](https://tokenbox.you/keys) 创建后粘贴 |
| 默认模型 | 从 [模型价格页](https://tokenbox.you/pricing) 选择实际可用的模型 |

不同版本的 cc-switch 字段名称可能略有差异。保存后，将 TokenBox 设为当前使用的供应商即可。

## 验证切换结果

切换后，在终端运行：

```bash
claude --version
```

然后发起一个最简单的对话，确认能正常返回。若报错，先回到 cc-switch 检查 Base URL、API Key 和默认模型是否填写正确。

::: tip 为什么用它
cc-switch 适合本地配置多套服务商、经常来回切换的开发场景。TokenBox 与 OpenAI 兼容的接口，可以作为其中一个供应商快速接入。
:::
