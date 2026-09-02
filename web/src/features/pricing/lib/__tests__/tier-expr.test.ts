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

import { parseTiersFromExpr } from '../billing-expr.ts'
import {
  generateSeedanceExpr,
  normalizeSeedanceConfig,
  tryParseSeedanceExpr,
} from '../tier-expr.ts'

describe('Seedance v2 tier expression editor helpers', () => {
  test('generates and parses a single resolution tier without fallback pricing', () => {
    const config = normalizeSeedanceConfig({
      tiers: [
        {
          label: '480p',
          resolution: '480p',
          method: 'per_second',
          videoInput: 'without_video',
          price: 0.32,
          fallback: false,
        },
      ],
    })
    const expr = generateSeedanceExpr(config)
    assert.equal(
      expr,
      'v2:param("resolution") == "480p" && !has_media("video") ? tier("480p", charge("per_second", quantity, 0.32)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'
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
          videoInputPrice: 0.120001,
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
    assert.match(
      expr,
      /charge\("per_second", video_input_durations, 0\.120001\)/
    )
    assert.deepEqual(
      tryParseSeedanceExpr(expr),
      normalizeSeedanceConfig(config)
    )
  })

  test('does not generate a resolution condition for an incomplete added tier', () => {
    const config = normalizeSeedanceConfig({
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
          label: 'tier_2',
          resolution: '',
          method: 'per_second',
          videoInput: 'without_video',
          price: 0,
          fallback: false,
        },
      ],
    })

    const expr = generateSeedanceExpr(config)

    assert.doesNotMatch(expr, /param\("resolution"\) == ""/)
    assert.deepEqual(tryParseSeedanceExpr(expr), {
      tiers: [
        {
          label: '720p',
          resolution: '720p',
          method: 'per_second',
          videoInput: 'without_video',
          price: 0.51,
          fallback: false,
        },
      ],
    })
  })

  test('ignores an incomplete resolution branch when reopening an expression', () => {
    const expr =
      'v2:param("resolution") == "720p" ? tier("720p", charge("per_call", quantity, 0.8)) : param("resolution") == "" && !has_media("video") ? tier("tier_2", charge("per_second", quantity, 0)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'

    assert.deepEqual(tryParseSeedanceExpr(expr), {
      tiers: [
        {
          label: '720p',
          resolution: '720p',
          method: 'per_call',
          videoInput: undefined,
          price: 0.8,
          fallback: false,
        },
      ],
    })
  })

  test('ignores an incomplete first resolution branch when reopening an expression', () => {
    const expr =
      'v2:param("resolution") == "" && !has_media("video") ? tier("tier_1", charge("per_second", quantity, 0)) : param("resolution") == "720p" ? tier("720p", charge("per_call", quantity, 0.8)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'

    assert.deepEqual(tryParseSeedanceExpr(expr), {
      tiers: [
        {
          label: '720p',
          resolution: '720p',
          method: 'per_call',
          videoInput: undefined,
          price: 0.8,
          fallback: false,
        },
      ],
    })
  })

  test('drops a legacy fallback tier when opening it in the editor', () => {
    const legacyExpr =
      'v2:param("resolution") == "720p" ? tier("720p", charge("per_second", quantity, 0.51)) : tier("custom default", charge("per_call", quantity, 1))'
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
      ],
    })
  })

  test('converts a standalone legacy fallback into an explicit 480p tier', () => {
    const legacyExpr =
      'v2:tier("custom default", charge("per_call", quantity, 1))'

    assert.deepEqual(tryParseSeedanceExpr(legacyExpr), {
      tiers: [
        {
          label: 'custom default',
          resolution: '480p',
          method: 'per_call',
          videoInput: undefined,
          price: 1,
          fallback: false,
        },
      ],
    })
  })

  test('preserves legacy fallback input video pricing when converting it to 480p', () => {
    const legacyExpr =
      'v2:tier("fallback", charge("per_second", quantity, 0.46) + charge("per_second", video_input_durations, 0.12))'

    assert.deepEqual(tryParseSeedanceExpr(legacyExpr), {
      tiers: [
        {
          label: 'fallback',
          resolution: '480p',
          method: 'per_second',
          videoInput: 'without_video',
          price: 0.46,
          fallback: false,
        },
        {
          label: 'fallback',
          resolution: '480p',
          method: 'per_second',
          videoInput: 'with_video',
          price: 0.46,
          videoInputPrice: 0.12,
          fallback: false,
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
      'v2:param("resolution") == "720p" ? tier("720p_call", charge("per_call", quantity, 0.8)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'
    )
    assert.doesNotMatch(expr, /has_media/)
    assert.deepEqual(tryParseSeedanceExpr(expr), config)
  })

  test('generates and parses input video duration pricing for a configured tier', () => {
    const config = normalizeSeedanceConfig({
      tiers: [
        {
          label: '720p_video',
          resolution: '720p',
          method: 'per_second',
          videoInput: 'with_video',
          price: 0.46,
          videoInputPrice: 0.12,
          fallback: false,
        },
      ],
    })

    const expr = generateSeedanceExpr(config)
    assert.equal(
      expr,
      'v2:param("resolution") == "720p" && has_media("video") ? tier("720p_video", charge("per_second", quantity, 0.46) + charge("per_second", video_input_durations, 0.12)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'
    )
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
      'v2:param("resolution") == "720p" && !has_media("video") ? tier("720p_no_video", charge("per_second", quantity, 0.51)) : param("resolution") == "720p" && has_media("video") ? tier("720p_video", charge("per_second", quantity, 0.31) + charge("per_second", video_input_durations, 0.12)) : tier("fallback", charge("per_second", quantity, 0.46) + charge("per_second", video_input_durations, 0.1))'

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
        video_input_unit_price: 0.12,
        resolution: '720p',
        video_input: 'with_video',
      },
      {
        label: 'fallback',
        conditions: [],
        billing_method: 'per_second',
        unit_price: 0.46,
        video_input_unit_price: 0.1,
      },
    ])
  })

  test('hides the unsupported-resolution sentinel from pricing details', () => {
    const expr =
      'v2:param("resolution") == "720p" ? tier("720p", charge("per_second", quantity, 0.51)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'

    assert.deepEqual(parseTiersFromExpr(expr), [
      {
        label: '720p',
        conditions: [],
        billing_method: 'per_second',
        unit_price: 0.51,
        resolution: '720p',
      },
    ])
  })

  test('keeps a real zero-priced tier named like the sentinel visible', () => {
    const expr =
      'v2:param("resolution") == "720p" ? tier("__unsupported_resolution__", charge("per_call", quantity, 1)) : tier("__unsupported_resolution__", charge("per_call", quantity, 0))'

    assert.deepEqual(parseTiersFromExpr(expr), [
      {
        label: '__unsupported_resolution__',
        conditions: [],
        billing_method: 'per_call',
        unit_price: 1,
        resolution: '720p',
      },
    ])
  })
})
