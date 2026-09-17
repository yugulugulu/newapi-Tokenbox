# 安装 Node.js

Codex CLI 和 Claude Code 都可以通过 npm 安装，因此先准备 Node.js 环境。

## macOS

推荐使用 Homebrew：

```bash
brew install node
```

安装后检查版本：

```bash
node -v
npm -v
```

如果还没有 Homebrew，请先访问 [brew.sh](https://brew.sh) 按提示安装。

## Windows

1. 打开 [Node.js 官网](https://nodejs.org/zh-cn)。
2. 下载 LTS 版本的 Windows Installer（`.msi`）。
3. 双击安装，一路保持默认选项。
4. 安装完成后打开 PowerShell，运行：

```powershell
node -v
npm -v
```

如果 PowerShell 提示找不到命令，先关闭并重新打开终端，或检查 Node.js 是否已加入系统 PATH。

## 检查 npm 是否可用

```bash
npm --version
```

能正常显示版本号，说明可以继续安装 Codex CLI 或 Claude Code。

::: tip 国内网络提示
如果 npm 下载速度较慢，可以临时使用镜像，例如 `npm config set registry https://registry.npmmirror.com`。
:::
