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
          price: 0.3200009,
          fallback: false,
        },
        {
          label: '720p wide',
          resolution: '720p wide',
          method: 'per_call',
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
    assert.match(expr, /charge\("per_call", quantity, 0\.470001\)/)
    assert.deepEqual(
      tryParseSeedanceExpr(expr),
      normalizeSeedanceConfig(config)
    )
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
})
