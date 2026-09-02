/**
 * Country & Region helpers for server nodes.
 */

// Chinese country name dictionary for common proxy server regions
const COUNTRY_NAME_MAP_ZH: Record<string, string> = {
  HK: "香港",
  JP: "日本",
  US: "美国",
  SG: "新加坡",
  TW: "台湾",
  KR: "韩国",
  GB: "英国",
  UK: "英国",
  DE: "德国",
  FR: "法国",
  NL: "荷兰",
  CA: "加拿大",
  AU: "澳大利亚",
  RU: "俄罗斯",
  IN: "印度",
  MY: "马来西亚",
  TH: "泰国",
  VN: "越南",
  PH: "菲律宾",
  ID: "印尼",
  MO: "澳门",
  CH: "瑞士",
  SE: "瑞典",
  NO: "挪威",
  FI: "芬兰",
  DK: "丹麦",
  PT: "葡萄牙",
  ES: "西班牙",
  IT: "意大利",
  BR: "巴西",
  AR: "阿根廷",
  ZA: "南非",
  AE: "阿联酋",
  TR: "土耳其",
};

// Common keywords for fallback matching from node names
const COUNTRY_KEYWORD_RULES: Array<[RegExp, string]> = [
  [/(?:HK|香港|HONG\s*KONG)/i, "HK"],
  [/(?:JP|日本|JAPAN|东京|TOKYO|大阪|OSAKA)/i, "JP"],
  [/(?:US|美国|USA|UNITED\s*STATES|洛杉矶|LOS\s*ANGELES|硅谷|SILICON|纽约|NEW\s*YORK)/i, "US"],
  [/(?:SG|新加坡|SINGAPORE|狮城)/i, "SG"],
  [/(?:TW|台湾|TAIWAN|台北|TAIPEI)/i, "TW"],
  [/(?:KR|韩国|KOREA|首尔|SEOUL)/i, "KR"],
  [/(?:GB|UK|英国|UNITED\s*KINGDOM|伦敦|LONDON)/i, "GB"],
  [/(?:DE|德国|GERMANY|法兰克福|FRANKFURT)/i, "DE"],
  [/(?:FR|法国|FRANCE|巴黎|PARIS)/i, "FR"],
  [/(?:NL|荷兰|NETHERLANDS|阿姆斯特丹|AMSTERDAM)/i, "NL"],
  [/(?:CA|加拿大|CANADA|多伦多|TORONTO|温哥华|VANCOUVER)/i, "CA"],
  [/(?:AU|澳大利亚|澳洲|AUSTRALIA|悉尼|SYDNEY|墨尔本|MELBOURNE)/i, "AU"],
  [/(?:RU|俄罗斯|RUSSIA|莫斯科|MOSCOW)/i, "RU"],
  [/(?:MY|马来西亚|MALAYSIA|吉隆坡|KUALA\s*LUMPUR)/i, "MY"],
  [/(?:TH|泰国|THAILAND|曼谷|BANGKOK)/i, "TH"],
  [/(?:VN|越南|VIETNAM|胡志明|HO\s*CHI\s*MINH)/i, "VN"],
  [/(?:PH|菲律宾|PHILIPPINES|马尼拉|MANILA)/i, "PH"],
  [/(?:IN|印度|INDIA|孟买|MUMBAI)/i, "IN"],
  [/(?:MO|澳门|MACAU)/i, "MO"],
  [/(?:TR|土耳其|TURKEY|伊斯坦布尔|ISTANBUL)/i, "TR"],
  [/(?:AE|阿联酋|DUBAI|迪拜)/i, "AE"],
  [/(?:BR|巴西|BRAZIL|圣保罗|SAO\s*PAULO)/i, "BR"],
  [/(?:AR|阿根廷|ARGENTINA)/i, "AR"],
];

/**
 * Convert a 2-letter ISO country code to Flag Emoji via Unicode Regional Indicator Symbols.
 */
export function isoToFlagEmoji(countryCode: string): string {
  const code = countryCode.trim().toUpperCase();
  if (/^[A-Z]{2}$/.test(code)) {
    const first = 0x1f1e6 + (code.charCodeAt(0) - 65);
    const second = 0x1f1e6 + (code.charCodeAt(1) - 65);
    return String.fromCodePoint(first, second);
  }
  return "";
}

/**
 * Get Flag Emoji from country code or fallback to node name extraction.
 */
export function getCountryFlag(countryCode?: string, name?: string): string {
  if (countryCode) {
    const flag = isoToFlagEmoji(countryCode);
    if (flag) return flag;
  }

  if (name) {
    // 1. Check if name contains a Unicode regional indicator flag emoji
    const flagMatch = name.match(/[\uD83C][\uDDE6-\uDDFF][\uD83C][\uDDE6-\uDDFF]/u);
    if (flagMatch) {
      return flagMatch[0];
    }

    // 2. Fallback to keyword matching in name
    for (const [pattern, code] of COUNTRY_KEYWORD_RULES) {
      if (pattern.test(name)) {
        const flag = isoToFlagEmoji(code);
        if (flag) return flag;
      }
    }
  }

  return "🌐";
}

/**
 * Format Country and Region display text (e.g. "日本 · 东京" or "美国 · 洛杉矶").
 */
export function getCountryDisplayName(
  countryCode?: string,
  region?: string,
  _t?: (key: string) => string
): string {
  const code = countryCode?.trim().toUpperCase() || "";
  const countryName = COUNTRY_NAME_MAP_ZH[code] || code;
  const regionName = region?.trim() || "";

  if (countryName && regionName) {
    return `${countryName} · ${regionName}`;
  }
  return countryName || regionName || "";
}

/**
 * Resolve server node status: 0 = Offline, 1 = Online, 2 = Degraded.
 */
export function getServerStatus(server: { status?: number; is_online?: boolean }): 0 | 1 | 2 {
  if (typeof server.status === "number") {
    if (server.status === 1) return 1;
    if (server.status === 2) return 2;
    if (server.status === 0) return 0;
  }
  return server.is_online ? 1 : 0;
}
