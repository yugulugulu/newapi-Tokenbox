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

import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { Window } from 'happy-dom'

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
  'customElements',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
})

Object.defineProperty(globalThis, 'IntersectionObserver', {
  configurable: true,
  value: class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const i18next = (await import('i18next')).default
const { initReactI18next } = await import('react-i18next')

await i18next.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Go to Dashboard': 'Go to Dashboard',
        home: {
          tokenbox: {
            hero: {
              badge: 'One gateway',
              titleLine1: 'Open the box',
              titleLine2: 'Meet every model',
              description: 'A unified AI gateway.',
              newcomer: 'Claim newcomer benefits',
              pricing: 'View model pricing',
              docs: 'Configuration docs',
              models: 'Supported models',
            },
            status: {
              title: 'API status',
              operational: 'Operational',
              gateway: 'Gateway',
              models: 'Models',
            },
            onboarding: {
              title: 'Newcomer unboxing route',
              description: 'Make your first request in three steps.',
              createKey: 'Create an API KEY~',
              benefit: 'New user credit',
              benefitDescription: 'Join the community and claim $5.',
              step1Title: 'Create account',
              step1Description: 'Done in seconds',
              step2Title: 'Copy API Key',
              step2Description: 'Secure one-click copy',
              step3Title: 'Send first request',
              step3Description: 'Receive 200 OK',
            },
          },
        },
      },
    },
  },
})

const { TokenBoxHero } = await import('../sections/tokenbox-hero')
const { TokenBoxOnboarding } = await import('../sections/tokenbox-onboarding')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderHero(isAuthenticated: boolean) {
  const rootRoute = createRootRoute({
    component: () => (
      <TokenBoxHero
        docsUrl='https://docs.example.com/getting-started'
        isAuthenticated={isAuthenticated}
      />
    ),
  })
  const routes = ['pricing', 'sign-up', 'dashboard'].map((path) =>
    createRoute({
      getParentRoute: () => rootRoute,
      path,
      component: () => null,
    })
  )
  const router = createRouter({
    routeTree: rootRoute.addChildren(routes),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(<RouterProvider router={router} />)
    await router.load()
  })

  return { container, root }
}

async function unmountHero(rendered: Awaited<ReturnType<typeof renderHero>>) {
  await act(async () => rendered.root.unmount())
  rendered.container.remove()
}

async function renderOnboarding() {
  const rootRoute = createRootRoute({
    component: TokenBoxOnboarding,
  })
  const keysRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: 'keys',
    component: () => null,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([keysRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(<RouterProvider router={router} />)
    await router.load()
  })

  return { container, root }
}

function findLink(container: HTMLElement, label: string) {
  const link = [...container.querySelectorAll('a')].find((anchor) =>
    anchor.textContent?.includes(label)
  )
  assert.ok(link)
  return link
}

describe('TokenBox homepage actions', () => {
  after(() => {
    domWindow.close()
  })

  test('sends a new visitor to sign-up and exposes pricing and docs', async () => {
    const rendered = await renderHero(false)

    assert.equal(
      findLink(rendered.container, 'Claim newcomer benefits').getAttribute(
        'href'
      ),
      '/sign-up'
    )
    assert.equal(
      findLink(rendered.container, 'View model pricing').getAttribute('href'),
      '/pricing'
    )
    const docsLink = findLink(rendered.container, 'Configuration docs')
    assert.equal(
      docsLink.getAttribute('href'),
      'https://docs.example.com/getting-started'
    )
    assert.equal(docsLink.getAttribute('target'), '_blank')
    assert.equal(docsLink.getAttribute('rel'), 'noreferrer')
    assert.equal(rendered.container.textContent?.includes('API status'), false)
    const mascot = rendered.container.querySelector(
      '[aria-label="TokenBox mascot holding OpenAI and Claude logos"]'
    )
    assert.ok(mascot?.querySelector('[aria-label="OpenAI"] svg'))
    assert.ok(mascot?.querySelector('[aria-label="Claude"] svg'))
    assert.ok(rendered.container.querySelector('[aria-label="OpenAI"] svg'))
    assert.ok(rendered.container.querySelector('[aria-label="Claude"] svg'))

    await unmountHero(rendered)
  })

  test('sends an authenticated visitor to the dashboard', async () => {
    const rendered = await renderHero(true)

    assert.equal(
      findLink(rendered.container, 'Go to Dashboard').getAttribute('href'),
      '/dashboard'
    )

    await unmountHero(rendered)
  })

  test('links the onboarding action to API key creation', async () => {
    const rendered = await renderOnboarding()

    assert.equal(
      findLink(rendered.container, 'Create an API KEY~').getAttribute('href'),
      '/keys'
    )

    await unmountHero(rendered)
  })
})
