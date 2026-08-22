/**
 * Built-in routing presets for sing-box/xray.
 *
 * Each preset is a complete routing config (domainStrategy + final + rules)
 * that can be merged into an existing routing configuration.
 *
 * Rules use `rule_set` to reference sing-geosite / sing-geoip categories
 * (e.g. "geosite-openai", "geoip-cn"). The outboundTag values are
 * placeholders that users should customize after applying.
 */

export type RoutingRule = {
  enabled?: boolean;
  domain?: string[];
  ip?: string[];
  port?: string;
  sourcePort?: string;
  network?: string;
  protocol?: string[];
  inboundTag?: string[];
  outboundTag?: string;
  balancerTag?: string;
  rule_set?: string[];
  action?: string;
  type?: string;
};

export type RoutingConfig = {
  domainStrategy?: string;
  final?: string;
  rules?: RoutingRule[];
};

export interface RoutingPreset {
  id: string;
  name: string;
  nameZh: string;
  description: string;
  descriptionZh: string;
  category: "proxy" | "direct" | "block" | "recommended";
  categoryZh: string;
  tag: string;  // default tag for the outbound
  config: RoutingConfig;
}

const BUILTIN_PRESETS: RoutingPreset[] = [
  // ============================================================
  // 一、代理分流类 - Proxy
  // ============================================================
  {
    id: "ai-chat",
    name: "AI Chat",
    nameZh: "AI 聊天",
    description: "Route AI services (OpenAI, Claude, Gemini, Copilot) through proxy",
    descriptionZh: "将 AI 服务（OpenAI、Claude、Gemini、Copilot）路由到代理",
    category: "proxy",
    categoryZh: "代理分流",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-openai"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-anthropic"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-google-gemini"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-github-copilot"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-bytedance-ai-!cn"],
        },
      ],
    },
  },

  {
    id: "streaming-media",
    name: "Streaming Media",
    nameZh: "海外流媒体",
    description: "Route Netflix, Spotify, YouTube, TikTok, Disney+, HBO through proxy",
    descriptionZh: "将 Netflix、Spotify、YouTube、TikTok、Disney+、HBO 路由到代理",
    category: "proxy",
    categoryZh: "代理分流",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-netflix"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-spotify"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-youtube"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-tiktok"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-disney"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-primevideo"],
        },
      ],
    },
  },

  {
    id: "social-media",
    name: "Social Networks",
    nameZh: "社交网络",
    description: "Route Twitter/X, Facebook, Instagram, Telegram, Discord through proxy",
    descriptionZh: "将 Twitter/X、Facebook、Instagram、Telegram、Discord 路由到代理",
    category: "proxy",
    categoryZh: "代理分流",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-twitter"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-facebook"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-instagram"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-telegram"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-discord"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-bluesky"],
        },
      ],
    },
  },

  {
    id: "google",
    name: "Google Services",
    nameZh: "Google 全家桶",
    description: "Route Google search, Scholar, and Google services through proxy",
    descriptionZh: "将 Google 搜索、学术及 Google 服务路由到代理",
    category: "proxy",
    categoryZh: "代理分流",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-google"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-google-scholar"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          ip: ["8.8.8.8/32", "8.8.4.4/32"],
        },
      ],
    },
  },

  {
    id: "gaming",
    name: "Gaming",
    nameZh: "海外游戏",
    description: "Route Steam, Epic Games, Blizzard, Riot Games through proxy",
    descriptionZh: "将 Steam、Epic Games、暴雪、Riot 游戏路由到代理",
    category: "proxy",
    categoryZh: "代理分流",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-steam"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-epicgames"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-blizzard"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-riot"],
        },
      ],
    },
  },

  {
    id: "developer",
    name: "Developer Tools",
    nameZh: "开发者工具",
    description: "Route GitHub, GitLab, npm, Docker through proxy (faster access)",
    descriptionZh: "将 GitHub、GitLab、npm、Docker 路由到代理（加速访问）",
    category: "proxy",
    categoryZh: "代理分流",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-github"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-github-copilot"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          rule_set: ["geosite-jetbrains"],
        },
        {
          enabled: true,
          outboundTag: "proxy",
          domain: ["npmjs.org", "registry.npmjs.org", "hub.docker.com", "pypi.org", "files.pythonhosted.org"],
        },
      ],
    },
  },

  // ============================================================
  // 二、直连类 - Direct
  // ============================================================
  {
    id: "china-direct",
    name: "China Direct",
    nameZh: "中国大陆直连",
    description: "Route Chinese domains and IPs directly (not through proxy)",
    descriptionZh: "将中国大陆域名和 IP 直连（不走代理）",
    category: "direct",
    categoryZh: "直连",
    tag: "direct",
    config: {
      domainStrategy: "AsIs",
      final: "proxy",
      rules: [
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geosite-cn"],
        },
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geosite-geolocation-cn"],
        },
        {
          enabled: true,
          outboundTag: "direct",
          ip: [
            "10.0.0.0/8",
            "172.16.0.0/12",
            "192.168.0.0/16",
            "100.64.0.0/10",
            "127.0.0.0/8",
            "fc00::/7",
            "fe80::/10",
          ],
        },
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geoip-cn"],
        },
      ],
    },
  },

  {
    id: "lan-direct",
    name: "LAN & Private",
    nameZh: "内网/私有地址",
    description: "Bypass proxy for LAN, private, and multicast addresses",
    descriptionZh: "内网、私有地址和组播地址直连",
    category: "direct",
    categoryZh: "直连",
    tag: "direct",
    config: {
      domainStrategy: "AsIs",
      final: "proxy",
      rules: [
        {
          enabled: true,
          outboundTag: "direct",
          ip: [
            "10.0.0.0/8",
            "172.16.0.0/12",
            "192.168.0.0/16",
            "100.64.0.0/10",
            "127.0.0.0/8",
            "fc00::/7",
            "fe80::/10",
          ],
        },
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geosite-private"],
        },
      ],
    },
  },

  {
    id: "speedtest-cdn",
    name: "Speedtest & CDN",
    nameZh: "测速/CDN 直连",
    description: "Direct connection for speed test and CDN for optimal performance",
    descriptionZh: "测速和 CDN 直连以获得最佳性能",
    category: "direct",
    categoryZh: "直连",
    tag: "direct",
    config: {
      domainStrategy: "AsIs",
      final: "proxy",
      rules: [
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geosite-speedtest"],
        },
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geosite-cloudflare"],
        },
        {
          enabled: true,
          outboundTag: "direct",
          rule_set: ["geosite-cloudflare-cn"],
        },
      ],
    },
  },

  // ============================================================
  // 三、屏蔽类 - Block
  // ============================================================
  {
    id: "ads-trackers",
    name: "Ads & Trackers",
    nameZh: "广告/追踪",
    description: "Block ads, tracking, and malware domains",
    descriptionZh: "屏蔽广告、追踪和恶意域名",
    category: "block",
    categoryZh: "屏蔽",
    tag: "block",
    config: {
      domainStrategy: "AsIs",
      final: "proxy",
      rules: [
        {
          enabled: true,
          action: "block",
          rule_set: ["geosite-category-ads-all"],
        },
      ],
    },
  },

  {
    id: "p2p-adult",
    name: "P2P & Adult",
    nameZh: "BT/成人",
    description: "Block BitTorrent and adult content, or route through a specific exit",
    descriptionZh: "屏蔽 BitTorrent 和成人内容，或路由到指定出口",
    category: "block",
    categoryZh: "屏蔽",
    tag: "block",
    config: {
      domainStrategy: "AsIs",
      final: "proxy",
      rules: [
        {
          enabled: true,
          outboundTag: "block",
          rule_set: ["geosite-category-porn"],
        },
        {
          enabled: true,
          outboundTag: "block",
          protocol: ["bittorrent"],
        },
      ],
    },
  },

  // ============================================================
  // 四、组合推荐 - Recommended
  // ============================================================
  {
    id: "minimal-unlock",
    name: "Minimal Unlock (Recommended)",
    nameZh: "极简解锁（推荐）",
    description: "Only route AI + streaming + social through proxy, everything else goes direct. Good starting point for most users.",
    descriptionZh: "仅 AI + 流媒体 + 社交走代理，其余直连。适合大多数用户作为起点。",
    category: "recommended",
    categoryZh: "组合推荐",
    tag: "proxy",
    config: {
      domainStrategy: "AsIs",
      final: "direct",
      rules: [
        // AI
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-openai"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-google-gemini"] },
        // Streaming
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-netflix"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-spotify"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-youtube"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-tiktok"] },
        // Social
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-twitter"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-facebook"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-instagram"] },
        { enabled: true, outboundTag: "proxy", rule_set: ["geosite-telegram"] },
        // CN direct (must be before final)
        { enabled: true, outboundTag: "direct", rule_set: ["geosite-cn"] },
        { enabled: true, outboundTag: "direct", rule_set: ["geoip-cn"] },
        { enabled: true, outboundTag: "direct", ip: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"] },
      ],
    },
  },
];

export default BUILTIN_PRESETS;
