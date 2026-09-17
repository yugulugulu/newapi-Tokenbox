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
import { getLobeIcon } from '@/lib/lobe-icon'

export function TokenBoxMascot() {
  return (
    <div
      className='relative h-auto w-full drop-shadow-[0_28px_24px_rgba(15,86,180,0.2)] dark:drop-shadow-[0_28px_24px_rgba(0,0,0,0.42)]'
      role='img'
      aria-label='TokenBox mascot holding OpenAI and Claude logos'
    >
      <svg
        viewBox='0 0 660 580'
        aria-hidden='true'
        className='block h-auto w-full'
      >
        <defs>
          <linearGradient id='box-front' x1='0' y1='0' x2='1' y2='1'>
            <stop stopColor='#35a7ff' />
            <stop offset='.5' stopColor='#087bff' />
            <stop offset='1' stopColor='#0052cc' />
          </linearGradient>
          <linearGradient id='box-side' x1='0' y1='0' x2='1' y2='1'>
            <stop stopColor='#0871ea' />
            <stop offset='1' stopColor='#003795' />
          </linearGradient>
          <linearGradient id='box-top' x1='0' y1='0' x2='1' y2='1'>
            <stop stopColor='#50b8ff' />
            <stop offset='1' stopColor='#087bff' />
          </linearGradient>
          <linearGradient id='gold' x1='0' y1='0' x2='1' y2='1'>
            <stop stopColor='#ffe574' />
            <stop offset='1' stopColor='#ffad19' />
          </linearGradient>
          <filter
            id='mascot-shadow'
            x='-30%'
            y='-30%'
            width='160%'
            height='180%'
          >
            <feDropShadow
              dx='0'
              dy='16'
              stdDeviation='14'
              floodColor='#002b70'
              floodOpacity='.22'
            />
          </filter>
        </defs>

        <ellipse
          cx='335'
          cy='542'
          rx='185'
          ry='25'
          fill='currentColor'
          opacity='.09'
        />

        <g opacity='.95'>
          <g transform='translate(48 104) rotate(-10 45 45)'>
            <rect width='90' height='90' rx='27' fill='#10b981' />
            <circle
              cx='45'
              cy='45'
              r='24'
              fill='none'
              stroke='#fff'
              strokeWidth='7'
            />
            <path
              d='M35 45h20M45 35v20'
              stroke='#fff'
              strokeWidth='7'
              strokeLinecap='round'
            />
          </g>
          <g transform='translate(496 75) rotate(9 48 48)'>
            <rect width='96' height='96' rx='29' fill='#ff6f61' />
            <text x='21' y='64' fill='#fff' fontSize='43' fontWeight='900'>
              AI
            </text>
          </g>
          <g transform='translate(552 220) rotate(13 38 38)'>
            <rect width='76' height='76' rx='23' fill='#7549ed' />
            <path
              d='m38 13 6 17 18 1-14 11 5 18-15-10-15 10 5-18-14-11 18-1Z'
              fill='#fff'
            />
          </g>
          <g transform='translate(77 252) rotate(-12 36 36)'>
            <rect width='72' height='72' rx='22' fill='#ffc42d' />
            <text x='19' y='51' fill='#6b4800' fontSize='38' fontWeight='900'>
              G
            </text>
          </g>
        </g>

        <g filter='url(#mascot-shadow)'>
          <path
            d='M320 112V67'
            stroke='#0967d6'
            strokeWidth='17'
            strokeLinecap='round'
          />
          <circle cx='320' cy='51' r='25' fill='url(#gold)' />
          <circle cx='311' cy='42' r='7' fill='#fff8c8' />

          <path
            d='M143 281c-64 3-90-41-56-87'
            fill='none'
            stroke='#aebdcc'
            strokeWidth='30'
            strokeLinecap='round'
          />
          <path
            d='M88 195c-22-18-21-47 3-56 17-7 32 3 39 20 8-18 27-24 41-12 15 13 10 36-6 49'
            fill='#f7fbff'
            stroke='#c9d7e5'
            strokeWidth='5'
          />
          <path
            d='M493 280c62 13 94-19 91-66'
            fill='none'
            stroke='#aebdcc'
            strokeWidth='30'
            strokeLinecap='round'
          />
          <path
            d='M582 211c0-28 19-48 43-41 17 5 24 21 17 37 17-8 33 0 36 17 4 19-14 34-34 35'
            fill='#f7fbff'
            stroke='#c9d7e5'
            strokeWidth='5'
          />
          <path d='m173 164 49-33h225l42 37-31 30H202Z' fill='url(#box-top)' />
          <path
            d='m202 181 256 1 43 54-27 202-45 45H198l-52-43-25-201Z'
            fill='url(#box-front)'
          />
          <path d='m121 239 81-58-1 288-55-29Z' fill='url(#box-side)' />
          <path d='m458 182 43 54-27 202-46 35 2-252Z' fill='#005bc9' />

          <rect
            x='213'
            y='226'
            width='227'
            height='133'
            rx='42'
            fill='#062552'
            stroke='#79d8ff'
            strokeWidth='8'
          />
          <path
            d='m266 286 28-20m-28 20 28 20m94-40 28 20m-28 20 28-20'
            stroke='#6aeaff'
            strokeWidth='13'
            strokeLinecap='round'
          />
          <path
            d='M320 323c13 10 27 10 40 0'
            fill='none'
            stroke='#fff'
            strokeWidth='7'
            strokeLinecap='round'
          />

          <text x='222' y='414' fill='#fff' fontSize='37' fontWeight='900'>
            TOKEN BOX
          </text>
          <path
            d='M176 277c3 15 12 24 27 27-15 3-24 12-27 27-3-15-12-24-27-27 15-3 24-12 27-27Z'
            fill='#ffe14d'
          />
          <circle cx='449' cy='400' r='21' fill='#ffcb3c' />
          <path
            d='M440 397c4 7 14 7 18 0'
            fill='none'
            stroke='#4b3b00'
            strokeWidth='4'
            strokeLinecap='round'
          />

          <path
            d='M216 466v56m201-56v56'
            stroke='#aebdcc'
            strokeWidth='27'
            strokeLinecap='round'
          />
          <path
            d='M167 515c42-28 88-14 99 31l-9 18H157c-20-8-14-37 10-49Z'
            fill='#f8fbff'
            stroke='#cbd9e7'
            strokeWidth='5'
          />
          <path
            d='M372 538c17-39 62-47 100-18 20 15 22 37 0 44h-97Z'
            fill='#f8fbff'
            stroke='#cbd9e7'
            strokeWidth='5'
          />
          <path d='M171 535h90m119 8h91' stroke='#ffcb36' strokeWidth='8' />
        </g>
      </svg>

      <div
        aria-label='OpenAI'
        className='absolute top-[23.6%] left-[14.1%] flex size-[10%] rotate-[-8deg] items-center justify-center rounded-[28%] border-[3px] border-white bg-white shadow-[0_9px_15px_rgba(17,24,39,0.22)] sm:border-4 [&>svg]:size-[72%]'
      >
        {getLobeIcon('OpenAI.Color', 64)}
      </div>
      <div
        aria-label='Claude'
        className='absolute top-[28.2%] left-[86.8%] flex size-[10.6%] rotate-[8deg] items-center justify-center rounded-[28%] border-[3px] border-white bg-white shadow-[0_9px_15px_rgba(17,24,39,0.22)] sm:border-4 [&>svg]:size-[72%]'
      >
        {getLobeIcon('Claude.Color', 64)}
      </div>
    </div>
  )
}
