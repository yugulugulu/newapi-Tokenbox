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
import { useQuery } from '@tanstack/react-query'
import { Bell, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'
import { getNotice } from '@/lib/api'
import { useNotificationStore } from '@/stores/notification-store'

// The dialog greets a visitor once per page load: navigating away from the
// home page and back inside the SPA must not pop it up again.
let shownThisPageLoad = false

const CARD_BACKGROUND = [
  'radial-gradient(90% 120% at 0% 100%, rgba(58,108,255,0.95) 0%, rgba(44,82,235,0.55) 30%, transparent 62%)',
  'radial-gradient(60% 60% at 8% 0%, rgba(96,150,255,0.35) 0%, transparent 70%)',
  'radial-gradient(50% 50% at 100% 100%, rgba(40,90,255,0.28) 0%, transparent 70%)',
  'linear-gradient(135deg, #0b1437 0%, #070b1f 55%, #060917 100%)',
].join(', ')

function MegaphoneArt() {
  return (
    <svg
      viewBox='0 0 240 240'
      fill='none'
      aria-hidden='true'
      className='h-full w-full drop-shadow-[0_18px_30px_rgba(10,20,80,0.55)]'
    >
      <defs>
        <linearGradient id='home-notice-body' x1='0' y1='0' x2='1' y2='1'>
          <stop offset='0' stopColor='#ffffff' />
          <stop offset='1' stopColor='#c9d7ff' />
        </linearGradient>
        <linearGradient id='home-notice-blue' x1='0' y1='0' x2='1' y2='1'>
          <stop offset='0' stopColor='#6aa5ff' />
          <stop offset='1' stopColor='#2b62f0' />
        </linearGradient>
        <linearGradient id='home-notice-inner' x1='0' y1='0' x2='1' y2='1'>
          <stop offset='0' stopColor='#f4f7ff' />
          <stop offset='1' stopColor='#b9c9fb' />
        </linearGradient>
        <linearGradient id='home-notice-amber' x1='0' y1='0' x2='1' y2='1'>
          <stop offset='0' stopColor='#ffd166' />
          <stop offset='1' stopColor='#ff9f1c' />
        </linearGradient>
        <radialGradient id='home-notice-orbit' cx='0.5' cy='0.5' r='0.5'>
          <stop offset='0.55' stopColor='#7fa8ff' stopOpacity='0' />
          <stop offset='1' stopColor='#7fa8ff' stopOpacity='0.55' />
        </radialGradient>
      </defs>

      <ellipse
        cx='116'
        cy='156'
        rx='100'
        ry='42'
        transform='rotate(16 116 156)'
        fill='url(#home-notice-orbit)'
      />

      <g className='origin-center motion-safe:animate-[home-notice-float_5s_ease-in-out_infinite]'>
        <g transform='rotate(-18 120 120)'>
          <rect
            x='94'
            y='136'
            width='28'
            height='60'
            rx='13'
            transform='rotate(14 108 166)'
            fill='url(#home-notice-body)'
          />
          <rect
            x='44'
            y='92'
            width='34'
            height='48'
            rx='13'
            fill='url(#home-notice-blue)'
          />
          <path
            d='M68 96 L158 56 Q172 116 158 176 L68 136 Z'
            fill='url(#home-notice-body)'
          />
          <ellipse
            cx='160'
            cy='116'
            rx='27'
            ry='63'
            fill='url(#home-notice-blue)'
          />
          <ellipse
            cx='163'
            cy='116'
            rx='18'
            ry='50'
            fill='url(#home-notice-inner)'
          />
          <ellipse
            cx='159'
            cy='116'
            rx='8'
            ry='13'
            fill='url(#home-notice-amber)'
          />
        </g>
        <g
          stroke='url(#home-notice-amber)'
          strokeWidth='9'
          strokeLinecap='round'
          className='motion-safe:animate-[home-notice-pulse_1.8s_ease-in-out_infinite]'
        >
          <path d='M193 44 L201 26' />
          <path d='M207 66 L225 54' />
          <path d='M213 92 L233 89' />
        </g>
      </g>

      <path
        d='M40 62 Q42 76 56 78 Q42 80 40 94 Q38 80 24 78 Q38 76 40 62 Z'
        fill='#5b93ff'
        className='origin-[40px_78px] motion-safe:animate-[home-notice-pulse_2.6s_ease-in-out_infinite]'
      />
      <circle cx='196' cy='196' r='6' fill='#4f86ff' />
    </svg>
  )
}

/**
 * Announcement card shown when the home page opens. Its content is the notice
 * configured in the admin console (same source as the header bell).
 */
export function HomeNoticeDialog() {
  const { t } = useTranslation()
  const { data } = useQuery({
    queryKey: ['notice'],
    queryFn: getNotice,
    staleTime: 1000 * 60 * 5,
  })
  const notice = data?.success ? (data.data || '').trim() : ''

  const { lastReadNotice, isNoticeClosed, markNoticeRead, setClosedUntilDate } =
    useNotificationStore()
  const [dismissed, setDismissed] = useState(shownThisPageLoad)

  // "Do not show again today" only silences the notice it was clicked on, so
  // an edited notice shows up again the same day.
  const closedForToday = isNoticeClosed() && lastReadNotice === notice
  const open = Boolean(notice) && !dismissed && !closedForToday

  const dismiss = (untilTomorrow: boolean) => {
    shownThisPageLoad = true
    setDismissed(true)
    markNoticeRead(notice)
    if (untilTomorrow) setClosedUntilDate(new Date().toDateString())
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) dismiss(false)
      }}
    >
      <DialogContent
        showCloseButton={false}
        className='dark block overflow-hidden rounded-[28px] border-0 p-0 text-slate-100 shadow-[0_30px_80px_-10px_rgba(20,50,180,0.55)] ring-white/10 duration-200 sm:max-w-[760px]'
        style={{ background: CARD_BACKGROUND }}
      >
        <DialogClose
          render={
            <Button
              variant='ghost'
              size='icon'
              className='absolute top-4 right-4 z-10 rounded-full text-slate-300 hover:bg-white/10 hover:text-white'
            />
          }
        >
          <X className='size-5' />
          <span className='sr-only'>{t('Close')}</span>
        </DialogClose>

        <div className='grid sm:grid-cols-[260px_minmax(0,1fr)]'>
          <div className='flex items-center justify-center px-6 pt-8 sm:py-10 sm:pr-0 sm:pl-8'>
            <div className='size-28 sm:size-56'>
              <MegaphoneArt />
            </div>
          </div>

          <div className='flex min-w-0 flex-col px-6 pt-4 pb-6 sm:px-9 sm:pt-11 sm:pb-9'>
            <div className='inline-flex w-fit items-center gap-1.5 rounded-full bg-[#2f6bff]/25 px-3 py-1 text-xs font-medium text-white shadow-[0_0_18px_rgba(47,107,255,0.55)] ring-1 ring-[#5b93ff]/70'>
              <Bell className='size-3.5 fill-current' />
              {t('Announcements')}
            </div>

            <DialogTitle className='mt-4 text-3xl leading-tight font-bold tracking-tight text-white sm:text-[2.5rem]'>
              {t('System Notice')}
            </DialogTitle>
            <DialogDescription className='sr-only'>
              {t('Latest platform updates and notices')}
            </DialogDescription>

            <div className='mt-4 max-h-[min(42vh,320px)] [scrollbar-width:thin] [scrollbar-color:rgba(148,163,184,0.5)_transparent] overflow-y-auto pr-2 text-[15px] leading-7 text-slate-300'>
              <RichContent breaks content={notice} />
            </div>

            <div className='mt-7 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end'>
              <Button
                type='button'
                variant='outline'
                className='h-11 rounded-full border-white/20 bg-transparent px-6 text-sm text-slate-100 hover:bg-white/10 hover:text-white'
                onClick={() => dismiss(true)}
              >
                {t('Do not show again today')}
              </Button>
              <Button
                type='button'
                className='h-11 rounded-full border-0 bg-gradient-to-r from-[#2f7bff] to-[#55a2ff] px-7 text-sm font-semibold text-white shadow-[0_10px_28px_rgba(47,123,255,0.5)] hover:from-[#3d86ff] hover:to-[#68adff]'
                onClick={() => dismiss(false)}
              >
                {t('Got it')}
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
