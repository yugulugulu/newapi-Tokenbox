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
import {
  BILLING_CACHE_VAR_MAP,
  SEEDANCE_UNSUPPORTED_RESOLUTION_TIER,
} from './billing-expr'

export const CACHE_MODE_TIMED = 'timed'
export const CACHE_MODE_GENERIC = 'generic'
export type CacheMode = typeof CACHE_MODE_TIMED | typeof CACHE_MODE_GENERIC

export type TierConditionInput = {
  var: 'p' | 'c' | 'len'
  op: '<' | '<=' | '>' | '>='
  value: number | string
}

export type VisualTier = {
  label: string
  conditions: TierConditionInput[]
  input_unit_cost: number
  output_unit_cost: number
  cache_mode: CacheMode
  cache_read_unit_cost?: number
  cache_create_unit_cost?: number
  cache_create_1h_unit_cost?: number
  image_unit_cost?: number
  image_output_unit_cost?: number
  audio_input_unit_cost?: number
  audio_output_unit_cost?: number
  [field: string]: unknown
}

export type VisualConfig = {
  tiers: VisualTier[]
}

export type SeedanceBillingMethod = 'per_second' | 'per_call'
export type SeedanceVideoInput = 'with_video' | 'without_video'

export type SeedanceTier = {
  label: string
  resolution?: string
  method: SeedanceBillingMethod
  videoInput?: SeedanceVideoInput
  price: number
  videoInputPrice?: number
  fallback: boolean
}

export type SeedanceConfig = {
  tiers: SeedanceTier[]
}

function isSeedanceUnsupportedResolutionTier(tier: SeedanceTier): boolean {
  return (
    tier.label === SEEDANCE_UNSUPPORTED_RESOLUTION_TIER &&
    tier.method === 'per_call' &&
    tier.price === 0 &&
    tier.resolution === undefined &&
    tier.videoInput === undefined &&
    tier.videoInputPrice === undefined
  )
}

function normalizeSeedancePrice(value: unknown): number {
  const price = Number(value)
  if (!Number.isFinite(price) || price < 0) return 0
  return Math.round(price * 1_000_000) / 1_000_000
}

export function normalizeSeedanceConfig(
  config: SeedanceConfig | null | undefined
): SeedanceConfig {
  const configuredTiers = Array.isArray(config?.tiers)
    ? config.tiers.filter((tier) => !tier.fallback)
    : []
  const tiers =
    configuredTiers.length > 0
      ? configuredTiers
      : [
          {
            label: '480p',
            resolution: '480p',
            method: 'per_second' as const,
            videoInput: 'without_video' as const,
            price: 0,
            fallback: false,
          },
        ]
  return {
    tiers: tiers.map((tier) => {
      let videoInput: SeedanceVideoInput | undefined
      if (tier.method !== 'per_call') {
        videoInput =
          tier.videoInput === 'with_video' ? 'with_video' : 'without_video'
      }
      const normalizedTier: SeedanceTier = {
        label: String(tier.label ?? ''),
        resolution: String(tier.resolution ?? '').trim(),
        method: tier.method === 'per_call' ? 'per_call' : 'per_second',
        videoInput,
        price: normalizeSeedancePrice(tier.price),
        fallback: false,
      }
      if (
        tier.videoInputPrice !== undefined &&
        tier.method !== 'per_call' &&
        videoInput === 'with_video'
      ) {
        normalizedTier.videoInputPrice = normalizeSeedancePrice(
          tier.videoInputPrice
        )
      }
      return normalizedTier
    }),
  }
}

function formatSeedancePrice(value: number): string {
  const normalized = normalizeSeedancePrice(value)
  return normalized.toFixed(6).replace(/0+$/, '').replace(/\.$/, '') || '0'
}

export function createDefaultSeedanceConfig(): SeedanceConfig {
  return normalizeSeedanceConfig({
    tiers: [
      {
        label: '480p',
        resolution: '480p',
        method: 'per_second',
        videoInput: 'without_video',
        price: 0,
        fallback: false,
      },
    ],
  })
}

export function generateSeedanceExpr(
  config: SeedanceConfig | null | undefined
): string {
  const normalized = normalizeSeedanceConfig(config)
  const parts = normalized.tiers.map((tier, index) => {
    const label = JSON.stringify(tier.label || `tier_${index + 1}`)
    let charge = `charge(${JSON.stringify(tier.method)}, quantity, ${formatSeedancePrice(tier.price)})`
    if (
      tier.method === 'per_second' &&
      tier.videoInput === 'with_video' &&
      tier.videoInputPrice !== undefined
    ) {
      charge += ` + charge("per_second", video_input_durations, ${formatSeedancePrice(tier.videoInputPrice)})`
    }
    const body = `tier(${label}, ${charge})`
    let condition = `param("resolution") == ${JSON.stringify((tier.resolution || '').trim())}`
    if (tier.method === 'per_second') {
      const mediaCondition =
        tier.videoInput === 'with_video'
          ? 'has_media("video")'
          : '!has_media("video")'
      condition = `${condition} && ${mediaCondition}`
    }
    return `${condition} ? ${body}`
  })
  parts.push(
    `tier(${JSON.stringify(SEEDANCE_UNSUPPORTED_RESOLUTION_TIER)}, charge("per_call", quantity, 0))`
  )
  return `v2:${parts.join(' : ')}`
}

export function tryParseSeedanceExpr(
  exprStr: string | null | undefined
): SeedanceConfig | null {
  if (!exprStr || !exprStr.startsWith('v2:')) {
    return null
  }
  const body = exprStr.slice(3).trim()
  const branchRe = new RegExp(
    String.raw`(?:(param\("resolution"\)\s*==\s*("(?:\\.|[^"\\])*")(?:\s*&&\s*(!?has_media\("video"\)))?)\s*\?\s*)?tier\(("(?:\\.|[^"\\])*"),\s*charge\(("(?:per_second|per_call)"),\s*quantity,\s*([0-9]+(?:\.[0-9]+)?)\)(?:\s*\+\s*charge\("per_second",\s*video_input_durations,\s*([0-9]+(?:\.[0-9]+)?)\))?\)`,
    'g'
  )
  const tiers: SeedanceTier[] = []
  let lastBranchHadCondition = false
  let cursor = 0
  let match: RegExpExecArray | null
  while ((match = branchRe.exec(body)) !== null) {
    if (
      body.slice(cursor, match.index).trim() !== (tiers.length === 0 ? '' : ':')
    ) {
      return null
    }
    const hasCondition = Boolean(match[1])
    lastBranchHadCondition = hasCondition
    const label = JSON.parse(match[4]) as string
    const method = JSON.parse(match[5]) as SeedanceBillingMethod
    const price = Number(match[6])
    const videoInputPrice = match[7] === undefined ? undefined : Number(match[7])
    const mediaCondition = match[3]
    if (mediaCondition && method !== 'per_second') return null
    if (videoInputPrice !== undefined && method !== 'per_second') return null
    if (
      videoInputPrice !== undefined &&
      mediaCondition === '!has_media("video")'
    ) {
      return null
    }
    let videoInput: SeedanceVideoInput | undefined
    if (mediaCondition === 'has_media("video")') {
      videoInput = 'with_video'
    } else if (mediaCondition === '!has_media("video")') {
      videoInput = 'without_video'
    }
    const tier: SeedanceTier = {
      label,
      resolution: hasCondition
        ? String(JSON.parse(match[2])).trim()
        : undefined,
      method,
      price,
      videoInput,
      fallback: false,
    }
    if (videoInputPrice !== undefined) {
      tier.videoInputPrice = videoInputPrice
    }
    if (hasCondition && method === 'per_second' && !mediaCondition) {
      tiers.push(
        { ...tier, videoInput: 'without_video' },
        { ...tier, videoInput: 'with_video' }
      )
    } else {
      tiers.push(tier)
    }
    cursor = branchRe.lastIndex
  }
  if (
    tiers.length === 0 ||
    lastBranchHadCondition ||
    body.slice(cursor).trim() !== '' ||
    tiers.some((tier, index) =>
      index === tiers.length - 1 ? false : !tier.resolution
    )
  ) {
    return null
  }
  const terminalTier = tiers.at(-1)
  if (!terminalTier) {
    return null
  }
  // Seedance expressions always end in an unconditional branch. Remove that
  // branch when it is the editor's rejection sentinel or a legacy fallback;
  // keep a standalone unconditional tier as a valid legacy expression.
  if (
    isSeedanceUnsupportedResolutionTier(terminalTier) ||
    (tiers.length > 1 && !terminalTier.resolution)
  ) {
    tiers.pop()
  }
  if (tiers.length === 1 && !tiers[0].resolution) {
    const legacyTier = tiers[0]
    legacyTier.resolution = '480p'
    if (legacyTier.method === 'per_second') {
      return normalizeSeedanceConfig({
        tiers: [
          {
            ...legacyTier,
            videoInput: 'without_video',
            videoInputPrice: undefined,
          },
          {
            ...legacyTier,
            videoInput: 'with_video',
          },
        ],
      })
    }
  }
  if (tiers.length === 0) {
    return null
  }
  // The grammar above is deliberately strict enough to reject non-Seedance
  // v2 expressions. Do not compare by removing all whitespace from the full
  // expression: that would also remove meaningful spaces inside JSON strings
  // such as a tier label or resolution value.
  return normalizeSeedanceConfig({ tiers })
}

export function getTierCacheMode(
  tier: Partial<VisualTier> | null | undefined
): CacheMode {
  if (tier?.cache_mode === CACHE_MODE_TIMED) return CACHE_MODE_TIMED
  if (tier?.cache_mode === CACHE_MODE_GENERIC) return CACHE_MODE_GENERIC
  return Number(tier?.cache_create_1h_unit_cost) > 0
    ? CACHE_MODE_TIMED
    : CACHE_MODE_GENERIC
}

export function normalizeVisualTier(
  tier: Partial<VisualTier> = {}
): VisualTier {
  return {
    label: tier.label ?? '',
    input_unit_cost: Number(tier.input_unit_cost) || 0,
    output_unit_cost: Number(tier.output_unit_cost) || 0,
    cache_mode: getTierCacheMode(tier),
    conditions: Array.isArray(tier.conditions) ? tier.conditions : [],
    ...tier,
    cache_read_unit_cost: Number(tier.cache_read_unit_cost) || 0,
    cache_create_unit_cost: Number(tier.cache_create_unit_cost) || 0,
    cache_create_1h_unit_cost: Number(tier.cache_create_1h_unit_cost) || 0,
    image_unit_cost: Number(tier.image_unit_cost) || 0,
    image_output_unit_cost: Number(tier.image_output_unit_cost) || 0,
    audio_input_unit_cost: Number(tier.audio_input_unit_cost) || 0,
    audio_output_unit_cost: Number(tier.audio_output_unit_cost) || 0,
  }
}

export function createDefaultVisualConfig(): VisualConfig {
  return {
    tiers: [
      normalizeVisualTier({
        conditions: [],
        input_unit_cost: 0,
        output_unit_cost: 0,
        label: 'base',
        cache_mode: CACHE_MODE_GENERIC,
      }),
    ],
  }
}

export function normalizeVisualConfig(
  config: VisualConfig | null | undefined
): VisualConfig {
  if (!config || !Array.isArray(config.tiers) || config.tiers.length === 0) {
    return createDefaultVisualConfig()
  }
  return {
    ...config,
    tiers: config.tiers.map((tier) => normalizeVisualTier(tier)),
  }
}

function buildConditionStr(conditions: TierConditionInput[]): string {
  if (!conditions || conditions.length === 0) return ''
  return conditions
    .filter((c) => c.var && c.op && c.value != null && c.value !== '')
    .map((c) => `${c.var} ${c.op} ${c.value}`)
    .join(' && ')
}

function buildTierBodyExpr(tier: VisualTier): string {
  const parts: string[] = []
  const ic = Number(tier.input_unit_cost) || 0
  const oc = Number(tier.output_unit_cost) || 0
  parts.push(`p * ${ic}`)
  parts.push(`c * ${oc}`)
  for (const cv of BILLING_CACHE_VAR_MAP) {
    const v = Number((tier as Record<string, unknown>)[cv.field]) || 0
    if (v !== 0) parts.push(`${cv.exprVar} * ${v}`)
  }
  return parts.join(' + ')
}

export function generateExprFromVisualConfig(
  config: VisualConfig | null | undefined
): string {
  if (!config || !config.tiers || config.tiers.length === 0) {
    return 'p * 0 + c * 0'
  }
  const tiers = config.tiers

  if (tiers.length === 1) {
    const tier = tiers[0]
    const label = tier.label || 'default'
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)
    if (cond) {
      return `${cond} ? ${body} : p * 0 + c * 0`
    }
    return body
  }

  const parts: string[] = []
  for (let i = 0; i < tiers.length; i++) {
    const tier = tiers[i]
    const label = tier.label || `tier_${i + 1}`
    const body = `tier("${label}", ${buildTierBodyExpr(tier)})`
    const cond = buildConditionStr(tier.conditions)

    if (i < tiers.length - 1 && cond) {
      parts.push(`${cond} ? ${body}`)
    } else {
      parts.push(body)
    }
  }
  return parts.join(' : ')
}

export function tryParseVisualConfig(
  exprStr: string | null | undefined
): VisualConfig | null {
  if (!exprStr) return null
  try {
    let body = exprStr
    const versionMatch = body.match(/^v\d+:([\s\S]*)$/)
    if (versionMatch) body = versionMatch[1]
    const cacheVarNames = BILLING_CACHE_VAR_MAP.map((cv) => cv.exprVar)
    const optCacheStr = cacheVarNames
      .map((v) => `(?:\\s*\\+\\s*${v}\\s*\\*\\s*([\\d.eE+-]+))?`)
      .join('')

    const bodyPat = `p\\s*\\*\\s*([\\d.eE+-]+)\\s*\\+\\s*c\\s*\\*\\s*([\\d.eE+-]+)${optCacheStr}`

    const singleRe = new RegExp(`^tier\\("([^"]*)",\\s*${bodyPat}\\)$`)
    const simple = body.match(singleRe)
    if (simple) {
      const tier: Record<string, unknown> = {
        conditions: [],
        input_unit_cost: Number(simple[2]),
        output_unit_cost: Number(simple[3]),
        label: simple[1],
      }
      BILLING_CACHE_VAR_MAP.forEach((cv, i) => {
        const val = simple[4 + i]
        if (val != null) tier[cv.field] = Number(val)
      })
      return normalizeVisualConfig({
        tiers: [normalizeVisualTier(tier as Partial<VisualTier>)],
      })
    }

    const condGroup =
      `((?:(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)` +
      `(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<|<=|>|>=)\\s*[\\d.eE+]+)*)`
    const tierRe = new RegExp(
      `(?:${condGroup}\\s*\\?\\s*)?tier\\("([^"]*)",\\s*${bodyPat}\\)`,
      'g'
    )
    const tiers: VisualTier[] = []
    let match: RegExpExecArray | null
    while ((match = tierRe.exec(body)) !== null) {
      const condStr = match[1] || ''
      const conditions: TierConditionInput[] = []
      if (condStr) {
        for (const cp of condStr.split(/\s*&&\s*/)) {
          const cm = cp.trim().match(/^(p|c|len)\s*(<|<=|>|>=)\s*([\d.eE+]+)$/)
          if (cm) {
            conditions.push({
              var: cm[1] as TierConditionInput['var'],
              op: cm[2] as TierConditionInput['op'],
              value: Number(cm[3]),
            })
          }
        }
      }
      const tier: Record<string, unknown> = {
        conditions,
        input_unit_cost: Number(match[3]),
        output_unit_cost: Number(match[4]),
        label: match[2],
      }
      const m = match
      BILLING_CACHE_VAR_MAP.forEach((cv, i) => {
        const val = m[5 + i]
        if (val != null) tier[cv.field] = Number(val)
      })
      tiers.push(normalizeVisualTier(tier as Partial<VisualTier>))
    }
    if (tiers.length === 0) return null

    const cfg = normalizeVisualConfig({ tiers })
    const regenerated = generateExprFromVisualConfig(cfg)
    if (regenerated.replaceAll(/\s+/g, '') !== body.replaceAll(/\s+/g, '')) {
      return null
    }
    return cfg
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Local cost evaluator (for the estimator preview)
// ---------------------------------------------------------------------------

const ESTIMATOR_VARS = [
  { var: 'cr', stateKey: 'cacheReadTokens' },
  { var: 'cc', stateKey: 'cacheCreateTokens' },
  { var: 'cc1h', stateKey: 'cacheCreate1hTokens' },
  { var: 'img', stateKey: 'imageTokens' },
  { var: 'img_o', stateKey: 'imageOutputTokens' },
  { var: 'ai', stateKey: 'audioInputTokens' },
  { var: 'ao', stateKey: 'audioOutputTokens' },
] as const

export type ExtraTokenValues = Record<
  (typeof ESTIMATOR_VARS)[number]['stateKey'],
  number
>

export type EvalResult = {
  cost: number
  matchedTier: string
  error: string | null
}

export function evalExprLocally(
  exprStr: string,
  promptTokens: number,
  completionTokens: number,
  extraTokenValues: ExtraTokenValues
): EvalResult {
  try {
    if (!exprStr || !exprStr.trim()) {
      return { cost: 0, matchedTier: '', error: null }
    }
    let matchedTier = ''
    const tierFn = (name: string, value: number) => {
      matchedTier = name
      return value
    }
    const cacheReadTokens = extraTokenValues.cacheReadTokens || 0
    const cacheCreateTokens = extraTokenValues.cacheCreateTokens || 0
    const cacheCreate1hTokens = extraTokenValues.cacheCreate1hTokens || 0
    const len =
      promptTokens + cacheReadTokens + cacheCreateTokens + cacheCreate1hTokens
    const env: Record<string, unknown> = {
      p: promptTokens,
      c: completionTokens,
      len,
      tier: tierFn,
      max: Math.max,
      min: Math.min,
      abs: Math.abs,
      ceil: Math.ceil,
      floor: Math.floor,
    }
    for (const field of ESTIMATOR_VARS) {
      env[field.var] = extraTokenValues[field.stateKey] || 0
    }
    const fn = new Function(
      ...Object.keys(env),
      `"use strict"; return (${exprStr});`
    )
    const cost = Number(fn(...Object.values(env))) || 0
    return { cost, matchedTier, error: null }
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e)
    return { cost: 0, matchedTier: '', error: message }
  }
}

export function exprUsesExtraVars(exprStr: string): boolean {
  if (!exprStr) return false
  const varNames = ESTIMATOR_VARS.map((f) => f.var).join('|')
  return new RegExp(`\\b(${varNames})\\b`).test(exprStr)
}

export const ESTIMATOR_EXTRA_FIELDS = ESTIMATOR_VARS
