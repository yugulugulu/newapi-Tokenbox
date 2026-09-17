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
import { ArrowUpRight, MessageCircleMore, User } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'

import { TokenBoxMascot } from '../tokenbox-mascot'

const QQ_GROUP_URL = 'mqqwpa://im/chat?chat_type=group&uin=1048793882'
const COMMUNITY_FEEDBACKS = [
  {
    key: 'api',
    translationKey: 'home.tokenbox.community.feedback1',
    authorKey: 'home.tokenbox.community.feedback1Author',
    avatarClassName: 'bg-gradient-to-br from-blue-500 to-blue-600',
  },
  {
    key: 'models',
    translationKey: 'home.tokenbox.community.feedback2',
    authorKey: 'home.tokenbox.community.feedback2Author',
    avatarClassName: 'bg-gradient-to-br from-violet-500 to-purple-600',
  },
  {
    key: 'image',
    translationKey: 'home.tokenbox.community.feedback3',
    authorKey: 'home.tokenbox.community.feedback3Author',
    avatarClassName: 'bg-gradient-to-br from-amber-500 to-orange-600',
  },
] as const

export function TokenBoxCommunity() {
  const { t } = useTranslation()

  return (
    <section className='px-5 pb-20 sm:px-8 lg:px-10'>
      <AnimateInView className='bg-muted/70 border-border dark:bg-card relative mx-auto max-w-7xl overflow-hidden rounded-2xl border p-7 sm:p-10 lg:p-12'>
        <div
          aria-hidden
          className='border-primary/10 absolute top-10 right-[20%] size-20 rotate-12 rounded-2xl border-[14px]'
        />
        <div
          aria-hidden
          className='absolute right-[6%] bottom-10 size-12 -rotate-12 rounded-xl border-[9px] border-emerald-400/15'
        />

        <div className='relative z-10 max-w-2xl'>
          <h2 className='text-3xl font-black sm:text-4xl'>
            {t('home.tokenbox.community.title')}
          </h2>
          <p className='text-muted-foreground mt-3 text-base leading-7 sm:text-lg'>
            {t('home.tokenbox.community.description')}
          </p>
        </div>

        <div className='relative z-10 mt-10 grid items-stretch gap-8 lg:grid-cols-[minmax(0,1fr)_320px]'>
          <div className='bg-background border-border dark:bg-muted/60 rounded-2xl border p-6 shadow-sm'>
            <div className='mb-6 flex items-center gap-2 text-base font-bold'>
              <MessageCircleMore
                className='text-primary size-5.5'
                aria-hidden='true'
              />
              {t('home.tokenbox.community.feedbackTitle')}
            </div>
            <div className='space-y-4'>
              {COMMUNITY_FEEDBACKS.map((feedback) => (
                <div
                  key={feedback.key}
                  className='bg-muted/40 border-border/50 group flex gap-3.5 rounded-xl border p-4 shadow-sm transition-all hover:scale-[1.01] hover:shadow-md'
                >
                  <span
                    className={`flex size-11 shrink-0 items-center justify-center rounded-full text-white shadow-md ${feedback.avatarClassName}`}
                    aria-hidden='true'
                  >
                    <User className='size-5' />
                  </span>
                  <div className='min-w-0 flex-1'>
                    <div className='mb-2 text-xs font-semibold'>
                      {t(feedback.authorKey)}
                    </div>
                    <p className='text-foreground/90 text-sm leading-relaxed font-medium'>
                      「{t(feedback.translationKey)}」
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className='bg-background border-border dark:bg-muted/60 relative flex min-h-[380px] flex-col items-center justify-end rounded-2xl border px-5 py-6 shadow-sm'>
            <div className='absolute top-6 right-4 z-10 w-[calc(100%-32px)]'>
              <div className='border-primary/20 bg-primary/10 text-foreground relative ml-auto max-w-[210px] rounded-2xl rounded-br-md border px-3.5 py-2.5 text-center text-xs leading-5 font-bold shadow-sm'>
                {t('home.tokenbox.community.qrCallout')}
                <span className='border-primary/20 absolute -right-2 bottom-[-8px] size-4 rotate-45 border-r border-b bg-[color-mix(in_oklab,var(--primary)_10%,transparent)]' />
              </div>
            </div>
            <div className='pointer-events-none absolute top-[-12px] right-[-6px] w-[90px] rotate-6'>
              <TokenBoxMascot />
            </div>
            <a
              href={QQ_GROUP_URL}
              target='_blank'
              rel='noreferrer'
              aria-label={t('home.tokenbox.community.qrLabel')}
              className='overflow-hidden rounded-lg bg-white p-3.5 shadow-lg ring-1 ring-black/5'
            >
              <img
                src='/images/tokenbox-qq-group.png'
                alt='TokenBox QQ群'
                className='h-[160px] w-[160px] object-contain'
              />
            </a>
            <Button
              className='mt-4 h-11 w-full rounded-xl'
              render={
                <a href={QQ_GROUP_URL} target='_blank' rel='noreferrer' />
              }
            >
              {t('home.tokenbox.community.join')}
              <ArrowUpRight className='size-4' aria-hidden='true' />
            </Button>
          </div>
        </div>
      </AnimateInView>
    </section>
  )
}
