import { useQuery } from '@tanstack/react-query'
import { Share2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  getCommissionSelf,
  getCommissionSelfRecords,
  getCommissionSelfReferrals,
} from '@/features/commissions/api'
import { formatTimestampToDate } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

function formatMoney(minor: number, currency: string) {
  const zeroDecimal = ['IDR', 'JPY', 'KRW', 'VND'].includes(currency)
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: zeroDecimal ? 0 : 2,
  }).format(minor / (zeroDecimal ? 1 : 100))
}

export function CommissionWalletCard() {
  const { t } = useTranslation()
  const role = useAuthStore((state) => state.auth.user?.role)
  const roleEligible = role === ROLE.AGENT || role === ROLE.ADMIN
  const selfQuery = useQuery({
    queryKey: ['commission-self'],
    queryFn: getCommissionSelf,
    enabled: roleEligible,
    retry: false,
  })
  const eligible = selfQuery.isSuccess && Boolean(selfQuery.data?.data)
  const recordsQuery = useQuery({
    queryKey: ['commission-self-records'],
    queryFn: () => getCommissionSelfRecords(1, 5),
    enabled: eligible,
    retry: false,
  })
  const referralsQuery = useQuery({
    queryKey: ['commission-self-referrals'],
    queryFn: () => getCommissionSelfReferrals(1, 5),
    enabled: eligible,
    retry: false,
  })
  if (!roleEligible || !eligible) return null

  const self = selfQuery.data?.data
  const records = recordsQuery.data?.data?.items ?? []
  const referrals = referralsQuery.data?.data?.items ?? []
  let rateSource = t('None')
  if (self?.rate_source === 'agent') {
    rateSource = t('Individual')
  } else if (self?.rate_source === 'global') {
    rateSource = t('Global')
  }

  return (
    <Card data-card-hover='false'>
      <CardHeader className='border-b'>
        <CardTitle className='flex items-center gap-2 text-sm'>
          <Share2 className='size-4' />
          {t('Commission and referrals')}
        </CardTitle>
      </CardHeader>
      <CardContent className='grid gap-4 p-4'>
        <div className='grid gap-3 sm:grid-cols-3'>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Current commission rate')}
            </div>
            <div className='font-semibold'>{self?.rate_percent ?? 0}%</div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Rate source')}
            </div>
            <div className='font-semibold'>
              {rateSource}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>{t('Invites')}</div>
            <div className='font-semibold'>
              {referralsQuery.data?.data?.total ?? 0}
            </div>
          </div>
        </div>
        <div className='grid gap-4 lg:grid-cols-2'>
          <div className='min-w-0'>
            <h4 className='mb-2 text-sm font-semibold'>
              {t('Commission records')}
            </h4>
            <div className='divide-y rounded-lg border text-sm'>
              {records.length === 0 ? (
                <div className='text-muted-foreground p-3'>
                  {t('No records')}
                </div>
              ) : (
                records.map((record) => (
                  <div
                    key={record.id}
                    className='flex items-center justify-between gap-3 p-3'
                  >
                    <span className='truncate'>
                      {record.descendant_name ||
                        record.descendant_username ||
                        `UID ${record.descendant_user_id}`}
                    </span>
                    <span className='shrink-0 font-medium'>
                      {formatMoney(
                        record.commission_amount_minor,
                        record.payment_currency
                      )}
                    </span>
                  </div>
                ))
              )}
            </div>
          </div>
          <div className='min-w-0'>
            <h4 className='mb-2 text-sm font-semibold'>
              {t('Invitation records')}
            </h4>
            <div className='divide-y rounded-lg border text-sm'>
              {referrals.length === 0 ? (
                <div className='text-muted-foreground p-3'>
                  {t('No records')}
                </div>
              ) : (
                referrals.map((referral) => (
                  <div
                    key={referral.id}
                    className='flex items-center justify-between gap-3 p-3'
                  >
                    <span className='truncate'>
                      {referral.display_name || referral.username}
                    </span>
                    <span className='text-muted-foreground shrink-0 text-xs'>
                      {formatTimestampToDate(referral.created_at)}
                    </span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
