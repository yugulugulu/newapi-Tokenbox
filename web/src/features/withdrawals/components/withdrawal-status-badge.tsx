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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import type { WithdrawalRequestView } from '@/features/commissions/types'

interface WithdrawalStatusBadgeProps {
  status: WithdrawalRequestView['status']
}

export function WithdrawalStatusBadge(props: WithdrawalStatusBadgeProps) {
  const { t } = useTranslation()

  if (props.status === 'approved') {
    return (
      <Badge
        variant='outline'
        className='border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
        data-withdrawal-status='approved'
      >
        {t('Approved')}
      </Badge>
    )
  }

  if (props.status === 'rejected') {
    return (
      <Badge
        variant='outline'
        className='border-destructive/40 bg-destructive/10 text-destructive'
        data-withdrawal-status='rejected'
      >
        {t('Rejected')}
      </Badge>
    )
  }

  return (
    <Badge
      variant='outline'
      className='border-warning/40 bg-warning/10 text-warning'
      data-withdrawal-status='pending'
    >
      {t('Pending review')}
    </Badge>
  )
}
