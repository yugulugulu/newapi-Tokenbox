import { api } from '@/lib/api'

import type {
  CommissionAgent,
  CommissionRecord,
  CommissionReferral,
  CommissionSelf,
  CommissionSettings,
  CommissionSummary,
  PageResponse,
} from './types'

export async function getCommissionSettings() {
  const res = await api.get('/api/commission/admin/settings')
  return res.data as { data: CommissionSettings }
}

export async function updateCommissionSettings(payload: {
  enabled?: boolean
  global_rate_enabled?: boolean
  global_rate_percent?: number
}) {
  const res = await api.put('/api/commission/admin/settings', payload)
  return res.data as { data: CommissionSettings }
}

export async function getCommissionAgents(
  page: number,
  pageSize: number,
  keyword: string
) {
  const res = await api.get('/api/commission/admin/agents', {
    params: { p: page, page_size: pageSize, keyword },
  })
  const data = res.data as {
    data: PageResponse<CommissionAgent> & {
      items: Array<
        CommissionAgent & {
          rate_basis_points: number
          effective_rate_basis_points: number
        }
      >
    }
  }
  return {
    ...data,
    data: {
      ...data.data,
      items: data.data.items.map((agent) => ({
        ...agent,
        rate_percent: agent.rate_basis_points / 100,
        effective_rate_percent: agent.effective_rate_basis_points / 100,
      })),
    },
  }
}

export async function updateCommissionAgent(
  userId: number,
  payload: { use_custom_rate: boolean; rate_percent: number }
) {
  const res = await api.put(`/api/commission/admin/agents/${userId}`, payload)
  return res.data
}

export async function getCommissionSummary(
  keyword: string,
  startTime: number,
  endTime: number
) {
  const res = await api.get('/api/commission/admin/summary', {
    params: {
      keyword,
      start_time: startTime || undefined,
      end_time: endTime || undefined,
    },
  })
  return res.data as { data: { items: CommissionSummary[] } }
}

export async function getCommissionRecords(
  page: number,
  pageSize: number,
  keyword: string,
  startTime: number,
  endTime: number
) {
  const res = await api.get('/api/commission/admin/records', {
    params: {
      p: page,
      page_size: pageSize,
      keyword,
      start_time: startTime || undefined,
      end_time: endTime || undefined,
    },
  })
  return res.data as { data: PageResponse<CommissionRecord> }
}

export async function getCommissionSelf() {
  const res = await api.get('/api/commission/self', { skipErrorHandler: true })
  return res.data as { data: CommissionSelf }
}

export async function getCommissionCode() {
  const res = await api.get('/api/commission/self/code', {
    skipErrorHandler: true,
  })
  return res.data as { data: string }
}

export async function getCommissionSelfRecords(page: number, pageSize: number) {
  const res = await api.get('/api/commission/self/records', {
    params: { p: page, page_size: pageSize },
    skipErrorHandler: true,
  })
  return res.data as { data: PageResponse<CommissionRecord> }
}

export async function getCommissionSelfReferrals(
  page: number,
  pageSize: number
) {
  const res = await api.get('/api/commission/self/referrals', {
    params: { p: page, page_size: pageSize },
    skipErrorHandler: true,
  })
  return res.data as { data: PageResponse<CommissionReferral> }
}
