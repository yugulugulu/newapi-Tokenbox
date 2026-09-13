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
  isCommissionParticipantRole,
  shouldShowAgentEligibleBadge,
  USER_ROLE,
} from '../constants'

describe('user commission role presentation', () => {
  test('treats admins and agents as commission participants', () => {
    assert.equal(isCommissionParticipantRole(USER_ROLE.AGENT), true)
    assert.equal(isCommissionParticipantRole(USER_ROLE.ADMIN), true)
  })

  test('does not treat root or regular users as commission participants', () => {
    assert.equal(isCommissionParticipantRole(USER_ROLE.USER), false)
    assert.equal(isCommissionParticipantRole(USER_ROLE.ROOT), false)
  })

  test('shows the agent eligibility badge only for admins', () => {
    assert.equal(shouldShowAgentEligibleBadge(USER_ROLE.ADMIN), true)
    assert.equal(shouldShowAgentEligibleBadge(USER_ROLE.AGENT), false)
    assert.equal(shouldShowAgentEligibleBadge(USER_ROLE.USER), false)
    assert.equal(shouldShowAgentEligibleBadge(USER_ROLE.ROOT), false)
  })
})
