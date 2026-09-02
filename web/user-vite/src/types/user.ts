export interface UserProfile {
  id: number;
  email: string;
  username?: string;
  uuid: string;
  token: string;
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

