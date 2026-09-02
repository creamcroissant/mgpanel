export interface ServerNode {
  id: number;
  code: string;
  name: string;
  type: string;
  host: string;
  port: number;
  rate: number;
  group_id?: number;
  tags?: string[];
  is_online?: boolean;
  status: number; // 0: 离线, 1: 在线, 2: 降级
  country?: string;
  region?: string;
  sort: number;
  created_at: number;
  updated_at: number;
}

export interface ServerGroup {
  id: number;
  name: string;
  servers?: ServerNode[];
}
