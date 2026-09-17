---
layout: home

hero:
  name: TokenBox
  text: 把所有 Token，装进一个 Box
  tagline: 一站式 AI API 中转站，用一个 API Key 接入主流模型、Agent 与图片视频能力。
  actions:
    - theme: brand
      text: 快速开始
      link: /quickstart
    - theme: alt
      text: 配置 Agent
      link: /agents/
    - theme: ghost
      text: 打开控制台
      link: https://tokenbox.you

features:
  - icon: 📦
    title: 一个 Box，装下所有 Token
    details: 聚合主流模型提供商，统一 API 入口。无需为不同平台维护多套 Key 和代码。
  - icon: 🚀
    title: 新手三步上手
    details: 登录、创建 API Key、发起第一次请求。清晰的引导让第一次调用也能快速跑通。
  - icon: 🤖
    title: Agent 开箱即用
    details: 面向 Codex CLI 与 Claude Code 提供完整配置指南，开发者工具可以直接接入 TokenBox。
  - icon: ⚡
    title: 快速切换配置
    details: 配合 cc-switch 工具，在多个模型服务之间一键切换，减少手动改配置的麻烦。
  - icon: 🎨
    title: 图片与视频模型
    details: 覆盖图片生成、视频生成等场景，按 OpenAI 兼容格式调用，降低接入成本。
  - icon: 🛟
    title: 常见问题随时查
    details: 汇总认证、模型、额度与调用中的常见错误，遇到问题先来这里快速定位。
---

## TokenBox 是什么

TokenBox 是一个 AI API 中转服务。它把 OpenAI、Anthropic、Google 等多家模型提供商的接口收敛到同一个入口，你只需要保存一个 API Key，就能调用聊天、图片、视频、Embedding 等多种能力。

和传统自建代理不同，TokenBox 更关心**开发者第一次接入是否顺畅**：

- 兼容 OpenAI 调用格式，已有的 OpenAI SDK 代码通常只需替换 Base URL 和 API Key。
- 支持 Agent 工具与第三方客户端，方便日常开发和自动化工作流。
- 页面中可以直接查看模型、价格、额度和调用示例。

## 建议的阅读路径

1. 先看 [快速开始](/quickstart)，完成登录和 API Key 创建。
2. 如果要配置 Codex CLI 或 Claude Code，继续看 [Agent 配置](/agents/)。
3. 图片、视频等特殊能力参考 [模型调用](/models/image)。
4. 遇到报错，先在 [常见错误](/help) 中搜索关键词。
