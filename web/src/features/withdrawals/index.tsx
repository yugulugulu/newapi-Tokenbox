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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Search, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import {
  approveWithdrawalRequest,
  getWithdrawalRequests,
  rejectWithdrawalRequest,
} from '@/features/commissions/api'
import { CommissionDateRangeFields } from '@/features/commissions/components/commission-date-range-fields'
import { getCommissionTimeRange } from '@/features/commissions/lib/time-range'
import type { WithdrawalRequestView } from '@/features/commissions/types'
import { WithdrawalStatusBadge } from '@/features/withdrawals/components/withdrawal-status-badge'
import { formatTimestampToDate } from '@/lib/format'

const PAGE_SIZE = 20

function formatUsdt(minor: number) {
  return `${new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 2,
  }).format(minor / 100)} USDT`
}

export function Withdrawals() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [appliedKeyword, setAppliedKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [appliedDateRange, setAppliedDateRange] = useState({
    start: '',
    end: '',
  })
  const [page, setPage] = useState(1)
  const [approveTarget, setApproveTarget] =
    useState<WithdrawalRequestView | null>(null)
  const [rejectTarget, setRejectTarget] =
    useState<WithdrawalRequestView | null>(null)
  const [txHash, setTxHash] = useState('')
  const [rejectReason, setRejectReason] = useState('')

  const { startTime, endTime } = getCommissionTimeRange(
    appliedDateRange.start,
    appliedDateRange.end
  )

  const query = useQuery({
    queryKey: [
      'withdrawal-requests',
      page,
      appliedKeyword,
      status,
      startTime,
      endTime,
    ],
    queryFn: () =>
      getWithdrawalRequests({
        page,
        pageSize: PAGE_SIZE,
        keyword: appliedKeyword,
        status,
        startTime,
        endTime,
      }),
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['withdrawal-requests'] })

  const approveMutation = useMutation({
    mutationFn: ({ id, txHash }: { id: number; txHash: string }) =>
      approveWithdrawalRequest(id, txHash),
    onSuccess: () => {
      toast.success(t('Withdrawal approved'))
      setApproveTarget(null)
      setTxHash('')
      void invalidate()
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const rejectMutation = useMutation({
    mutationFn: ({ id, reason }: { id: number; reason: string }) =>
      rejectWithdrawalRequest(id, reason),
    onSuccess: () => {
      toast.success(t('Withdrawal rejected'))
      setRejectTarget(null)
      setRejectReason('')
      void invalidate()
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const items = query.data?.data?.items ?? []
  const total = query.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Withdrawal Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='grid gap-4'>
            <div className='flex flex-wrap items-end gap-3'>
              <div className='grid w-full max-w-sm gap-1'>
                <Label>{t('Search')}</Label>
                <Input
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      setAppliedKeyword(keyword)
                      setAppliedDateRange({ start: startDate, end: endDate })
                      setPage(1)
                    }
                  }}
                  placeholder={t('Email, user, or UID')}
                />
              </div>
              <div className='grid gap-1'>
                <Label>{t('Status')}</Label>
                <NativeSelect
                  value={status}
                  onChange={(event) => {
                    setStatus(event.target.value)
                    setPage(1)
                  }}
                >
                  <NativeSelectOption value=''>{t('All')}</NativeSelectOption>
                  <NativeSelectOption value='pending'>
                    {t('Pending review')}
                  </NativeSelectOption>
                  <NativeSelectOption value='approved'>
                    {t('Approved')}
                  </NativeSelectOption>
                  <NativeSelectOption value='rejected'>
                    {t('Rejected')}
                  </NativeSelectOption>
                </NativeSelect>
              </div>
              <CommissionDateRangeFields
                idPrefix='withdrawal-requests'
                startDate={startDate}
                endDate={endDate}
                onStartDateChange={setStartDate}
                onEndDateChange={setEndDate}
              />
              <Button
                size='sm'
                variant='outline'
                onClick={() => {
                  setAppliedKeyword(keyword)
                  setAppliedDateRange({ start: startDate, end: endDate })
                  setPage(1)
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
                    <TableHead>{t('User')}</TableHead>
                    <TableHead>{t('Withdrawal amount')}</TableHead>
                    <TableHead>{t('Fee')}</TableHead>
                    <TableHead>{t('Actual arrival')}</TableHead>
                    <TableHead>{t('Network')}</TableHead>
                    <TableHead>{t('Receiving address')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead>{t('Application time')}</TableHead>
                    <TableHead>{t('Actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={9}
                        className='text-muted-foreground text-center'
                      >
                        {t('No records')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    items.map((request) => (
                      <TableRow key={request.id}>
                        <TableCell>
                          <div className='font-medium'>
                            {request.display_name ||
                              request.username ||
                              request.email ||
                              '-'}
                          </div>
                          <div className='text-muted-foreground text-xs'>
                            {request.email || request.username || ''}
                          </div>
                          <div className='text-muted-foreground text-xs'>
                            UID {request.user_id}
                          </div>
                        </TableCell>
                        <TableCell>
                          {formatUsdt(request.amount_usdt_minor)}
                        </TableCell>
                        <TableCell>
                          {formatUsdt(request.fee_usdt_minor)}
                        </TableCell>
                        <TableCell>
                          {formatUsdt(request.actual_usdt_minor)}
                        </TableCell>
                        <TableCell>{request.network}</TableCell>
                        <TableCell className='max-w-44'>
                          <span className='block truncate'>
                            {request.address || '-'}
                          </span>
                        </TableCell>
                        <TableCell>
                          <WithdrawalStatusBadge status={request.status} />
                          {request.status === 'rejected' &&
                          request.reject_reason ? (
                            <div className='text-muted-foreground text-xs'>
                              {request.reject_reason}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          {formatTimestampToDate(request.created_at)}
                        </TableCell>
                        <TableCell>
                          {request.status === 'pending' ? (
                            <div className='flex gap-2'>
                              <Button
                                size='sm'
                                variant='outline'
                                onClick={() => setApproveTarget(request)}
                              >
                                <Check />
                                {t('Approve')}
                              </Button>
                              <Button
                                size='sm'
                                variant='destructive'
                                onClick={() => setRejectTarget(request)}
                              >
                                <X />
                                {t('Reject')}
                              </Button>
                            </div>
                          ) : null}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>

            <div className='flex items-center justify-end gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={page <= 1}
                onClick={() => setPage((value) => value - 1)}
              >
                {t('Previous')}
              </Button>
              <span className='text-sm'>
                {page} / {totalPages}
              </span>
              <Button
                variant='outline'
                size='sm'
                disabled={page >= totalPages}
                onClick={() => setPage((value) => value + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <Dialog
        open={Boolean(approveTarget)}
        onOpenChange={(open) => {
          if (!open) setApproveTarget(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Approve withdrawal')}</DialogTitle>
            <DialogDescription>
              {t('Enter the on-chain transaction hash.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-2'>
            <Label htmlFor='withdrawal-tx-hash'>{t('Transaction hash')}</Label>
            <Input
              id='withdrawal-tx-hash'
              value={txHash}
              onChange={(event) => setTxHash(event.target.value)}
              placeholder='0x...'
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setApproveTarget(null)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={() =>
                approveTarget &&
                approveMutation.mutate({ id: approveTarget.id, txHash })
              }
              disabled={approveMutation.isPending || !txHash.trim()}
            >
              {t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(rejectTarget)}
        onOpenChange={(open) => {
          if (!open) setRejectTarget(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Reject withdrawal')}</DialogTitle>
            <DialogDescription>
              {t('Enter the rejection reason shown to the user.')}
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-2'>
            <Label htmlFor='withdrawal-reject-reason'>
              {t('Rejection reason')}
            </Label>
            <Textarea
              id='withdrawal-reject-reason'
              value={rejectReason}
              onChange={(event) => setRejectReason(event.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setRejectTarget(null)}>
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={() =>
                rejectTarget &&
                rejectMutation.mutate({
                  id: rejectTarget.id,
                  reason: rejectReason,
                })
              }
              disabled={rejectMutation.isPending || !rejectReason.trim()}
            >
              {t('Confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
