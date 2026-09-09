import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Save, Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import {
  getCommissionAgents,
  getCommissionRecords,
  getCommissionSettings,
  getCommissionSummary,
  updateCommissionAgent,
  updateCommissionSettings,
} from './api'
import type { CommissionAgent, CommissionRecord } from './types'

const PAGE_SIZE = 20

function formatMoney(minor: number, currency: string) {
  const zeroDecimal = ['IDR', 'JPY', 'KRW', 'VND'].includes(currency)
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: zeroDecimal ? 0 : 2,
  }).format(minor / (zeroDecimal ? 1 : 100))
}

function displayPerson(username: string, name: string, email: string) {
  return name || username || email || '-'
}

function validRate(value: string) {
  const rate = Number(value)
  return value.trim() !== '' && Number.isFinite(rate) && rate >= 0 && rate <= 100
}

function AgentRow(props: { agent: CommissionAgent }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draftRate, setDraftRate] = useState(
    String(props.agent.rate_percent ?? props.agent.rate_basis_points / 100)
  )
  let rateSource = t('None')
  if (props.agent.rate_source === 'agent') {
    rateSource = t('Individual')
  } else if (props.agent.rate_source === 'global') {
    rateSource = t('Global')
  }
  const mutation = useMutation({
    mutationFn: () => {
      if (!validRate(draftRate)) {
        throw new Error('invalid rate')
      }
      return updateCommissionAgent(props.agent.user_id, {
        use_custom_rate: props.agent.use_custom_rate,
        rate_percent: Number(draftRate),
      })
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      queryClient.invalidateQueries({ queryKey: ['commission-agents'] })
    },
    onError: () => toast.error(t('Save failed')),
  })
  const updateAgent = async (
    payload: Parameters<typeof updateCommissionAgent>[1]
  ) => {
    try {
      await updateCommissionAgent(props.agent.user_id, payload)
      await queryClient.invalidateQueries({ queryKey: ['commission-agents'] })
    } catch {
      toast.error(t('Save failed'))
    }
  }

  return (
    <TableRow>
      <TableCell className='font-medium'>
        <div>
          {displayPerson(
            props.agent.username,
            props.agent.display_name,
            props.agent.email
          )}
        </div>
        <div className='text-muted-foreground text-xs'>
          UID {props.agent.user_id}
        </div>
      </TableCell>
      <TableCell>{props.agent.role === 10 ? t('Admin') : t('Agent')}</TableCell>
      <TableCell>{props.agent.parent_user_id || t('None')}</TableCell>
      <TableCell>
        <div className='flex items-center gap-2'>
          <Switch
            checked={props.agent.use_custom_rate}
            onCheckedChange={(checked) => {
              if (!validRate(draftRate)) {
                toast.error(t('Rate must be between 0 and 100'))
                return
              }
              void updateAgent({
                use_custom_rate: checked,
                rate_percent: Number(draftRate),
              })
            }}
            aria-label={t('Use individual rate')}
          />
          <Input
            type='number'
            min='0'
            max='100'
            step='0.01'
            value={draftRate}
            onChange={(event) => setDraftRate(event.target.value)}
            className='w-24'
            aria-label={t('Individual rate percent')}
          />
          <span>%</span>
          <Button
            variant='outline'
            size='icon-sm'
            onClick={() => mutation.mutate()}
            disabled={mutation.isPending}
            aria-label={t('Save individual rate')}
            title={t('Save individual rate')}
          >
            <Save />
          </Button>
        </div>
      </TableCell>
      <TableCell>
        {props.agent.effective_rate_percent ??
          props.agent.effective_rate_basis_points / 100}
        %
      </TableCell>
      <TableCell>{rateSource}</TableCell>
    </TableRow>
  )
}

export function Commissions() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [agentPage, setAgentPage] = useState(1)
  const [recordPage, setRecordPage] = useState(1)
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [globalRate, setGlobalRate] = useState('0')
  const [globalRateDirty, setGlobalRateDirty] = useState(false)

  const settingsQuery = useQuery({
    queryKey: ['commission-settings'],
    queryFn: getCommissionSettings,
  })
  const settings = settingsQuery.data?.data
  const displayedGlobalRate = globalRateDirty
    ? globalRate
    : String(settings?.global_rate_percent ?? 0)
  const settingsMutation = useMutation({
    mutationFn: (payload: Parameters<typeof updateCommissionSettings>[0]) =>
      updateCommissionSettings(payload),
    onSuccess: () => {
      toast.success(t('Saved'))
      queryClient.invalidateQueries({ queryKey: ['commission-settings'] })
      queryClient.invalidateQueries({ queryKey: ['commission-agents'] })
    },
    onError: () => toast.error(t('Save failed')),
  })
  const agentsQuery = useQuery({
    queryKey: ['commission-agents', agentPage, keyword],
    queryFn: () => getCommissionAgents(agentPage, PAGE_SIZE, keyword),
    placeholderData: (previous) => previous,
  })
  const startTime = startDate
    ? Math.floor(new Date(`${startDate}T00:00:00`).getTime() / 1000)
    : 0
  const endTime = endDate
    ? Math.floor(new Date(`${endDate}T23:59:59`).getTime() / 1000)
    : 0
  const summaryQuery = useQuery({
    queryKey: ['commission-summary', keyword, startTime, endTime],
    queryFn: () => getCommissionSummary(keyword, startTime, endTime),
  })
  const recordsQuery = useQuery({
    queryKey: ['commission-records', recordPage, keyword, startTime, endTime],
    queryFn: () =>
      getCommissionRecords(recordPage, PAGE_SIZE, keyword, startTime, endTime),
    placeholderData: (previous) => previous,
  })

  const updateSetting = (
    payload: Parameters<typeof updateCommissionSettings>[0]
  ) => settingsMutation.mutate(payload)
  const agentData = agentsQuery.data?.data
  const recordData = recordsQuery.data?.data

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Commission Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-6'>
          <section className='border-border grid gap-4 rounded-lg border p-4'>
            <div>
              <h3 className='font-semibold'>{t('Commission Settings')}</h3>
              <p className='text-muted-foreground text-sm'>
                {t('Configure the global switch and fallback commission rate.')}
              </p>
            </div>
            <div className='grid gap-4 md:grid-cols-3'>
              <label className='flex items-center justify-between gap-3'>
                <span>{t('Commission enabled')}</span>
                <Switch
                  checked={settings?.enabled ?? false}
                  onCheckedChange={(checked) =>
                    updateSetting({ enabled: checked })
                  }
                />
              </label>
              <label className='flex items-center justify-between gap-3'>
                <span>{t('Global rate enabled')}</span>
                <Switch
                  checked={settings?.global_rate_enabled ?? false}
                  onCheckedChange={(checked) =>
                    updateSetting({ global_rate_enabled: checked })
                  }
                />
              </label>
              <div className='flex items-center gap-2'>
                <Label htmlFor='global-rate'>{t('Global rate')}</Label>
                <Input
                  id='global-rate'
                  type='number'
                  min='0'
                  max='100'
                  step='0.01'
                  value={displayedGlobalRate}
                  onChange={(event) => {
                    setGlobalRate(event.target.value)
                    setGlobalRateDirty(true)
                  }}
                  onBlur={() => {
                    if (!validRate(displayedGlobalRate)) {
                      toast.error(t('Rate must be between 0 and 100'))
                      setGlobalRateDirty(false)
                      return
                    }
                    updateSetting({
                      global_rate_percent: Number(displayedGlobalRate),
                    })
                    setGlobalRateDirty(false)
                  }}
                />
                <span>%</span>
              </div>
            </div>
          </section>

          <section className='border-border overflow-hidden rounded-lg border'>
            <div className='flex flex-wrap items-center justify-between gap-3 border-b p-4'>
              <div>
                <h3 className='font-semibold'>
                  {t('Commission Participants')}
                </h3>
                <p className='text-muted-foreground text-sm'>
                  {t('Admins and agents can participate. Root users cannot.')}
                </p>
              </div>
              <div className='relative w-full sm:w-72'>
                <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
                <Input
                  className='pl-8'
                  value={keyword}
                  onChange={(event) => {
                    setKeyword(event.target.value)
                    setAgentPage(1)
                    setRecordPage(1)
                  }}
                  placeholder={t('Search UID, email or name')}
                />
              </div>
            </div>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Participant')}</TableHead>
                    <TableHead>{t('Role')}</TableHead>
                    <TableHead>{t('Parent UID')}</TableHead>
                    <TableHead>{t('Individual rate')}</TableHead>
                    <TableHead>{t('Effective rate')}</TableHead>
                    <TableHead>{t('Rate source')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(agentData?.items ?? []).map((agent) => (
                    <AgentRow key={agent.user_id} agent={agent} />
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pager
              page={agentData?.page ?? agentPage}
              total={agentData?.total ?? 0}
              onPageChange={setAgentPage}
            />
          </section>

          <section className='grid gap-4 md:grid-cols-2'>
            {(summaryQuery.data?.data?.items ?? []).map((summary) => (
              <div
                key={summary.currency}
                className='border-border rounded-lg border p-4'
              >
                <div className='text-muted-foreground text-sm'>
                  {summary.currency}
                </div>
                <div className='mt-2 grid grid-cols-2 gap-3'>
                  <div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Recharge amount')}
                    </div>
                    <div className='font-semibold'>
                      {formatMoney(
                        summary.payment_amount_minor,
                        summary.currency
                      )}
                    </div>
                  </div>
                  <div>
                    <div className='text-muted-foreground text-xs'>
                      {t('Commission amount')}
                    </div>
                    <div className='font-semibold'>
                      {formatMoney(
                        summary.commission_amount_minor,
                        summary.currency
                      )}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </section>

          <section className='border-border overflow-hidden rounded-lg border'>
            <div className='flex flex-wrap items-end gap-3 border-b p-4'>
              <div className='grid gap-1'>
                <Label htmlFor='start-date'>{t('Start date')}</Label>
                <Input
                  id='start-date'
                  type='date'
                  value={startDate}
                  onChange={(event) => {
                    setStartDate(event.target.value)
                    setRecordPage(1)
                  }}
                />
              </div>
              <div className='grid gap-1'>
                <Label htmlFor='end-date'>{t('End date')}</Label>
                <Input
                  id='end-date'
                  type='date'
                  value={endDate}
                  onChange={(event) => {
                    setEndDate(event.target.value)
                    setRecordPage(1)
                  }}
                />
              </div>
              <div className='text-muted-foreground text-sm'>
                {t('Records: {{count}}', { count: recordData?.total ?? 0 })}
              </div>
            </div>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Agent')}</TableHead>
                    <TableHead>{t('Descendant')}</TableHead>
                    <TableHead>{t('Recharge amount')}</TableHead>
                    <TableHead>{t('Commission amount')}</TableHead>
                    <TableHead>{t('Rate')}</TableHead>
                    <TableHead>{t('Time')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(recordData?.items ?? []).map((record: CommissionRecord) => (
                    <TableRow key={record.id}>
                      <TableCell>
                        {displayPerson(
                          record.agent_username,
                          record.agent_display_name,
                          record.agent_email
                        )}
                        <div className='text-muted-foreground text-xs'>
                          UID {record.agent_user_id}
                        </div>
                      </TableCell>
                      <TableCell>
                        {displayPerson(
                          record.descendant_username,
                          record.descendant_name,
                          record.descendant_email
                        )}
                        <div className='text-muted-foreground text-xs'>
                          UID {record.descendant_user_id}
                        </div>
                      </TableCell>
                      <TableCell>
                        {formatMoney(
                          record.payment_amount_minor,
                          record.payment_currency
                        )}
                      </TableCell>
                      <TableCell>
                        {formatMoney(
                          record.commission_amount_minor,
                          record.payment_currency
                        )}
                      </TableCell>
                      <TableCell>
                        {record.commission_rate_basis_points / 100}%
                      </TableCell>
                      <TableCell>
                        {formatTimestampToDate(record.created_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pager
              page={recordData?.page ?? recordPage}
              total={recordData?.total ?? 0}
              onPageChange={setRecordPage}
            />
          </section>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function Pager(props: {
  page: number
  total: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const pages = Math.max(1, Math.ceil(props.total / PAGE_SIZE))
  return (
    <div className='flex items-center justify-end gap-2 border-t p-3'>
      <Button
        variant='outline'
        size='sm'
        disabled={props.page <= 1}
        onClick={() => props.onPageChange(props.page - 1)}
      >
        {t('Previous')}
      </Button>
      <span className='text-muted-foreground text-sm'>
        {props.page} / {pages}
      </span>
      <Button
        variant='outline'
        size='sm'
        disabled={props.page >= pages}
        onClick={() => props.onPageChange(props.page + 1)}
      >
        {t('Next')}
      </Button>
    </div>
  )
}
