// Package unlock 提供流媒体解锁状态检测，供 agent 出口节点使用。
// 检测方法参考 github.com/xykt/IPQuality。
package unlock

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Service 代表一个流媒体平台。
type Service string

const (
	ServiceNetflix    Service = "netflix"
	ServiceDisneyPlus Service = "disney_plus"
	ServiceYouTube    Service = "youtube"
	ServiceTikTok     Service = "tiktok"
	ServiceReddit     Service = "reddit"
	ServiceChatGPT    Service = "chatgpt"
)

// AllServices 返回所有默认检测平台列表。
func AllServices() []Service {
	return []Service{ServiceNetflix, ServiceDisneyPlus, ServiceYouTube, ServiceTikTok, ServiceReddit, ServiceChatGPT}
}

// Result 是单个平台的解锁检测结果。
type Result struct {
	Service Service `json:"service"`
	Status  string  `json:"status"`  // unlocked / locked / error / unknown
	Region  string  `json:"region"`  // 检测到的地区码（如 US/JP/HK），空表示未知
	Detail  string  `json:"detail"`  // 原始判定摘要
}

// Detector 执行流媒体解锁检测。
type Detector struct {
	client  *http.Client
	timeout time.Duration
}

// NewDetector 创建 Detector，每个探测请求超时 10s。
func NewDetector(timeout time.Duration) *Detector {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Detector{
		client:  &http.Client{Timeout: timeout},
		timeout: timeout,
	}
}

// ProbeAll 对指定平台列表全部执行一次检测，返回结果切片。
func (d *Detector) ProbeAll(ctx context.Context, services []Service) []Result {
	var results []Result
	for _, svc := range services {
		r := d.probeOne(ctx, svc)
		results = append(results, r)
	}
	return results
}

// ProbeDefault 对默认的 6 个平台执行检测。
func (d *Detector) ProbeDefault(ctx context.Context) []Result {
	return d.ProbeAll(ctx, AllServices())
}

func (d *Detector) probeOne(ctx context.Context, svc Service) Result {
	switch svc {
	case ServiceNetflix:
		return d.probeNetflix(ctx)
	case ServiceDisneyPlus:
		return d.probeDisneyPlus(ctx)
	case ServiceYouTube:
		return d.probeYouTube(ctx)
	case ServiceTikTok:
		return d.probeTikTok(ctx)
	case ServiceReddit:
		return d.probeReddit(ctx)
	case ServiceChatGPT:
		return d.probeChatGPT(ctx)
	default:
		return Result{Service: svc, Status: "unknown", Detail: fmt.Sprintf("unrecognized service: %s", svc)}
	}
}

// probeNetflix 检测 Netflix 解锁状态。
// 参考 IPQuality: 同时请求两个 title，任一返回 region 即解锁；都返回 Not Available 则仅 DNS 解锁。
func (d *Detector) probeNetflix(ctx context.Context) Result {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	urls := []string{
		"https://www.netflix.com/title/81280792",
		"https://www.netflix.com/title/70143836",
	}
	var foundRegion string
	notAvailable := 0
	var lastErr string
	for _, url := range urls {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", ua)
		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
		resp.Body.Close()
		s := string(body)
		if strings.Contains(s, "Not Available") || strings.Contains(s, "not available") || strings.Contains(s, "Oh no") {
			notAvailable++
			continue
		}
		// 直接提取 countryName 字段值，格式: ..."countryName":"Japan"...
		idx := strings.Index(s, `"countryName":"`)
		if idx > 0 {
			tail := s[idx+len(`"countryName":"`):]
			if end := strings.Index(tail, `"`); end > 0 {
				foundRegion = tail[:end]
			}
		}
	}
	if foundRegion != "" {
		return Result{Service: ServiceNetflix, Status: "unlocked", Region: foundRegion, Detail: "native unlock"}
	}
	if notAvailable >= 2 {
		return Result{Service: ServiceNetflix, Status: "locked", Detail: "not available"}
	}
	if notAvailable > 0 {
		return Result{Service: ServiceNetflix, Status: "locked", Detail: "dns only"}
	}
	if lastErr != "" {
		return Result{Service: ServiceNetflix, Status: "error", Detail: lastErr}
	}
	return Result{Service: ServiceNetflix, Status: "error", Detail: "no countryName in response"}
}

// probeDisneyPlus 检测 Disney+ 解锁状态。
func (d *Detector) probeDisneyPlus(ctx context.Context) Result {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	bearer := "ZGlzbmV5JmJyb3dzZXImMS4wLjA.Cu56AgSfBTDag5NiRA81oLHkDZfu5L3CKadnefEAY84"
	// 第一步：获取 assertion
	body := `{"deviceFamily":"browser","applicationRuntime":"chrome","deviceProfile":"windows","attributes":{}}`
	req1, _ := http.NewRequestWithContext(ctx, "POST", "https://disney.api.edge.bamgrid.com/devices", strings.NewReader(body))
	req1.Header.Set("User-Agent", ua)
	req1.Header.Set("Authorization", "Bearer "+bearer)
	req1.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp1, err := d.client.Do(req1)
	if err != nil {
		return Result{Service: ServiceDisneyPlus, Status: "error", Detail: fmt.Sprintf("assertion request: %v", err)}
	}
	var assertionResp struct {
		Assertion string `json:"assertion"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&assertionResp); err != nil {
		resp1.Body.Close()
		return Result{Service: ServiceDisneyPlus, Status: "error", Detail: fmt.Sprintf("assertion decode: %v", err)}
	}
	resp1.Body.Close()
	if assertionResp.Assertion == "" {
		return Result{Service: ServiceDisneyPlus, Status: "error", Detail: "empty assertion"}
	}

	// 第二步：token-exchange，subject_token_type 用 bamtech device token-type（URL 编码）
	tokenBody := fmt.Sprintf(
		"grant_type=urn%%3Aietf%%3Aparams%%3Aoauth%%3Agrant-type%%3Atoken-exchange&latitude=0&longitude=0&platform=browser&subject_token=%s&subject_token_type=urn%%3Abamtech%%3Aparams%%3Aoauth%%3Atoken-type%%3Adevice",
		url.QueryEscape(assertionResp.Assertion),
	)
	req2, _ := http.NewRequestWithContext(ctx, "POST", "https://disney.api.edge.bamgrid.com/token", strings.NewReader(tokenBody))
	req2.Header.Set("User-Agent", ua)
	req2.Header.Set("Authorization", "Bearer "+bearer)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := d.client.Do(req2)
	if err != nil {
		return Result{Service: ServiceDisneyPlus, Status: "error", Detail: fmt.Sprintf("token request: %v", err)}
	}
	tokenContent, _ := io.ReadAll(io.LimitReader(resp2.Body, 8192))
	resp2.Body.Close()
	s := string(tokenContent)
	if strings.Contains(s, "forbidden-location") {
		return Result{Service: ServiceDisneyPlus, Status: "locked", Detail: "forbidden location"}
	}
	var tokenResp struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(tokenContent, &tokenResp); err != nil || tokenResp.Error != "" {
		return Result{Service: ServiceDisneyPlus, Status: "error", Detail: fmt.Sprintf("token error: %s", tokenResp.Error)}
	}

	// 第三步：graphql 查询地区
	if tokenResp.RefreshToken == "" {
		return Result{Service: ServiceDisneyPlus, Status: "error", Detail: "no refresh token"}
	}
	graphqlBody := fmt.Sprintf(
		`{"query":"mutation refreshToken($input: RefreshTokenInput!) { refreshToken(refreshToken: $input) { activeSession { sessionId } } }","variables":{"input":{"refreshToken":%q}}}`,
		tokenResp.RefreshToken,
	)
	req3, _ := http.NewRequestWithContext(ctx, "POST", "https://disney.api.edge.bamgrid.com/graph/v1/device/graphql", strings.NewReader(graphqlBody))
	req3.Header.Set("User-Agent", ua)
	req3.Header.Set("Authorization", bearer)
	req3.Header.Set("Content-Type", "application/json")
	resp3, err := d.client.Do(req3)
	if err != nil {
		return Result{Service: ServiceDisneyPlus, Status: "unlocked", Detail: "token ok but graphql failed"}
	}
	graphqlContent, _ := io.ReadAll(io.LimitReader(resp3.Body, 8192))
	resp3.Body.Close()

	// 提取地区与支持状态
	var graphqlResp struct {
		Extensions struct {
			SDK struct {
				Session struct {
					CountryCode       string `json:"countryCode"`
					InSupportedLocation bool   `json:"inSupportedLocation"`
				} `json:"session"`
			} `json:"sdk"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(graphqlContent, &graphqlResp); err != nil {
		return Result{Service: ServiceDisneyPlus, Status: "unlocked", Region: d.fetchRegion(ctx), Detail: "native unlock"}
	}
	region := graphqlResp.Extensions.SDK.Session.CountryCode
	if region == "" {
		region = d.fetchRegion(ctx)
	}
	if graphqlResp.Extensions.SDK.Session.InSupportedLocation {
		return Result{Service: ServiceDisneyPlus, Status: "unlocked", Region: region, Detail: "native unlock"}
	}
	if region == "" {
		return Result{Service: ServiceDisneyPlus, Status: "unlocked", Detail: "native unlock (region unknown)"}
	}
	return Result{Service: ServiceDisneyPlus, Status: "locked", Region: region, Detail: "not supported location"}
}

// probeYouTube 检测 YouTube Premium 解锁状态。
func (d *Detector) probeYouTube(ctx context.Context) Result {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.youtube.com/premium", nil)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("Cookie", "YSC=BiCUU3-5Gdk; CONSENT=YES+cb.20220301-11-p0.en+FX+700; GPS=1; VISITOR_INFO1_LIVE=4VwPMkB7W5A; PREF=tz=Asia.Shanghai")
	resp, err := d.client.Do(req)
	if err != nil {
		return Result{Service: ServiceYouTube, Status: "error", Detail: fmt.Sprintf("request: %v", err)}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	resp.Body.Close()
	s := string(body)
	if strings.Contains(s, "www.google.cn") {
		return Result{Service: ServiceYouTube, Status: "locked", Region: "CN", Detail: "China region"}
	}
	// 提取 countryCode（位于 HTML 深处，约 850KB 处）
	idx := strings.Index(s, `"countryCode":"`)
	if idx > 0 {
		tail := s[idx+len(`"countryCode":"`):]
		if end := strings.Index(tail, `"`); end > 0 {
			region := tail[:end]
			return Result{Service: ServiceYouTube, Status: "unlocked", Region: region, Detail: "premium available"}
		}
	}
	return Result{Service: ServiceYouTube, Status: "unlocked", Region: d.fetchRegion(ctx), Detail: "premium available"}
}

// probeTikTok 检测 TikTok 解锁状态。
// 参考 RegionRestrictionCheck: GET / 检查重定向；POST /passport/web/store_region/ 获取地区。
func (d *Detector) probeTikTok(ctx context.Context) Result {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	// 第一步：检查重定向，确定是否被墙/被 DNS 劫持
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.tiktok.com/", nil)
	req.Header.Set("User-Agent", ua)
	resp, err := d.client.Do(req)
	if err != nil {
		return Result{Service: ServiceTikTok, Status: "error", Detail: fmt.Sprintf("request: %v", err)}
	}
	finalURL := resp.Request.URL.String()
	resp.Body.Close()

	if !strings.Contains(finalURL, "tiktok.com") {
		return Result{Service: ServiceTikTok, Status: "locked", Detail: "blocked/redirected"}
	}

	// 第二步：POST store_region 获取地区
	payload := strings.NewReader("region=JP")
	req2, _ := http.NewRequestWithContext(ctx, "POST", "https://www.tiktok.com/passport/web/store_region/", payload)
	req2.Header.Set("User-Agent", ua)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Origin", "https://www.tiktok.com")
	req2.Header.Set("Referer", "https://www.tiktok.com/")
	resp2, err := d.client.Do(req2)
	if err != nil {
		return Result{Service: ServiceTikTok, Status: "error", Detail: fmt.Sprintf("region api: %v", err)}
	}
	body, _ := io.ReadAll(io.LimitReader(resp2.Body, 16384))
	resp2.Body.Close()

	// 如果响应是 HTML（反爬/重定向），视为 locked
	if !json.Valid(body) {
		return Result{Service: ServiceTikTok, Status: "locked", Detail: "blocked (anti-crawl)"}
	}

	var r struct {
		Data struct {
			StoreRegion string `json:"store_region"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return Result{Service: ServiceTikTok, Status: "locked", Detail: "blocked (anti-crawl)"}
	}

	if r.Data.StoreRegion != "" {
		return Result{Service: ServiceTikTok, Status: "unlocked", Region: r.Data.StoreRegion, Detail: "native unlock"}
	}
	return Result{Service: ServiceTikTok, Status: "locked", Detail: fmt.Sprintf("msg: %s", r.Message)}
}

// probeReddit 检测 Reddit 解锁状态。
func (d *Detector) probeReddit(ctx context.Context) Result {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.reddit.com/svc/shreddit/reddit-chat", nil)
	req.Header.Set("User-Agent", ua)
	resp, err := d.client.Do(req)
	if err != nil {
		return Result{Service: ServiceReddit, Status: "error", Detail: fmt.Sprintf("request: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		s := string(body)
		// country="JP" 在 body 约 290KB 处
		idx := strings.Index(s, `country="`)
		if idx > 0 {
			tail := s[idx+9:]
			if end := strings.Index(tail, `"`); end > 0 {
				region := tail[:end]
				return Result{Service: ServiceReddit, Status: "unlocked", Region: region, Detail: "accessible"}
			}
		}
		return Result{Service: ServiceReddit, Status: "unlocked", Region: d.fetchRegion(ctx), Detail: "accessible"}
	}
	if resp.StatusCode == 403 {
		return Result{Service: ServiceReddit, Status: "locked", Detail: "403 forbidden"}
	}
	return Result{Service: ServiceReddit, Status: "locked", Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// probeChatGPT 检测 ChatGPT/OpenAI 解锁状态。
func (d *Detector) probeChatGPT(ctx context.Context) Result {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Edg/119.0.0.0"
	req1, _ := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/compliance/cookie_requirements", nil)
	req1.Header.Set("User-Agent", ua)
	req1.Header.Set("Authorization", "Bearer null")
	req1.Header.Set("Origin", "https://platform.openai.com")
	req1.Header.Set("Referer", "https://platform.openai.com/")
	resp1, err := d.client.Do(req1)
	if err != nil {
		return Result{Service: ServiceChatGPT, Status: "error", Detail: fmt.Sprintf("api request: %v", err)}
	}
	body1, _ := io.ReadAll(io.LimitReader(resp1.Body, 4096))
	resp1.Body.Close()
	s1 := string(body1)
	if strings.Contains(s1, "unsupported_country") {
		return Result{Service: ServiceChatGPT, Status: "locked", Detail: "unsupported country"}
	}
	// 二次确认：访问 ios.chat.openai.com
	req2, _ := http.NewRequestWithContext(ctx, "GET", "https://ios.chat.openai.com/", nil)
	req2.Header.Set("User-Agent", ua)
	resp2, err := d.client.Do(req2)
	if err != nil {
		return Result{Service: ServiceChatGPT, Status: "unlocked", Detail: "api accessible (ios check failed)"}
	}
	body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 4096))
	resp2.Body.Close()
	s2 := string(body2)
	if strings.Contains(s2, "VPN") || strings.Contains(s2, "unsupported_country") {
		return Result{Service: ServiceChatGPT, Status: "locked", Detail: "VPN blocked"}
	}
	// 通过 Cloudflare trace 获取出口地区（loc=JP）
	region := d.fetchRegion(ctx)
	return Result{Service: ServiceChatGPT, Status: "unlocked", Region: region, Detail: "accessible"}
}

// fetchRegion 通过 ipinfo.io/country 获取出口 IP 地区码（如 JP/US）。
// 失败时返回空字符串。
func (d *Detector) fetchRegion(ctx context.Context) string {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://ipinfo.io/country", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := d.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	region := strings.TrimSpace(string(body))
	if len(region) == 2 {
		return strings.ToUpper(region)
	}
	return ""
}
