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

export interface PageResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}
