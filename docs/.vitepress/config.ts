import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'TokenBox 文档',
  description: '把所有 Token，装进一个 Box',
  lang: 'zh-CN',
  base: '/docs/',
  lastUpdated: true,
  cleanUrls: true,

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/docs/logo.png' }]
  ],

  themeConfig: {
    logo: '/logo.png',

    nav: [
      { text: '首页', link: '/' },
      { text: '快速开始', link: '/quickstart' },
      { text: 'Agent 配置', link: '/agents/' },
      { text: '模型调用', link: '/models/image' },
      { text: '帮助', link: '/help' },
      { text: '控制台', link: 'https://tokenbox.you' }
    ],

    sidebar: [
      {
        text: '快速开始',
        items: [
          { text: '什么是 TokenBox', link: '/' },
          { text: '登录与领取额度', link: '/quickstart#登录-tokenbox' },
          { text: '创建 API Key', link: '/quickstart#创建-api-key' },
          { text: '发起第一次请求', link: '/quickstart#发起第一次请求' }
        ]
      },
      {
        text: 'Agent 配置',
        items: [
          { text: '配置概览', link: '/agents/' },
          { text: '安装 Node.js', link: '/agents/nodejs' },
          { text: 'Codex CLI', link: '/agents/codex' },
          { text: 'Claude Code', link: '/agents/claude-code' }
        ]
      },
      {
        text: '快速配置工具',
        items: [
          { text: 'cc-switch 下载与使用', link: '/tools/cc-switch' }
        ]
      },
      {
        text: '模型调用',
        items: [
          { text: '图片模型', link: '/models/image' },
          { text: '视频模型', link: '/models/video' }
        ]
      },
      {
        text: '帮助',
        items: [
          { text: '常见错误', link: '/help' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/yugulugulu/newapi-Tokenbox' }
    ],

    footer: {
      message: 'TokenBox · 把所有 Token，装进一个 Box',
      copyright: 'Copyright © 2026 TokenBox'
    },

    search: {
      provider: 'local'
    },

    outline: {
      level: [2, 3],
      label: '本页目录'
    },

    docFooter: {
      prev: '上一篇',
      next: '下一篇'
    },

    lastUpdated: {
      text: '最后更新',
      formatOptions: {
        dateStyle: 'short',
        timeStyle: 'short'
      }
    }
  }
})
