export interface UserProfile {
  id: number;
  email: string;
  username?: string;
  uuid: string;
  token: string;
  plan_id?: number;
  plan?: Plan;
  group_id?: number;
  transfer_enable: number;
  transfer_used?: number;
  u: number;
  d: number;
  expired_at?: number;
  is_admin: boolean;
  is_staff: boolean;
  status: number;
  banned: boolean;
  telegram_id?: number;
  subscribe_url?: string;
  created_at: number;
  updated_at: number;
}

export interface Plan {
  id: number;
  name: string;
  content?: string;
  transfer_enable: number;
  speed_limit?: number;
  device_limit?: number;
  prices?: PlanPrice[];
  show: boolean;
  sell: boolean;
  renew: boolean;
  sort: number;
  created_at: number;
  updated_at: number;
}

export interface PlanPrice {
  period: string;
  price: number;
}
