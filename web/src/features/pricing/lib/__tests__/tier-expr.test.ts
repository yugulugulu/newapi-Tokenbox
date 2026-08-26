/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  generateSeedanceExpr,
  normalizeSeedanceConfig,
  tryParseSeedanceExpr,
} from '../tier-expr.ts'
import { parseTiersFromExpr } from '../billing-expr.ts'

describe('Seedance v2 tier expression editor helpers', () => {
  test('generates and parses a single fallback tier', () => {
    const config = normalizeSeedanceConfig({
      tiers: [
        {
          label: 'fallback',
          method: 'per_second',
          price: 0.32,
          fallback: true,
        },
      ],
    })
    const expr = generateSeedanceExpr(config)
    assert.equal(
      expr,
      'v2:tier("fallback", charge("per_second", quantity, 0.32))'
    )
    assert.deepEqual(tryParseSeedanceExpr(expr), config)
  })

  test('supports resolution tiers, per-call pricing, and six decimal places', () => {
    const config = normalizeSeedanceConfig({
      tiers: [
        {
          label: '480p',
          resolution: ' 480p ',
          method: 'per_second',
          videoInput: 'without_video',
          price: 0.3200009,
          fallback: false,
        },
        {
          label: '720p wide',
          resolution: '720p wide',
          method: 'per_second',
          videoInput: 'with_video',
          price: 0.470001,
          fallback: false,
        },
        {
          label: 'fallback tier',
          method: 'per_second',
          price: 0.55,
          fallback: true,
        },
      ],
    })
    const expr = generateSeedanceExpr(config)
    assert.match(expr, /param\("resolution"\) == "480p"/)
    assert.match(expr, /&& !has_media\("video"\)/)
    assert.match(expr, /&& has_media\("video"\)/)
    assert.match(expr, /charge\("per_second", quantity, 0\.470001\)/)
    assert.deepEqual(
      tryParseSeedanceExpr(expr),
      normalizeSeedanceConfig(config)
    )
  })

  test('expands a legacy per-second tier into equal video and no-video tiers', () => {
    const legacyExpr =
      'v2:param("resolution") == "720p" ? tier("720p", charge("per_second", quantity, 0.51)) : tier("fallback", charge("per_call", quantity, 1))'
    const parsed = tryParseSeedanceExpr(legacyExpr)

    assert.deepEqual(parsed, {
      tiers: [
        {
          label: '720p',
          resolution: '720p',
          method: 'per_second',
          videoInput: 'without_video',
          price: 0.51,
          fallback: false,
        },
        {
          label: '720p',
          resolution: '720p',
          method: 'per_second',
          videoInput: 'with_video',
          price: 0.51,
          fallback: false,
        },
        {
          label: 'fallback',
          resolution: undefined,
          method: 'per_call',
          videoInput: undefined,
          price: 1,
          fallback: true,
        },
      ],
    })
  })

  test('keeps per-call resolution tiers independent of video input', () => {
    const config = normalizeSeedanceConfig({
      tiers: [
        {
          label: '720p_call',
          resolution: '720p',
          method: 'per_call',
          price: 0.8,
          fallback: false,
        },
        {
          label: 'fallback',
          method: 'per_call',
          price: 1,
          fallback: true,
        },
      ],
    })

    const expr = generateSeedanceExpr(config)
    assert.equal(
      expr,
      'v2:param("resolution") == "720p" ? tier("720p_call", charge("per_call", quantity, 0.8)) : tier("fallback", charge("per_call", quantity, 1))'
    )
    assert.doesNotMatch(expr, /has_media/)
    assert.deepEqual(tryParseSeedanceExpr(expr), config)
  })

  test('rejects v1 and unrelated v2 expressions', () => {
    assert.equal(tryParseSeedanceExpr('tier("base", p * 2)'), null)
    assert.equal(tryParseSeedanceExpr('v2:tier("base", p * 2)'), null)
    assert.equal(
      tryParseSeedanceExpr(
        'v2:param("resolution") == "480p" ? tier("480p", charge("per_second", quantity, 0.3))'
      ),
      null
    )
  })

  test('exposes video-input conditions in pricing details', () => {
    const expr =
      'v2:param("resolution") == "720p" && !has_media("video") ? tier("720p_no_video", charge("per_second", quantity, 0.51)) : param("resolution") == "720p" && has_media("video") ? tier("720p_video", charge("per_second", quantity, 0.31)) : tier("fallback", charge("per_second", quantity, 0.46))'

    assert.deepEqual(parseTiersFromExpr(expr), [
      {
        label: '720p_no_video',
        conditions: [],
        billing_method: 'per_second',
        unit_price: 0.51,
        resolution: '720p',
        video_input: 'without_video',
      },
      {
        label: '720p_video',
        conditions: [],
        billing_method: 'per_second',
        unit_price: 0.31,
        resolution: '720p',
        video_input: 'with_video',
      },
      {
        label: 'fallback',
        conditions: [],
        billing_method: 'per_second',
        unit_price: 0.46,
      },
    ])
  })
})
