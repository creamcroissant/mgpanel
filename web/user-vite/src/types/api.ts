export interface ApiResponse<T> {
  data: T;
  message?: string;
}

export interface ApiError {
  message: string;
  code?: number;
}

export interface TrafficLog {
  id: number;
  user_id: number;
  u: number;
  d: number;
  record_at: number;
  record_type: number;
}

export interface KnowledgeArticle {
  id: number;
  title: string;
  category?: string;
  language?: string;
  body?: string;
  show?: boolean;
  sort?: number;
  created_at?: number;
  updated_at?: number;
}

export interface ShortLink {
  id: number;
  code: string;
  target_url: string;
  clicks: number;
  created_at: number;
  expires_at?: number;
}
