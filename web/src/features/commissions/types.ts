export interface CommissionSettings {
  enabled: boolean
  global_rate_enabled: boolean
  global_rate_percent: number
}

export interface CommissionAgent {
  user_id: number
  username: string
  display_name: string
  email: string
  role: number
  status: number
  parent_user_id: number
  parent_email: string
  use_custom_rate: boolean
  rate_basis_points: number
  effective_rate_basis_points: number
  rate_percent?: number
  effective_rate_percent?: number
  rate_source: 'agent' | 'global' | 'none' | string
}

export interface CommissionRecord {
  id: number
  agent_user_id: number
  descendant_user_id: number
  top_up_id: number
  trade_no: string
  payment_amount_minor: number
  payment_currency: string
  commission_rate_basis_points: number
  commission_amount_minor: number
  created_at: number
  agent_username: string
  agent_display_name: string
  agent_email: string
  descendant_username: string
  descendant_name: string
  descendant_email: string
}

export interface CommissionSummary {
  currency: string
  payment_amount_minor: number
  commission_amount_minor: number
  record_count: number
}

export interface CommissionReferral {
  id: number
  username: string
  display_name: string
  email: string
  role: number
  status: number
  created_at: number
}

export interface CommissionSelf {
  user_id: number
  use_custom_rate: boolean
  rate_percent: number
  rate_source: string
}

export interface CommissionCurrencyBalance {
  currency: string
  amount_minor: number
}

export interface CommissionWithdrawalBalance {
  currency_balances: CommissionCurrencyBalance[]
  total_commission_usd_minor: number
  withdrawable_usdt_minor: number
  transferable_quota: number
  exchange_rate_minor: number
  fee_usdt_minor: number
  actual_usdt_minor: number
  min_amount_usdt_minor: number
  can_withdraw: boolean
}

export interface CommissionWithdrawal {
  id: number
  user_id: number
  amount_usdt_minor: number
  fee_usdt_minor: number
  actual_usdt_minor: number
  network: string
  address: string
  status: 'pending' | 'approved' | 'rejected'
  tx_hash: string
  reject_reason: string
  created_at: number
  updated_at: number
}

export interface WithdrawalConfig {
  id: number
  currency: string
  network: string
  exchange_rate_minor: number
  fee_usdt_minor: number
  min_amount_usdt_minor: number
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface WithdrawalRequestView extends CommissionWithdrawal {
  username: string
  display_name: string
  email: string
}

export interface PageResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}
