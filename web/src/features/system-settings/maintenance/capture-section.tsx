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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { getChannels } from '@/features/channels/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { ChannelSelectorDialog } from '../models/channel-selector-dialog'
import type { UpstreamChannel } from '../types'
import { safeNumberFieldProps } from '../utils/numeric-field'

// Mirrors the bounds enforced by capture_setting.ValidateOption on the server.
const MAX_BODY_MB_LIMIT = 1024
const QUEUE_SIZE_LIMIT = 100000
const CHANNEL_PAGE_SIZE = 100

/**
 * Excluded user IDs are edited as free text ("1, 2, 3") but persisted as a
 * JSON integer array. Returns null when any token is not a positive integer.
 */
function parseUserIdList(raw: string): number[] | null {
  const tokens = raw
    .split(/[\s,，]+/)
    .map((token) => token.trim())
    .filter(Boolean)
  const ids: number[] = []
  for (const token of tokens) {
    if (!/^\d+$/.test(token)) return null
    const id = Number(token)
    if (!Number.isSafeInteger(id) || id <= 0) return null
    if (!ids.includes(id)) ids.push(id)
  }
  return ids
}

/**
 * Dotted option keys would be read by react-hook-form as nested paths, so the
 * form state is modelled as a nested object and flattened before saving (same
 * approach as performance-section.tsx).
 */
const captureSchema = z.object({
  capture_setting: z
    .object({
      channel_ids: z.array(z.number().int().positive()),
      only_qualified: z.boolean(),
      excluded_user_ids: z
        .string()
        .refine((value) => parseUserIdList(value) !== null, {
          message:
            'Enter positive integer user IDs separated by commas, e.g. 1, 2, 3',
        }),
      local_enabled: z.boolean(),
      local_dir: z.string(),
      s3_enabled: z.boolean(),
      s3_bucket: z.string(),
      s3_region: z.string(),
      s3_endpoint: z.string(),
      s3_prefix: z.string(),
      s3_access_key_id: z.string(),
      s3_secret: z.string(),
      max_body_mb: z.coerce.number().int().min(1).max(MAX_BODY_MB_LIMIT),
      queue_size: z.coerce.number().int().min(1).max(QUEUE_SIZE_LIMIT),
    })
    .superRefine((values, ctx) => {
      if (!values.s3_enabled) return
      if (!values.s3_bucket.trim()) {
        ctx.addIssue({
          code: 'custom',
          path: ['s3_bucket'],
          message: 'Bucket is required when S3 upload is enabled',
        })
      }
      if (!values.s3_region.trim()) {
        ctx.addIssue({
          code: 'custom',
          path: ['s3_region'],
          message: 'Region is required when S3 upload is enabled',
        })
      }
    }),
})

type CaptureFormInput = z.input<typeof captureSchema>
type CaptureFormValues = z.output<typeof captureSchema>

/** Values as returned by GET /api/option. `s3_secret` is write-only. */
type FlatCaptureDefaults = {
  'capture_setting.channel_ids': number[]
  'capture_setting.excluded_user_ids': number[]
  'capture_setting.only_qualified': boolean
  'capture_setting.local_enabled': boolean
  'capture_setting.local_dir': string
  'capture_setting.s3_enabled': boolean
  'capture_setting.s3_bucket': string
  'capture_setting.s3_region': string
  'capture_setting.s3_endpoint': string
  'capture_setting.s3_prefix': string
  'capture_setting.s3_access_key_id': string
  'capture_setting.max_body_mb': number
  'capture_setting.queue_size': number
}

const buildFormDefaults = (
  defaults: FlatCaptureDefaults
): CaptureFormInput => ({
  capture_setting: {
    channel_ids: defaults['capture_setting.channel_ids'] ?? [],
    excluded_user_ids: (
      defaults['capture_setting.excluded_user_ids'] ?? []
    ).join(', '),
    only_qualified: defaults['capture_setting.only_qualified'] ?? true,
    local_enabled: defaults['capture_setting.local_enabled'],
    local_dir: defaults['capture_setting.local_dir'] ?? '',
    s3_enabled: defaults['capture_setting.s3_enabled'],
    s3_bucket: defaults['capture_setting.s3_bucket'] ?? '',
    s3_region: defaults['capture_setting.s3_region'] ?? '',
    s3_endpoint: defaults['capture_setting.s3_endpoint'] ?? '',
    s3_prefix: defaults['capture_setting.s3_prefix'] ?? '',
    s3_access_key_id: defaults['capture_setting.s3_access_key_id'] ?? '',
    s3_secret: '',
    max_body_mb: defaults['capture_setting.max_body_mb'],
    queue_size: defaults['capture_setting.queue_size'],
  },
})

const normalizeFormValues = (
  values: CaptureFormValues
): FlatCaptureDefaults => {
  const v = values.capture_setting
  return {
    'capture_setting.channel_ids': [...v.channel_ids].sort((a, b) => a - b),
    'capture_setting.excluded_user_ids':
      parseUserIdList(v.excluded_user_ids) ?? [],
    'capture_setting.only_qualified': v.only_qualified,
    'capture_setting.local_enabled': v.local_enabled,
    'capture_setting.local_dir': v.local_dir.trim(),
    'capture_setting.s3_enabled': v.s3_enabled,
    'capture_setting.s3_bucket': v.s3_bucket.trim(),
    'capture_setting.s3_region': v.s3_region.trim(),
    'capture_setting.s3_endpoint': v.s3_endpoint.trim(),
    'capture_setting.s3_prefix': v.s3_prefix.trim(),
    'capture_setting.s3_access_key_id': v.s3_access_key_id.trim(),
    'capture_setting.max_body_mb': v.max_body_mb,
    'capture_setting.queue_size': v.queue_size,
  }
}

async function fetchAllChannels(): Promise<UpstreamChannel[]> {
  const channels: UpstreamChannel[] = []
  for (let page = 1; ; page++) {
    const res = await getChannels({
      p: page,
      page_size: CHANNEL_PAGE_SIZE,
      id_sort: true,
    })
    const items = res.data?.items ?? []
    for (const channel of items) {
      channels.push({
        id: channel.id,
        name: channel.name,
        base_url: channel.base_url ?? '',
        status: channel.status,
        type: channel.type,
      })
    }
    const total = res.data?.total ?? 0
    if (
      !res.success ||
      items.length < CHANNEL_PAGE_SIZE ||
      channels.length >= total
    ) {
      return channels
    }
  }
}

interface Props {
  defaultValues: FlatCaptureDefaults
}

export function CaptureSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [channelDialogOpen, setChannelDialogOpen] = useState(false)

  const { data: channels = [] } = useQuery({
    queryKey: ['capture-setting-channels'],
    queryFn: fetchAllChannels,
  })

  const channelNames = useMemo(
    () => new Map(channels.map((channel) => [channel.id, channel.name])),
    [channels]
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<CaptureFormInput, unknown, CaptureFormValues>({
    resolver: zodResolver(captureSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatCaptureDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const onSubmit = async (values: CaptureFormValues) => {
    const normalized = normalizeFormValues(values)
    const baseline = baselineRef.current
    const updates: Array<{ key: string; value: string | number | boolean }> = []

    for (const key of Object.keys(normalized) as Array<
      keyof FlatCaptureDefaults
    >) {
      const value = normalized[key]
      if (Array.isArray(value)) {
        const serialized = JSON.stringify(value)
        if (serialized !== JSON.stringify(baseline[key] ?? [])) {
          updates.push({ key, value: serialized })
        }
        continue
      }
      if (value !== baseline[key]) updates.push({ key, value })
    }

    const secret = values.capture_setting.s3_secret.trim()
    if (secret) {
      updates.push({ key: 'capture_setting.s3_secret', value: secret })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
  }

  const localEnabled = form.watch('capture_setting.local_enabled')
  const s3Enabled = form.watch('capture_setting.s3_enabled')

  return (
    <SettingsSection title={t('Upstream Capture')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Save the request body sent to the upstream and the upstream response body (including streams) for selected channels.'
            )}
          </p>

          <FormField
            control={form.control}
            name='capture_setting.channel_ids'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Captured channels')}</FormLabel>
                <div className='flex flex-wrap items-center gap-2'>
                  {field.value.length === 0 ? (
                    <span className='text-muted-foreground text-sm'>
                      {t('No channels selected')}
                    </span>
                  ) : (
                    field.value.map((id) => (
                      <StatusBadge
                        key={id}
                        label={`#${id} ${channelNames.get(id) ?? ''}`.trim()}
                        variant='neutral'
                        size='sm'
                        copyable={false}
                      />
                    ))
                  )}
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setChannelDialogOpen(true)}
                  >
                    {t('Select channels')}
                  </Button>
                </div>
                <FormDescription>
                  {t(
                    'Only traffic on these channels is captured, and only when at least one storage target is enabled.'
                  )}
                </FormDescription>
                <FormMessage />
                <ChannelSelectorDialog
                  open={channelDialogOpen}
                  onOpenChange={setChannelDialogOpen}
                  channels={channels}
                  selectedChannelIds={field.value}
                  onSelectedChannelIdsChange={field.onChange}
                  onConfirm={() => undefined}
                  title={t('Select captured channels')}
                  description={t(
                    'Choose channels whose upstream requests and responses are captured'
                  )}
                />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='capture_setting.excluded_user_ids'
            render={({ field }) => (
              <FormItem className='max-w-md'>
                <FormLabel>{t('Excluded user IDs')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('e.g. 1, 2, 3')} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Requests from these users are never captured. Separate IDs with commas.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='capture_setting.only_qualified'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Only keep qualified records')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Skip Chat Completions requests and drop records whose response lacks a reasoning signature or a tool call, or did not finish successfully'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <Separator />

          <div>
            <h4 className='font-medium'>{t('Local storage')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Write capture records as JSON files on the server disk.')}
            </p>
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='capture_setting.local_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable local storage')}</FormLabel>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.local_dir'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Local directory')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Leave empty to use logs/captures')}
                      {...field}
                      disabled={!localEnabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Separator />

          <div>
            <h4 className='font-medium'>{t('S3 upload')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Upload capture records to AWS S3 or an S3-compatible object storage.'
              )}
            </p>
          </div>

          <FormField
            control={form.control}
            name='capture_setting.s3_enabled'
            render={({ field }) => (
              <SettingsSwitchItem className='max-w-md'>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable S3 upload')}</FormLabel>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='capture_setting.s3_bucket'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Bucket')}</FormLabel>
                  <FormControl>
                    <Input {...field} disabled={!s3Enabled} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.s3_region'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Region')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='us-east-1'
                      {...field}
                      disabled={!s3Enabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.s3_endpoint'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Endpoint')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Leave empty to use AWS S3')}
                      {...field}
                      disabled={!s3Enabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Set this for S3-compatible storage such as MinIO or R2'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.s3_prefix'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Key prefix')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='captures/'
                      {...field}
                      disabled={!s3Enabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.s3_access_key_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Access Key ID')}</FormLabel>
                  <FormControl>
                    <Input
                      autoComplete='off'
                      {...field}
                      disabled={!s3Enabled}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.s3_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Secret Access Key')}</FormLabel>
                  <FormControl>
                    <Input
                      autoComplete='new-password'
                      type='password'
                      placeholder={t('Enter new secret to update')}
                      {...field}
                      disabled={!s3Enabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave blank to keep the existing credential')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Separator />

          <div>
            <h4 className='font-medium'>{t('Limits')}</h4>
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='capture_setting.max_body_mb'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max body size (MB)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={MAX_BODY_MB_LIMIT}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Request and response bodies are each truncated beyond this size'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='capture_setting.queue_size'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Queue size')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={QUEUE_SIZE_LIMIT}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Maximum number of capture records waiting to be written'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
