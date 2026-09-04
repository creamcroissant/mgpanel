/**
 * 分流规则匹配值预设候选库
 *
 * 提供 rule_set / domain / ip / protocol / port / inboundTag 等字段的
 * 预设候选值，供 “输入即搜索，点选填入” 使用。
 *
 * rule_set 基于 sing-box 官方 sing-geosite / sing-geoip 分类，
 * domain / ip / protocol / port 为常见运维高频值。
 */

export interface MatchCandidate {
  value: string;
  /** 字段实际填入值（默认同 value） */
  fill?: string;
  /** 中文说明 */
  descZh?: string;
  /** 英文说明 */
  descEn?: string;
}

/** rule_set 候选：geosite-* 常用分类 */
export const GEOSITE_CANDIDATES: MatchCandidate[] = [
  { value: "geosite-cn", descZh: "中国大陆域名", descEn: "Mainland China domains" },
  { value: "geosite-geolocation-cn", descZh: "中国+地理定位域名", descEn: "China Geolocation" },
  { value: "geosite-geolocation-!cn", descZh: "非中国地理定位", descEn: "Non-China Geolocation" },
  { value: "geosite-private", descZh: "私有/内网域名", descEn: "Private domains" },
  { value: "geosite-ads", descZh: "广告域名", descEn: "Ads domains" },
  { value: "geosite-category-ads-all", descZh: "广告追踪（全量）", descEn: "Ads & tracking (all)" },
  { value: "geosite-category-porn", descZh: "成人内容", descEn: "Porn categories" },
  { value: "geosite-malware", descZh: "恶意软件", descEn: "Malware domains" },
  { value: "geosite-phishing", descZh: "钓鱼网站", descEn: "Phishing domains" },

  // AI
  { value: "geosite-openai", descZh: "OpenAI / ChatGPT", descEn: "OpenAI / ChatGPT" },
  { value: "geosite-anthropic", descZh: "Anthropic / Claude", descEn: "Anthropic / Claude" },
  { value: "geosite-google-gemini", descZh: "Google Gemini", descEn: "Google Gemini" },
  { value: "geosite-github-copilot", descZh: "GitHub Copilot", descEn: "GitHub Copilot" },
  { value: "geosite-bytedance-ai-!cn", descZh: "字节 AI（非中国）", descEn: "ByteDance AI (non-CN)" },
  { value: "geosite-microsoft-copilot", descZh: "Microsoft Copilot", descEn: "Microsoft Copilot" },
  { value: "geosite-grok", descZh: "xAI Grok", descEn: "xAI Grok" },

  // 流媒体
  { value: "geosite-netflix", descZh: "Netflix", descEn: "Netflix" },
  { value: "geosite-spotify", descZh: "Spotify", descEn: "Spotify" },
  { value: "geosite-youtube", descZh: "YouTube", descEn: "YouTube" },
  { value: "geosite-tiktok", descZh: "TikTok", descEn: "TikTok" },
  { value: "geosite-disney", descZh: "Disney+", descEn: "Disney+" },
  { value: "geosite-primevideo", descZh: "Amazon Prime Video", descEn: "Prime Video" },
  { value: "geosite-hulu", descZh: "Hulu", descEn: "Hulu" },
  { value: "geosite-hbo", descZh: "HBO Max", descEn: "HBO Max" },
  { value: "geosite-apple-tv", descZh: "Apple TV+", descEn: "Apple TV+" },
  { value: "geosite-bilibili", descZh: "哔哩哔哩", descEn: "Bilibili" },
  { value: "geosite-iqiyi", descZh: "爱奇艺", descEn: "iQiyi" },
  { value: "geosite-youku", descZh: "优酷", descEn: "Youku" },
  { value: "geosite-tudou", descZh: "土豆", descEn: "Tudou" },
  { value: "geosite-mgtv", descZh: "芒果TV", descEn: "Mango TV" },
  { value: "geosite-qq-music", descZh: "QQ音乐", descEn: "QQ Music" },

  // 社交
  { value: "geosite-twitter", descZh: "Twitter / X", descEn: "Twitter / X" },
  { value: "geosite-facebook", descZh: "Facebook / Meta", descEn: "Facebook / Meta" },
  { value: "geosite-instagram", descZh: "Instagram", descEn: "Instagram" },
  { value: "geosite-telegram", descZh: "Telegram", descEn: "Telegram" },
  { value: "geosite-discord", descZh: "Discord", descEn: "Discord" },
  { value: "geosite-bluesky", descZh: "Bluesky", descEn: "Bluesky" },
  { value: "geosite-tumblr", descZh: "Tumblr", descEn: "Tumblr" },
  { value: "geosite-pinterest", descZh: "Pinterest", descEn: "Pinterest" },
  { value: "geosite-reddit", descZh: "Reddit", descEn: "Reddit" },
  { value: "geosite-whatsapp", descZh: "WhatsApp", descEn: "WhatsApp" },
  { value: "geosite-line", descZh: "LINE", descEn: "LINE" },
  { value: "geosite-snapchat", descZh: "Snapchat", descEn: "Snapchat" },
  { value: "geosite-wechat", descZh: "微信", descEn: "WeChat" },
  { value: "geosite-qq", descZh: "QQ/腾讯系", descEn: "QQ / Tencent" },

  // 开发 / 工作
  { value: "geosite-github", descZh: "GitHub", descEn: "GitHub" },
  { value: "geosite-gitlab", descZh: "GitLab", descEn: "GitLab" },
  { value: "geosite-jetbrains", descZh: "JetBrains", descEn: "JetBrains" },
  { value: "geosite-golang", descZh: "Go 语言", descEn: "Go lang" },
  { value: "geosite-npm", descZh: "npm", descEn: "npm" },
  { value: "geosite-pypi", descZh: "PyPI", descEn: "PyPI" },
  { value: "geosite-docker", descZh: "Docker", descEn: "Docker" },
  { value: "geosite-cloudflare", descZh: "Cloudflare", descEn: "Cloudflare" },
  { value: "geosite-amazon", descZh: "Amazon / AWS", descEn: "Amazon / AWS" },
  { value: "geosite-microsoft", descZh: "Microsoft", descEn: "Microsoft" },
  { value: "geosite-google", descZh: "Google", descEn: "Google" },
  { value: "geosite-google-scholar", descZh: "Google 学术", descEn: "Google Scholar" },
  { value: "geosite-apple", descZh: "Apple", descEn: "Apple" },

  // 游戏
  { value: "geosite-steam", descZh: "Steam", descEn: "Steam" },
  { value: "geosite-epicgames", descZh: "Epic Games", descEn: "Epic Games" },
  { value: "geosite-blizzard", descZh: "暴雪", descEn: "Blizzard" },
  { value: "geosite-riot", descZh: "Riot Games", descEn: "Riot Games" },
  { value: "geosite-playstation", descZh: "PlayStation", descEn: "PlayStation" },
  { value: "geosite-xbox", descZh: "Xbox", descEn: "Xbox" },
  { value: "geosite-nintendo", descZh: "Nintendo", descEn: "Nintendo" },
  { value: "geosite-origin", descZh: "EA / Origin", descEn: "EA / Origin" },

  // 其他常用服务
  { value: "geosite-speedtest", descZh: "测速网站", descEn: "Speedtest" },
  { value: "geosite-paypal", descZh: "PayPal", descEn: "PayPal" },
  { value: "geosite-stripe", descZh: "Stripe", descEn: "Stripe" },
  { value: "geosite-zoom", descZh: "Zoom", descEn: "Zoom" },
  { value: "geosite-google-play", descZh: "Google Play", descEn: "Google Play" },
  { value: "geosite-icloud", descZh: "iCloud", descEn: "iCloud" },
  { value: "geosite-japan", descZh: "日本域名", descEn: "Japan domains" },
  { value: "geosite-kr", descZh: "韩国域名", descEn: "Korea domains" },
  { value: "geosite-us", descZh: "美国域名", descEn: "US domains" },
  { value: "geosite-hk", descZh: "香港域名", descEn: "Hong Kong domains" },
  { value: "geosite-tw", descZh: "台湾域名", descEn: "Taiwan domains" },
  { value: "geosite-sg", descZh: "新加坡域名", descEn: "Singapore domains" },
];

/** rule_set 候选：geoip-* 常用分类 */
export const GEOIP_CANDIDATES: MatchCandidate[] = [
  { value: "geoip-cn", descZh: "中国 IP 段", descEn: "China IP ranges" },
  { value: "geoip-private", descZh: "私有 IP 段", descEn: "Private IP ranges" },
  { value: "geoip-google", descZh: "Google IP", descEn: "Google IP" },
  { value: "geoip-cloudflare", descZh: "Cloudflare IP", descEn: "Cloudflare IP" },
  { value: "geoip-microsoft", descZh: "Microsoft IP", descEn: "Microsoft IP" },
  { value: "geoip-netflix", descZh: "Netflix IP", descEn: "Netflix IP" },
  { value: "geoip-telegram", descZh: "Telegram IP", descEn: "Telegram IP" },
  { value: "geoip-twitter", descZh: "Twitter/X IP", descEn: "Twitter/X IP" },
  { value: "geoip-facebook", descZh: "Facebook IP", descEn: "Facebook IP" },
  { value: "geoip-amazon", descZh: "Amazon/AWS IP", descEn: "Amazon/AWS IP" },
  { value: "geoip-aws", descZh: "AWS IP", descEn: "AWS IP" },
  { value: "geoip-hk", descZh: "香港 IP", descEn: "Hong Kong IP" },
  { value: "geoip-jp", descZh: "日本 IP", descEn: "Japan IP" },
  { value: "geoip-kr", descZh: "韩国 IP", descEn: "Korea IP" },
  { value: "geoip-us", descZh: "美国 IP", descEn: "US IP" },
  { value: "geoip-sg", descZh: "新加坡 IP", descEn: "Singapore IP" },
  { value: "geoip-tw", descZh: "台湾 IP", descEn: "Taiwan IP" },
];

/** domain 候选：高频域名 */
export const DOMAIN_CANDIDATES: MatchCandidate[] = [
  { value: "google.com", descZh: "Google" },
  { value: "googleapis.com", descZh: "Google API" },
  { value: "gstatic.com" },
  { value: "youtube.com", descZh: "YouTube" },
  { value: "ytimg.com" },
  { value: "googlevideo.com" },
  { value: "github.com", descZh: "GitHub" },
  { value: "raw.githubusercontent.com" },
  { value: "githubusercontent.com" },
  { value: "chatgpt.com", descZh: "ChatGPT" },
  { value: "openai.com", descZh: "OpenAI" },
  { value: "anthropic.com", descZh: "Anthropic" },
  { value: "claude.ai", descZh: "Claude" },
  { value: "gemini.google.com", descZh: "Gemini" },
  { value: "telegram.org", descZh: "Telegram" },
  { value: "t.me" },
  { value: "twitter.com", descZh: "Twitter/X" },
  { value: "x.com", descZh: "X (Twitter)" },
  { value: "facebook.com", descZh: "Facebook" },
  { value: "instagram.com", descZh: "Instagram" },
  { value: "netflix.com", descZh: "Netflix" },
  { value: "spotify.com", descZh: "Spotify" },
  { value: "discord.com", descZh: "Discord" },
  { value: "reddit.com", descZh: "Reddit" },
  { value: "wikipedia.org", descZh: "Wikipedia" },
  { value: "cloudflare.com", descZh: "Cloudflare" },
  { value: "npmjs.org", descZh: "npm" },
  { value: "registry.npmjs.org" },
  { value: "pypi.org", descZh: "PyPI" },
  { value: "files.pythonhosted.org" },
  { value: "hub.docker.com", descZh: "Docker Hub" },
  { value: "steampowered.com", descZh: "Steam" },
  { value: "steamcommunity.com" },
  { value: "epicgames.com", descZh: "Epic Games" },
  { value: "speedtest.net", descZh: "Speedtest" },
  { value: "windowsupdate.com", descZh: "Windows 更新" },
  { value: "apple.com", descZh: "Apple" },
  { value: "icloud.com", descZh: "iCloud" },
  { value: "microsoft.com", descZh: "Microsoft" },
  { value: "msn.com" },
  { value: "whatsapp.com", descZh: "WhatsApp" },
  { value: "zoom.us", descZh: "Zoom" },
  { value: "paypal.com", descZh: "PayPal" },
  { value: "stripe.com", descZh: "Stripe" },
  { value: "jetbrains.com", descZh: "JetBrains" },
  { value: "docker.com", descZh: "Docker" },
  { value: "baidu.com", descZh: "百度" },
  { value: "qq.com", descZh: "腾讯" },
  { value: "weixin.qq.com", descZh: "微信" },
  { value: "taobao.com", descZh: "淘宝" },
  { value: "jd.com", descZh: "京东" },
  { value: "bilibili.com", descZh: "哔哩哔哩" },
  { value: "zhihu.com", descZh: "知乎" },
  { value: "douyin.com", descZh: "抖音" },
];

/** ip 候选：常见网段 / 服务器 */
export const IP_CANDIDATES: MatchCandidate[] = [
  { value: "10.0.0.0/8", descZh: "内网 A 类" },
  { value: "172.16.0.0/12", descZh: "内网 B 类" },
  { value: "192.168.0.0/16", descZh: "内网 C 类" },
  { value: "100.64.0.0/10", descZh: "CGNAT 共享地址" },
  { value: "127.0.0.0/8", descZh: "本机回环" },
  { value: "169.254.0.0/16", descZh: "链路本地" },
  { value: "fc00::/7", descZh: "IPv6 唯一本地" },
  { value: "fe80::/10", descZh: "IPv6 链路本地" },
  { value: "0.0.0.0/8", descZh: "本网络" },
  { value: "8.8.8.8", descZh: "Google DNS" },
  { value: "1.1.1.1", descZh: "Cloudflare DNS" },
  { value: "223.5.5.5", descZh: "阿里 DNS" },
  { value: "114.114.114.114", descZh: "114 DNS" },
];

/** protocol 候选（sing-box 支持的 inbound 协议嗅探值） */
export const PROTOCOL_CANDIDATES: MatchCandidate[] = [
  { value: "http", descZh: "HTTP" },
  { value: "tls", descZh: "TLS" },
  { value: "quic", descZh: "QUIC" },
  { value: "ssh", descZh: "SSH" },
  { value: "bittorrent", descZh: "BT 下载" },
  { value: "dns", descZh: "DNS" },
  { value: "stun", descZh: "STUN" },
  { value: "wireguard", descZh: "WireGuard" },
  { value: "rdp", descZh: "RDP 远程桌面" },
  { value: "smb", descZh: "SMB 文件共享" },
];

/** port 候选 */
export const PORT_CANDIDATES: MatchCandidate[] = [
  { value: "22", descZh: "SSH" },
  { value: "53", descZh: "DNS" },
  { value: "80", descZh: "HTTP" },
  { value: "443", descZh: "HTTPS" },
  { value: "8443", descZh: "HTTPS 备" },
  { value: "3000", descZh: "Node 开发" },
  { value: "3306", descZh: "MySQL" },
  { value: "5432", descZh: "PostgreSQL" },
  { value: "6379", descZh: "Redis" },
  { value: "8080", descZh: "HTTP 备" },
  { value: "8888", descZh: "面板常用" },
];

/** 字段 → 候选数据的映射 */
export type MatchFieldKind = "rule_set" | "domain" | "ip" | "protocol" | "port" | "inboundTag";

export const CANDIDATES_BY_KIND: Record<MatchFieldKind, MatchCandidate[]> = {
  rule_set: [...GEOSITE_CANDIDATES, ...GEOIP_CANDIDATES],
  domain: DOMAIN_CANDIDATES,
  ip: IP_CANDIDATES,
  protocol: PROTOCOL_CANDIDATES,
  port: PORT_CANDIDATES,
  inboundTag: [],
};

export function searchCandidates(kind: MatchFieldKind, keyword: string, limit = 50): MatchCandidate[] {
  const all = CANDIDATES_BY_KIND[kind] ?? [];
  const kw = keyword.trim().toLowerCase();
  if (!kw) return all.slice(0, limit);
  return all
    .filter((c) =>
      c.value.toLowerCase().includes(kw) ||
      (c.descZh ?? "").toLowerCase().includes(kw) ||
      (c.descEn ?? "").toLowerCase().includes(kw)
    )
    .slice(0, limit);
}

/**
 * 拓扑（mesh）匹配值候选：按匹配类型提供候选。
 * 拓扑语义中 geosite 值是不带 "geosite-" 前缀的裸分类名（如 netflix），
 * 后端 compiler 会按类型自动补全 rule_set/geosite 前缀；domain/ip_cidr 直用常见候选。
 */
export type TopologyMatchType = "geosite" | "domain" | "ip_cidr";

export function topologyMatchCandidates(matchType: TopologyMatchType): MatchCandidate[] {
  if (matchType === "domain") return DOMAIN_CANDIDATES;
  if (matchType === "ip_cidr") return IP_CANDIDATES;
  return GEOSITE_CANDIDATES.map((c) => ({
    value: c.value.replace(/^geosite-/, ""),
    fill: c.fill,
    descZh: c.descZh,
    descEn: c.descEn,
  }));
}

/** 拓扑匹配值搜索（裸值，匹配 value / descZh / descEn，大小写不敏感） */
export function searchTopologyMatchCandidates(matchType: TopologyMatchType, keyword: string, limit = 50): MatchCandidate[] {
  const all = topologyMatchCandidates(matchType);
  const kw = keyword.trim().toLowerCase();
  if (!kw) return all.slice(0, limit);
  return all
    .filter((c) =>
      c.value.toLowerCase().includes(kw) ||
      (c.descZh ?? "").toLowerCase().includes(kw) ||
      (c.descEn ?? "").toLowerCase().includes(kw)
    )
    .slice(0, limit);
}