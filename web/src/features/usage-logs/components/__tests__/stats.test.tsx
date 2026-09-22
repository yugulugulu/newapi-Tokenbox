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

import { Window } from 'happy-dom'
import type React from 'react'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Usage: 'Usage',
        Refund: 'Refund',
        RPM: 'RPM',
        TPM: 'TPM',
      },
    },
  },
})

const { LogStatsBadges } = await import('../common-logs-stats')
const { formatLogQuota } = await import('@/lib/format')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type RenderedStats = {
  container: HTMLDivElement
  root: ReturnType<typeof createRoot>
}

async function renderStats(
  props: React.ComponentProps<typeof LogStatsBadges>
): Promise<RenderedStats> {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <LogStatsBadges {...props} />
      </I18nextProvider>
    )
  })

  return { container, root }
}

async function unmountStats(rendered: RenderedStats) {
  await act(async () => rendered.root.unmount())
  rendered.container.remove()
}

function badgeText(container: HTMLElement, label: string): string {
  const labelElement = [...container.querySelectorAll('span')].find(
    (element) => element.textContent === label
  )
  assert.ok(labelElement?.parentElement)
  return labelElement.parentElement.textContent ?? ''
}

describe('usage log statistics', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the refund total separately from usage', async () => {
    const rendered = await renderStats({
      stats: { quota: 12500, refund_quota: 3750, rpm: 2, tpm: 45 },
      sensitiveVisible: true,
    })

    assert.equal(
      badgeText(rendered.container, 'Refund').includes(formatLogQuota(3750)),
      true
    )
    assert.equal(
      badgeText(rendered.container, 'Usage').includes(formatLogQuota(12500)),
      true
    )

    await unmountStats(rendered)
  })

  test('shows zero when an older response omits the refund total', async () => {
    const rendered = await renderStats({
      stats: { quota: 12500, rpm: 2, tpm: 45 },
      sensitiveVisible: true,
    })

    assert.equal(
      badgeText(rendered.container, 'Refund').includes(formatLogQuota(0)),
      true
    )

    await unmountStats(rendered)
  })

  test('masks both usage and refund totals when sensitive values are hidden', async () => {
    const rendered = await renderStats({
      stats: { quota: 12500, refund_quota: 3750, rpm: 2, tpm: 45 },
      sensitiveVisible: false,
    })

    assert.equal(badgeText(rendered.container, 'Usage').includes('••••'), true)
    assert.equal(badgeText(rendered.container, 'Refund').includes('••••'), true)
    assert.equal(badgeText(rendered.container, 'RPM').includes('2'), true)

    await unmountStats(rendered)
  })
})
