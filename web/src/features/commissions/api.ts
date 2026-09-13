import { api } from '@/lib/api'

import type {
  CommissionAgent,
  CommissionRecord,
  CommissionReferral,
  CommissionSelf,
  CommissionSettings,
  CommissionSummary,
  CommissionWithdrawal,
  CommissionWithdrawalBalance,
  PageResponse,
  WithdrawalConfig,
  WithdrawalRequestView,
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

export async function getCommissionSelfRecords(
  page: number,
  pageSize: number,
  keyword = '',
  startTime = 0,
  endTime = 0
) {
  const res = await api.get('/api/commission/self/records', {
    params: {
      p: page,
      page_size: pageSize,
      keyword: keyword || undefined,
      start_time: startTime || undefined,
      end_time: endTime || undefined,
    },
    skipErrorHandler: true,
  })
  return res.data as { data: PageResponse<CommissionRecord> }
}

export async function getCommissionSelfReferrals(
  page: number,
  pageSize: number,
  keyword = '',
  startTime = 0,
  endTime = 0
) {
  const res = await api.get('/api/commission/self/referrals', {
    params: {
      p: page,
      page_size: pageSize,
      keyword: keyword || undefined,
      start_time: startTime || undefined,
      end_time: endTime || undefined,
    },
    skipErrorHandler: true,
  })
  return res.data as { data: PageResponse<CommissionReferral> }
}

export async function getCommissionSelfBalance() {
  const res = await api.get('/api/commission/self/balance', {
    skipErrorHandler: true,
  })
  return res.data as { data: CommissionWithdrawalBalance }
}

export async function transferCommissionBalanceToQuota() {
  const res = await api.post(
    '/api/commission/self/transfer-to-balance',
    undefined,
    { skipErrorHandler: true }
  )
  return res.data as { data: { quota: number } }
}

export async function createCommissionSelfWithdrawal(address: string) {
  const res = await api.post(
    '/api/commission/self/withdrawals',
    { address },
    { skipErrorHandler: true }
  )
  return res.data as { data: CommissionWithdrawal }
}

export async function getCommissionSelfWithdrawals(
  page: number,
  pageSize: number,
  startTime = 0,
  endTime = 0
) {
  const res = await api.get('/api/commission/self/withdrawals', {
    params: {
      p: page,
      page_size: pageSize,
      start_time: startTime || undefined,
      end_time: endTime || undefined,
    },
    skipErrorHandler: true,
  })
  return res.data as { data: PageResponse<CommissionWithdrawal> }
}

export async function getWithdrawalConfigs() {
  const res = await api.get('/api/withdrawals/configs')
  return res.data as { data: { items: WithdrawalConfig[] } }
}

export async function createWithdrawalConfig(payload: {
  exchange_rate: number
  fee_usdt: number
  min_amount_usdt: number
  enabled: boolean
}) {
  const res = await api.post('/api/withdrawals/configs', payload)
  return res.data as { data: WithdrawalConfig }
}

export async function updateWithdrawalConfig(
  id: number,
  payload: {
    exchange_rate: number
    fee_usdt: number
    min_amount_usdt: number
    enabled: boolean
  }
) {
  const res = await api.put(`/api/withdrawals/configs/${id}`, payload)
  return res.data as { data: WithdrawalConfig }
}

export async function deleteWithdrawalConfig(id: number) {
  const res = await api.delete(`/api/withdrawals/configs/${id}`)
  return res.data
}

export async function getWithdrawalRequests(params: {
  page: number
  pageSize: number
  keyword?: string
  status?: string
  startTime?: number
  endTime?: number
}) {
  const res = await api.get('/api/withdrawals', {
    params: {
      p: params.page,
      page_size: params.pageSize,
      keyword: params.keyword || undefined,
      status: params.status || undefined,
      start_time: params.startTime || undefined,
      end_time: params.endTime || undefined,
    },
  })
  return res.data as { data: PageResponse<WithdrawalRequestView> }
}

export async function approveWithdrawalRequest(id: number, txHash: string) {
  const res = await api.post(`/api/withdrawals/${id}/approve`, {
    tx_hash: txHash,
  })
  return res.data as { data: WithdrawalRequestView }
}

export async function rejectWithdrawalRequest(id: number, reason: string) {
  const res = await api.post(`/api/withdrawals/${id}/reject`, {
    reject_reason: reason,
  })
  return res.data as { data: WithdrawalRequestView }
}
