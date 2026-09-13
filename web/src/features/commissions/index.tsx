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
import { getCommissionTimeRange } from './lib/time-range'
import { CommissionDateRangeFields } from './components/commission-date-range-fields'

const PARTICIPANT_PAGE_SIZE = 5
const RECORD_PAGE_SIZE = 10

function formatCnyAmount(amount: number) {
  return `¥${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount)}`
}

function displayPerson(username: string, name: string, email: string) {
  return name || username || email || '-'
}

function validRate(value: string) {
  const rate = Number(value)
  return (
    value.trim() !== '' && Number.isFinite(rate) && rate >= 0 && rate <= 100
  )
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
      <TableCell>{props.agent.parent_email || t('None')}</TableCell>
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
  const [participantKeyword, setParticipantKeyword] = useState('')
  const [recordKeyword, setRecordKeyword] = useState('')
  const [appliedRecordKeyword, setAppliedRecordKeyword] = useState('')
  const [agentPage, setAgentPage] = useState(1)
  const [recordPage, setRecordPage] = useState(1)
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [appliedDateRange, setAppliedDateRange] = useState({
    start: '',
    end: '',
  })
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
    queryKey: ['commission-agents', agentPage, participantKeyword],
    queryFn: () =>
      getCommissionAgents(agentPage, PARTICIPANT_PAGE_SIZE, participantKeyword),
  })
  const { startTime, endTime } = getCommissionTimeRange(
    appliedDateRange.start,
    appliedDateRange.end
  )
  const summaryQuery = useQuery({
    queryKey: ['commission-summary', appliedRecordKeyword, startTime, endTime],
    queryFn: () =>
      getCommissionSummary(appliedRecordKeyword, startTime, endTime),
  })
  const recordsQuery = useQuery({
    queryKey: [
      'commission-records',
      recordPage,
      appliedRecordKeyword,
      startTime,
      endTime,
    ],
    queryFn: () =>
      getCommissionRecords(
        recordPage,
        RECORD_PAGE_SIZE,
        appliedRecordKeyword,
        startTime,
        endTime
      ),
  })

  const updateSetting = (
    payload: Parameters<typeof updateCommissionSettings>[0]
  ) => settingsMutation.mutate(payload)
  const agentData = agentsQuery.data?.data
  const recordData = recordsQuery.data?.data
  const cnySummary = (summaryQuery.data?.data?.items ?? []).find(
    (summary) => summary.currency.toUpperCase() === 'CNY'
  )
  const totalRechargeCny = (cnySummary?.payment_amount_minor ?? 0) / 100
  const totalCommissionCny =
    (cnySummary?.commission_amount_minor ?? 0) / 100

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
            </div>
            <div className='grid gap-4 md:grid-cols-3'>
              <label className='flex min-h-10 items-center justify-between gap-3'>
                <span>{t('Commission master switch')}</span>
                <Switch
                  aria-label={t('Commission master switch')}
                  checked={settings?.enabled ?? false}
                  onCheckedChange={(checked) =>
                    updateSetting({ enabled: checked })
                  }
                />
              </label>
              <label className='flex min-h-10 items-center justify-between gap-3'>
                <span>{t('Enable global default rate')}</span>
                <Switch
                  aria-label={t('Enable global default rate')}
                  checked={settings?.global_rate_enabled ?? false}
                  onCheckedChange={(checked) =>
                    updateSetting({ global_rate_enabled: checked })
                  }
                />
              </label>
              <div className='grid gap-1'>
                <Label htmlFor='global-rate'>{t('Global default rate')}</Label>
                <div className='flex items-center gap-2'>
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
                    className='min-w-0 flex-1'
                  />
                  <span>%</span>
                </div>
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
                  value={participantKeyword}
                  onChange={(event) => {
                    setParticipantKeyword(event.target.value)
                    setAgentPage(1)
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
                    <TableHead>{t('Parent email')}</TableHead>
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
              pageSize={PARTICIPANT_PAGE_SIZE}
              onPageChange={setAgentPage}
            />
          </section>

          <section className='grid gap-4 md:grid-cols-2'>
            <div className='border-border rounded-lg border p-4 md:col-span-2'>
              <div className='text-muted-foreground text-sm'>
                {t('Current filter totals')}
              </div>
              <div className='mt-2 grid gap-4 sm:grid-cols-2'>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('Total recharge amount')}
                  </div>
                  <div className='text-lg font-semibold'>
                    {formatCnyAmount(totalRechargeCny)}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground text-xs'>
                    {t('Total commission amount')}
                  </div>
                  <div className='text-lg font-semibold'>
                    {formatCnyAmount(totalCommissionCny)}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className='border-border overflow-hidden rounded-lg border'>
            <div className='flex flex-wrap items-end justify-between gap-3 border-b p-4'>
              <div>
                <h3 className='font-semibold'>{t('Commission Details')}</h3>
                <p className='text-muted-foreground text-sm'>
                  {t('Records: {{count}}', { count: recordData?.total ?? 0 })}
                </p>
              </div>
              <div className='flex flex-wrap items-end gap-3'>
                <div className='grid gap-1'>
                  <Label htmlFor='commission-record-search'>
                    {t('Search users')}
                  </Label>
                  <div className='relative w-full sm:w-72'>
                    <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
                    <Input
                      id='commission-record-search'
                      className='pl-8'
                      value={recordKeyword}
                      onChange={(event) => {
                        setRecordKeyword(event.target.value)
                        setRecordPage(1)
                      }}
                      placeholder={t('Search user email, username or UID')}
                    />
                  </div>
                </div>
                <CommissionDateRangeFields
                  idPrefix='commission-records'
                  startDate={startDate}
                  endDate={endDate}
                  onStartDateChange={setStartDate}
                  onEndDateChange={setEndDate}
                />
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => {
                    setAppliedRecordKeyword(recordKeyword)
                    setAppliedDateRange({ start: startDate, end: endDate })
                    setRecordPage(1)
                  }}
                >
                  <Search />
                  {t('Search')}
                </Button>
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
                        {formatCnyAmount(record.payment_amount_minor / 100)}
                      </TableCell>
                      <TableCell>
                        {formatCnyAmount(
                          record.commission_amount_minor / 100
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
              pageSize={RECORD_PAGE_SIZE}
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
  pageSize: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const pages = Math.max(1, Math.ceil(props.total / props.pageSize))
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
