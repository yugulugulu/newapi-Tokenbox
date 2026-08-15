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
import { describe, test } from 'node:test'

import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  type ChannelFormValues,
} from '../channel-form'

function formValues(
  overrides: Partial<ChannelFormValues> = {}
): ChannelFormValues {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Image channel',
    key: 'test-key',
    models: 'gpt-image-2',
    ...overrides,
  }
}

function parseCreatedSettings(form: ChannelFormValues) {
  const payload = transformFormDataToCreatePayload(form)
  return JSON.parse(String(payload.channel.settings)) as Record<string, unknown>
}

function channelWithSettings(settings: string): Channel {
  return {
    id: 1,
    type: 1,
    key: '',
    status: 1,
    name: 'Image channel',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'gpt-image-2',
    group: 'default',
    used_quota: 0,
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
    },
    settings,
    other: '',
    balance: 0,
    other_info: '',
    remark: '',
    max_input_tokens: 0,
  }
}

describe('gpt-image-2 channel resolution limit', () => {
  test('stores a configured limit while preserving other settings', () => {
    const settings = parseCreatedSettings(
      formValues({
        image_resolution_limit: '2k',
        settings: '{"custom_setting":"keep"}',
      })
    )

    assert.equal(settings.image_resolution_limit, '2k')
    assert.equal(settings.custom_setting, 'keep')
  })

  test('omits unlimited and removes stale settings without a matching model', () => {
    const unlimited = parseCreatedSettings(
      formValues({ image_resolution_limit: 'unlimited' })
    )
    assert.equal('image_resolution_limit' in unlimited, false)

    const withoutMatchingModel = parseCreatedSettings(
      formValues({
        models: 'gpt-image-1',
        image_resolution_limit: '4k',
        settings: '{"image_resolution_limit":"1k","custom_setting":"keep"}',
      })
    )
    assert.equal('image_resolution_limit' in withoutMatchingModel, false)
    assert.equal(withoutMatchingModel.custom_setting, 'keep')
  })

  test('restores supported limits and defaults old channels to unlimited', () => {
    assert.equal(
      transformChannelToFormDefaults(
        channelWithSettings('{"image_resolution_limit":"4k"}')
      ).image_resolution_limit,
      '4k'
    )
    assert.equal(
      transformChannelToFormDefaults(channelWithSettings('{}'))
        .image_resolution_limit,
      'unlimited'
    )
    assert.equal(
      transformChannelToFormDefaults(
        channelWithSettings('{"image_resolution_limit":"unsupported"}')
      ).image_resolution_limit,
      'unlimited'
    )
  })
})
