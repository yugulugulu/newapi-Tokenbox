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

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'KeyboardEvent',
  'PointerEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { CommissionDateRangeFields } =
  await import('../commission-date-range-fields')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Clear: 'Clear',
        'Start date': 'Start date',
        'End date': 'End date',
        'Pick a date': 'Pick a date',
        'Clear start date': 'Clear start date',
        'Clear end date': 'Clear end date',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function Harness() {
  const [startDate, setStartDate] = useState('2026-09-14')
  const [endDate, setEndDate] = useState('2026-09-30')

  return (
    <I18nextProvider i18n={i18n}>
      <CommissionDateRangeFields
        idPrefix='records'
        startDate={startDate}
        endDate={endDate}
        onStartDateChange={setStartDate}
        onEndDateChange={setEndDate}
      />
      <output data-testid='start-date'>{startDate}</output>
      <output data-testid='end-date'>{endDate}</output>
    </I18nextProvider>
  )
}

describe('commission date range clear action', () => {
  after(() => {
    domWindow.close()
  })

  test('clears the selected start date from inside the calendar popover', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => root.render(<Harness />))

    const startDateTrigger = container.querySelector<HTMLButtonElement>(
      '#records-start-date'
    )
    assert.ok(startDateTrigger)
    await act(async () => startDateTrigger.click())

    const clearButton = [...document.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === 'Clear'
    )
    assert.ok(clearButton)
    await act(async () => clearButton.click())

    assert.equal(
      container.querySelector('[data-testid="start-date"]')?.textContent,
      ''
    )
    assert.equal(
      container.querySelector('[data-testid="end-date"]')?.textContent,
      '2026-09-30'
    )

    await act(async () => root.unmount())
    container.remove()
  })
})
