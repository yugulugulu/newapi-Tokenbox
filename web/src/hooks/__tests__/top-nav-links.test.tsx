/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Window } from 'happy-dom'

const domWindow = new Window()
for (const key of ['window', 'document', 'navigator', 'HTMLElement'] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act, createElement } = await import('react')
const { createRoot } = await import('react-dom/client')
const i18next = (await import('i18next')).default
const { initReactI18next } = await import('react-i18next')
const { useTopNavLinks } = await import('../use-top-nav-links')

await i18next.use(initReactI18next).init({
  lng: 'zh',
  resources: {
    zh: {
      translation: {
        Docs: '文档',
        About: '关于',
        'Image Studio': '生图工作台',
        'Card Store': '卡网',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  domWindow.happyDOM.abort()
})

describe('top navigation links', () => {
  test('shows the two external links after docs and omits about when enabled', async () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(['status'], {
      HeaderNavModules: JSON.stringify({ about: true }),
      docs_link: 'https://docs.example.com/',
    })
    const container = document.createElement('div')
    const root = createRoot(container)

    function Links() {
      const links = useTopNavLinks()
      return createElement(
        'nav',
        null,
        links.map((link) =>
          createElement(
            'a',
            { key: link.href, href: link.href, 'data-external': link.external },
            link.title
          )
        )
      )
    }

    try {
      await act(async () => {
        root.render(
          createElement(
            QueryClientProvider,
            { client: queryClient },
            createElement(Links)
          )
        )
      })
      const links = [...container.querySelectorAll('a')]
      assert.deepEqual(
        links.slice(-3).map((link) => link.textContent),
        ['文档', '生图工作台', '卡网']
      )
      assert.deepEqual(
        links.slice(-3).map((link) => link.href),
        [
          'https://docs.example.com/',
          'https://image.tokenbox.you/',
          'https://jjbone.fun/',
        ]
      )
      assert.ok(
        links.slice(-3).every((link) => link.dataset.external === 'true')
      )
      assert.ok(!links.some((link) => link.textContent === '关于'))
    } finally {
      await act(async () => root.unmount())
      queryClient.clear()
    }
  })
})
