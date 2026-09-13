import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Banknote,
  ChevronLeft,
  ChevronRight,
  Search,
  Send,
  Share2,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  createCommissionSelfWithdrawal,
  getCommissionSelfBalance,
  getCommissionSelfRecords,
  getCommissionSelfReferrals,
  getCommissionSelfWithdrawals,
  transferCommissionBalanceToQuota,
} from '@/features/commissions/api'
import type { CommissionWithdrawal } from '@/features/commissions/types'
import { getCommissionTimeRange } from '@/features/commissions/lib/time-range'
import { CommissionDateRangeFields } from '@/features/commissions/components/commission-date-range-fields'
import { useSystemConfig } from '@/hooks/use-system-config'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

function formatUsdt(minor: number) {
  return `${new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 2,
  }).format(minor / 100)} USDT`
}

function formatUsd(minor: number) {
  return `$${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(minor / 100)}`
}

function formatCny(minor: number, currency: string, usdExchangeRate: number) {
  const normalizedCurrency = currency.toUpperCase()
  const zeroDecimal = ['IDR', 'JPY', 'KRW', 'VND'].includes(normalizedCurrency)
  const amount = minor / (zeroDecimal ? 1 : 100)
  const cnyAmount =
    normalizedCurrency === 'USD' ? amount * usdExchangeRate : amount
  return `¥${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(cnyAmount)}`
}

interface PageControlsProps {
  page: number
  total: number
  pageSize: number
  onPageChange: (page: number) => void
}

function PageControls(props: PageControlsProps) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))
  if (props.total <= 0) return null
  return (
    <div className='flex items-center justify-between border-t px-3 py-2'>
      <span className='text-muted-foreground text-xs'>
        {t('Showing')} {(props.page - 1) * props.pageSize + 1}-
        {Math.min(props.page * props.pageSize, props.total)} {t('of')}{' '}
        {props.total}
      </span>
      <div className='flex items-center gap-1'>
        <Button
          variant='ghost'
          size='icon-xs'
          onClick={() => props.onPageChange(props.page - 1)}
          disabled={props.page <= 1}
          aria-label={t('Previous')}
        >
          <ChevronLeft />
        </Button>
        <span className='text-muted-foreground min-w-12 text-center text-xs'>
          {props.page} / {totalPages}
        </span>
        <Button
          variant='ghost'
          size='icon-xs'
          onClick={() => props.onPageChange(props.page + 1)}
          disabled={props.page >= totalPages}
          aria-label={t('Next')}
        >
          <ChevronRight />
        </Button>
      </div>
    </div>
  )
}

interface ListFiltersProps {
  idPrefix: string
  keyword: string
  onKeywordChange: (value: string) => void
  startDate: string
  endDate: string
  onStartDateChange: (value: string) => void
  onEndDateChange: (value: string) => void
  onSearch: () => void
}

function ListFilters(props: ListFiltersProps) {
  const { t } = useTranslation()
  return (
    <div className='mb-2 flex flex-wrap items-end gap-3'>
      <div className='grid min-w-0 flex-1 gap-1 sm:min-w-56'>
        <Label htmlFor={`${props.idPrefix}-keyword`}>{t('Search users')}</Label>
        <div className='relative'>
          <Search className='text-muted-foreground absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2' />
          <Input
            id={`${props.idPrefix}-keyword`}
            value={props.keyword}
            onChange={(event) => props.onKeywordChange(event.target.value)}
            placeholder={t('Email or username')}
            className='h-8 pl-8 text-xs'
            aria-label={t('Email or username')}
          />
        </div>
      </div>
      <CommissionDateRangeFields
        idPrefix={props.idPrefix}
        startDate={props.startDate}
        endDate={props.endDate}
        onStartDateChange={props.onStartDateChange}
        onEndDateChange={props.onEndDateChange}
      />
      <Button type='button' size='sm' onClick={props.onSearch}>
        <Search />
        {t('Search')}
      </Button>
    </div>
  )
}

function withdrawalStatusLabel(
  status: CommissionWithdrawal['status'],
  t: ReturnType<typeof useTranslation>['t']
) {
  if (status === 'approved') return t('Withdrawal successful')
  if (status === 'rejected') return t('Withdrawal failed')
  return t('Under review')
}

function withdrawalStatusVariant(status: CommissionWithdrawal['status']) {
  if (status === 'pending') return 'warning' as const
  if (status === 'rejected') return 'destructive' as const
  return 'outline' as const
}

interface CommissionWalletCardProps {
  onBalanceChanged?: () => Promise<void> | void
}

export function CommissionWalletCard({
  onBalanceChanged,
}: CommissionWalletCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const role = useAuthStore((state) => state.auth.user?.role)
  const visible = role !== undefined && role !== ROLE.SUPER_ADMIN
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [withdrawalDialogOpen, setWithdrawalDialogOpen] = useState(false)
  const [withdrawalAddress, setWithdrawalAddress] = useState('')
  const [recordKeyword, setRecordKeyword] = useState('')
  const [appliedRecordKeyword, setAppliedRecordKeyword] = useState('')
  const [appliedRecordRange, setAppliedRecordRange] = useState({
    start: '',
    end: '',
  })
  const [referralKeyword, setReferralKeyword] = useState('')
  const [appliedReferralKeyword, setAppliedReferralKeyword] = useState('')
  const [appliedReferralRange, setAppliedReferralRange] = useState({
    start: '',
    end: '',
  })
  const [recordStartDate, setRecordStartDate] = useState('')
  const [recordEndDate, setRecordEndDate] = useState('')
  const [referralStartDate, setReferralStartDate] = useState('')
  const [referralEndDate, setReferralEndDate] = useState('')
  const [withdrawalStartDate, setWithdrawalStartDate] = useState('')
  const [withdrawalEndDate, setWithdrawalEndDate] = useState('')
  const [appliedWithdrawalRange, setAppliedWithdrawalRange] = useState({
    start: '',
    end: '',
  })
  const [recordPage, setRecordPage] = useState(1)
  const [referralPage, setReferralPage] = useState(1)
  const [withdrawalPage, setWithdrawalPage] = useState(1)
  const pageSize = 5
  const { currency } = useSystemConfig()
  const usdExchangeRate = currency.usdExchangeRate
  const withdrawalTimeRange = getCommissionTimeRange(
    appliedWithdrawalRange.start,
    appliedWithdrawalRange.end
  )
  const recordTimeRange = getCommissionTimeRange(
    appliedRecordRange.start,
    appliedRecordRange.end
  )
  const referralTimeRange = getCommissionTimeRange(
    appliedReferralRange.start,
    appliedReferralRange.end
  )

  const balanceQuery = useQuery({
    queryKey: ['commission-self-balance'],
    queryFn: getCommissionSelfBalance,
    enabled: visible,
    retry: false,
  })
  const withdrawalsQuery = useQuery({
    queryKey: [
      'commission-self-withdrawals',
      withdrawalPage,
      appliedWithdrawalRange,
    ],
    queryFn: () =>
      getCommissionSelfWithdrawals(
        withdrawalPage,
        pageSize,
        withdrawalTimeRange.startTime,
        withdrawalTimeRange.endTime
    ),
    enabled: visible,
    retry: false,
  })
  const recordsQuery = useQuery({
    queryKey: [
      'commission-self-records',
      recordPage,
      appliedRecordKeyword,
      appliedRecordRange,
    ],
    queryFn: () =>
      getCommissionSelfRecords(
        recordPage,
        pageSize,
        appliedRecordKeyword,
        recordTimeRange.startTime,
        recordTimeRange.endTime
    ),
    enabled: visible,
    retry: false,
  })
  const referralsQuery = useQuery({
    queryKey: [
      'commission-self-referrals',
      referralPage,
      appliedReferralKeyword,
      appliedReferralRange,
    ],
    queryFn: () =>
      getCommissionSelfReferrals(
        referralPage,
        pageSize,
        appliedReferralKeyword,
        referralTimeRange.startTime,
        referralTimeRange.endTime
    ),
    enabled: visible,
    retry: false,
  })

  useEffect(() => {
    const total = recordsQuery.data?.data?.total ?? 0
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    if (recordPage > totalPages) setRecordPage(totalPages)
  }, [recordPage, recordsQuery.data?.data?.total])

  useEffect(() => {
    const total = referralsQuery.data?.data?.total ?? 0
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    if (referralPage > totalPages) setReferralPage(totalPages)
  }, [referralPage, referralsQuery.data?.data?.total])

  useEffect(() => {
    const total = withdrawalsQuery.data?.data?.total ?? 0
    const totalPages = Math.max(1, Math.ceil(total / pageSize))
    if (withdrawalPage > totalPages) setWithdrawalPage(totalPages)
  }, [withdrawalPage, withdrawalsQuery.data?.data?.total])

  const transferMutation = useMutation({
    mutationFn: transferCommissionBalanceToQuota,
    onSuccess: () => {
      toast.success(t('Transferred to balance'))
      setTransferDialogOpen(false)
      void Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['commission-self-balance'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['commission-self-withdrawals'],
        }),
        onBalanceChanged?.(),
      ])
    },
    onError: () => toast.error(t('Transfer failed')),
  })

  const withdrawalMutation = useMutation({
    mutationFn: createCommissionSelfWithdrawal,
    onSuccess: () => {
      toast.success(t('Withdrawal request submitted'))
      setWithdrawalDialogOpen(false)
      setWithdrawalAddress('')
      void Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['commission-self-balance'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['commission-self-withdrawals'],
        }),
        onBalanceChanged?.(),
      ])
    },
    onError: () => toast.error(t('Withdrawal request failed')),
  })

  if (!visible) return null

  const balance = balanceQuery.data?.data
  const records = recordsQuery.data?.data?.items ?? []
  const referrals = referralsQuery.data?.data?.items ?? []
  const withdrawals = withdrawalsQuery.data?.data?.items ?? []

  return (
    <>
      <Card data-card-hover='false'>
        <CardHeader className='border-b'>
          <CardTitle className='flex items-center gap-2 text-sm'>
            <Share2 className='size-4' />
            {t('Commission and referrals')}
          </CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 p-4'>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Invitation count')}
            </div>
            <div className='font-semibold'>
              {referralsQuery.data?.data?.total ?? 0}
            </div>
          </div>

          <div className='bg-muted/30 rounded-lg border px-4 py-3'>
            <div className='text-muted-foreground text-xs'>
              {t('Total commission')}
            </div>
            <div className='mt-1 text-2xl font-semibold tracking-normal'>
              {formatUsd(balance?.total_commission_usd_minor ?? 0)}
            </div>
          </div>

          <div className='flex flex-wrap gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setTransferDialogOpen(true)}
              disabled={
                transferMutation.isPending ||
                !balance ||
                balance.transferable_quota <= 0
              }
            >
              <Banknote />
              {t('Transfer to Balance')}
            </Button>
            <Button
              size='sm'
              onClick={() => {
                if (!balance) return
                if (!balance.can_withdraw) {
                  toast.error(
                    t('Minimum withdrawal required', {
                      amount: formatUsd(balance.min_amount_usdt_minor),
                    })
                  )
                  return
                }
                setWithdrawalDialogOpen(true)
              }}
              disabled={withdrawalMutation.isPending}
            >
              <Send />
              {t('Withdraw')}
            </Button>
          </div>

          <div className='grid gap-4 lg:grid-cols-2'>
            <div className='min-w-0'>
              <h4 className='mb-2 text-sm font-semibold'>
                {t('Commission records')}
              </h4>
              <ListFilters
                idPrefix='commission-records'
                keyword={recordKeyword}
                onKeywordChange={setRecordKeyword}
                startDate={recordStartDate}
                endDate={recordEndDate}
                onStartDateChange={setRecordStartDate}
                onEndDateChange={setRecordEndDate}
                onSearch={() => {
                  setAppliedRecordKeyword(recordKeyword)
                  setAppliedRecordRange({
                    start: recordStartDate,
                    end: recordEndDate,
                  })
                  setRecordPage(1)
                }}
              />
              <div className='rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Descendant email')}</TableHead>
                      <TableHead>{t('Recharge amount')}</TableHead>
                      <TableHead>{t('Commission amount')}</TableHead>
                      <TableHead>{t('Time')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {records.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={4}
                          className='text-muted-foreground text-center'
                        >
                          {t('No records')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      records.map((record) => (
                        <TableRow key={record.id}>
                          <TableCell className='max-w-48'>
                            <span className='block truncate'>
                              {record.descendant_email || '-'}
                            </span>
                          </TableCell>
                          <TableCell>
                            {formatCny(
                              record.payment_amount_minor,
                              record.payment_currency,
                              usdExchangeRate
                            )}
                          </TableCell>
                          <TableCell className='font-medium'>
                            {formatCny(
                              record.commission_amount_minor,
                              record.payment_currency,
                              usdExchangeRate
                            )}
                          </TableCell>
                          <TableCell>
                            {formatTimestampToDate(record.created_at)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
                <PageControls
                  page={recordPage}
                  total={recordsQuery.data?.data?.total ?? 0}
                  pageSize={pageSize}
                  onPageChange={setRecordPage}
                />
              </div>
            </div>
            <div className='min-w-0'>
              <h4 className='mb-2 text-sm font-semibold'>
                {t('Invitation records')}
              </h4>
              <ListFilters
                idPrefix='commission-referrals'
                keyword={referralKeyword}
                onKeywordChange={setReferralKeyword}
                startDate={referralStartDate}
                endDate={referralEndDate}
                onStartDateChange={setReferralStartDate}
                onEndDateChange={setReferralEndDate}
                onSearch={() => {
                  setAppliedReferralKeyword(referralKeyword)
                  setAppliedReferralRange({
                    start: referralStartDate,
                    end: referralEndDate,
                  })
                  setReferralPage(1)
                }}
              />
              <div className='rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Invitee email')}</TableHead>
                      <TableHead>{t('Invitation time')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {referrals.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={2}
                          className='text-muted-foreground text-center'
                        >
                          {t('No records')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      referrals.map((referral) => (
                        <TableRow key={referral.id}>
                          <TableCell className='max-w-48'>
                            <span className='block truncate'>
                              {referral.email || '-'}
                            </span>
                          </TableCell>
                          <TableCell>
                            {formatTimestampToDate(referral.created_at)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
                <PageControls
                  page={referralPage}
                  total={referralsQuery.data?.data?.total ?? 0}
                  pageSize={pageSize}
                  onPageChange={setReferralPage}
                />
              </div>
            </div>
          </div>

          <div className='min-w-0'>
            <h4 className='mb-2 text-sm font-semibold'>
              {t('Withdrawal records')}
            </h4>
            <div className='mb-2 flex flex-wrap items-end gap-3'>
              <CommissionDateRangeFields
                idPrefix='commission-withdrawals'
                startDate={withdrawalStartDate}
                endDate={withdrawalEndDate}
                onStartDateChange={setWithdrawalStartDate}
                onEndDateChange={setWithdrawalEndDate}
              />
              <Button
                type='button'
                size='sm'
                onClick={() => {
                  setAppliedWithdrawalRange({
                    start: withdrawalStartDate,
                    end: withdrawalEndDate,
                  })
                  setWithdrawalPage(1)
                }}
              >
                <Search />
                {t('Search')}
              </Button>
            </div>
            <div className='rounded-lg border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Amount')}</TableHead>
                    <TableHead>{t('Network')}</TableHead>
                    <TableHead>{t('Address')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead>{t('Transaction hash')}</TableHead>
                    <TableHead>{t('Time')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {withdrawals.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={6}
                        className='text-muted-foreground text-center'
                      >
                        {t('No records')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    withdrawals.map((withdrawal) => (
                      <TableRow key={withdrawal.id}>
                        <TableCell>
                          {formatUsdt(withdrawal.actual_usdt_minor)}
                        </TableCell>
                        <TableCell>{withdrawal.network}</TableCell>
                        <TableCell className='max-w-40'>
                          <span className='block truncate'>
                            {withdrawal.address || '-'}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={withdrawalStatusVariant(withdrawal.status)}
                            className={
                              withdrawal.status === 'approved'
                                ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-600'
                                : undefined
                            }
                          >
                            {withdrawalStatusLabel(withdrawal.status, t)}
                          </Badge>
                          {withdrawal.status === 'rejected' &&
                          withdrawal.reject_reason ? (
                            <div className='text-muted-foreground text-xs'>
                              {withdrawal.reject_reason}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell className='max-w-40'>
                          {withdrawal.status === 'approved' &&
                          withdrawal.tx_hash ? (
                            <a
                              href={`https://bscscan.com/tx/${encodeURIComponent(withdrawal.tx_hash)}`}
                              target='_blank'
                              rel='noopener noreferrer'
                              className='text-primary block truncate hover:underline'
                              title={withdrawal.tx_hash}
                            >
                              {withdrawal.tx_hash}
                            </a>
                          ) : (
                            '-'
                          )}
                        </TableCell>
                        <TableCell>
                          {formatTimestampToDate(withdrawal.created_at)}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
              <PageControls
                page={withdrawalPage}
                total={withdrawalsQuery.data?.data?.total ?? 0}
                pageSize={pageSize}
                onPageChange={setWithdrawalPage}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <Dialog open={transferDialogOpen} onOpenChange={setTransferDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Transfer to Balance')}</DialogTitle>
            <DialogDescription>
              {t('Transfer commission balance confirmation', {
                amount: formatQuota(balance?.transferable_quota ?? 0),
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setTransferDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              onClick={() => transferMutation.mutate()}
              disabled={transferMutation.isPending}
            >
              {t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={withdrawalDialogOpen}
        onOpenChange={(open) => {
          setWithdrawalDialogOpen(open)
          if (!open) setWithdrawalAddress('')
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Withdraw commission')}</DialogTitle>
            <DialogDescription>
              {t('Withdrawal will be sent to the configured BSC address.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-2'>
            <div className='bg-muted/30 grid gap-2 rounded-md border p-3 text-sm sm:grid-cols-3'>
              <div>
                <div className='text-muted-foreground text-xs'>
                  {t('Withdrawable USDT')}
                </div>
                <div className='font-medium'>
                  {formatUsdt(balance?.withdrawable_usdt_minor ?? 0)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground text-xs'>
                  {t('Withdrawal fee')}
                </div>
                <div className='font-medium'>
                  {formatUsdt(balance?.fee_usdt_minor ?? 0)}
                </div>
              </div>
              <div>
                <div className='text-muted-foreground text-xs'>
                  {t('Actual arrival')}
                </div>
                <div className='font-medium'>
                  {formatUsdt(balance?.actual_usdt_minor ?? 0)}
                </div>
              </div>
            </div>
            <Label htmlFor='withdrawal-address'>
              {t('Web3 wallet address')}
            </Label>
            <Input
              id='withdrawal-address'
              value={withdrawalAddress}
              onChange={(event) => setWithdrawalAddress(event.target.value)}
              placeholder='0x...'
              autoComplete='off'
            />
            <p className='text-warning text-xs'>
              {t('Withdrawal address warning')}
            </p>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setWithdrawalDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              onClick={() => withdrawalMutation.mutate(withdrawalAddress)}
              disabled={withdrawalMutation.isPending}
            >
              {t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
