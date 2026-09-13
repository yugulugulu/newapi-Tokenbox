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
import { CalendarDays, X } from 'lucide-react'
import { useState } from 'react'
import { enUS, fr, ja, ru, vi, zhCN } from 'react-day-picker/locale'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

interface CommissionDateRangeFieldsProps {
  idPrefix: string
  startDate: string
  endDate: string
  onStartDateChange: (value: string) => void
  onEndDateChange: (value: string) => void
  className?: string
}

const calendarLocales = {
  en: enUS,
  zh: zhCN,
  fr,
  ru,
  ja,
  vi,
} as const

function parseDate(value: string): Date | undefined {
  if (!value) return undefined
  const [year, month, day] = value.split('-').map(Number)
  if (!year || !month || !day) return undefined

  const date = new Date(year, month - 1, day)
  return date.getFullYear() === year &&
    date.getMonth() === month - 1 &&
    date.getDate() === day
    ? date
    : undefined
}

interface CommissionDateFieldProps {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
}

function CommissionDateField(props: CommissionDateFieldProps) {
  const { t, i18n } = useTranslation()
  const [open, setOpen] = useState(false)
  const selectedDate = parseDate(props.value)
  const calendarLocale =
    calendarLocales[i18n.language as keyof typeof calendarLocales] ?? enUS
  const currentYear = new Date().getFullYear()
  const clearLabel = `${t('Clear')}: ${props.label}`

  return (
    <div className='grid gap-1'>
      <Label htmlFor={props.id}>{props.label}</Label>
      <div className='flex items-center gap-1'>
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger
            render={
              <Button
                id={props.id}
                type='button'
                variant='outline'
                className={cn(
                  'h-8 w-[190px] justify-between px-2.5 text-xs font-normal',
                  !selectedDate && 'text-muted-foreground'
                )}
                aria-label={props.label}
              />
            }
          >
            <span>
              {selectedDate
                ? dayjs(selectedDate).format('YYYY-MM-DD')
                : t('Pick a date')}
            </span>
            <CalendarDays className='size-3.5 opacity-70' />
          </PopoverTrigger>
          <PopoverContent className='w-auto overflow-hidden p-0' align='start'>
            <Calendar
              mode='single'
              selected={selectedDate}
              onSelect={(date) => {
                props.onChange(date ? dayjs(date).format('YYYY-MM-DD') : '')
                setOpen(false)
              }}
              locale={calendarLocale}
              captionLayout='dropdown'
              startMonth={new Date(currentYear - 100, 0)}
              endMonth={new Date(currentYear + 100, 11)}
            />
            <div className='border-border border-t p-2'>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='w-full justify-start'
                disabled={!props.value}
                onClick={() => {
                  props.onChange('')
                  setOpen(false)
                }}
                aria-label={clearLabel}
              >
                {t('Clear')}
              </Button>
            </div>
          </PopoverContent>
        </Popover>
        <Button
          type='button'
          variant='ghost'
          size='icon-xs'
          disabled={!props.value}
          onClick={() => props.onChange('')}
          aria-label={clearLabel}
          title={clearLabel}
        >
          <X />
        </Button>
      </div>
    </div>
  )
}

export function CommissionDateRangeFields(
  props: CommissionDateRangeFieldsProps
) {
  const { t } = useTranslation()

  return (
    <div className={cn('flex flex-wrap items-end gap-3', props.className)}>
      <CommissionDateField
        id={`${props.idPrefix}-start-date`}
        label={t('Start date')}
        value={props.startDate}
        onChange={props.onStartDateChange}
      />
      <CommissionDateField
        id={`${props.idPrefix}-end-date`}
        label={t('End date')}
        value={props.endDate}
        onChange={props.onEndDateChange}
      />
    </div>
  )
}
