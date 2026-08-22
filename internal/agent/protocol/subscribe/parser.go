package subscribe

import (
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

// Parser 解析订阅目录中的客户端配置。
type Parser struct {
	subscribeDir string
	lastHash     string // 缓存上次 Hash，用于变更检测
}

// NewParser 创建订阅目录解析器。
func NewParser(subscribeDir string) *Parser {
	return &Parser{
		subscribeDir: subscribeDir,
	}
}

// Parse 解析订阅文件并返回客户端配置。
func (p *Parser) Parse() (*SubscribeData, error) {
	// 检查订阅目录是否存在
	if _, err := os.Stat(p.subscribeDir); os.IsNotExist(err) {
		slog.Debug("Subscribe directory does not exist", "dir", p.subscribeDir)
		return &SubscribeData{}, nil
	}

	// 读取全部文件并计算内容 Hash
	files := map[string][]byte{}
	entries, err := os.ReadDir(p.subscribeDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(p.subscribeDir, entry.Name()))
		if err != nil {
			slog.Warn("Failed to read subscribe file", "file", entry.Name(), "error", err)
			continue
		}
		files[entry.Name()] = content
	}

	// 计算内容 Hash 用于变更检测
	contentHash := p.calculateHash(files)

	// 优先解析 proxies 文件（Clash YAML，信息最完整）
	var configs []ClientConfig
	if proxiesContent, ok := files["proxies"]; ok {
		slog.Info("Found proxies file, parsing...")
		parsed, err := ParseClash(proxiesContent)
		if err != nil {
			slog.Warn("Failed to parse proxies file", "error", err)
		} else {
			slog.Info("Parsed proxies file", "count", len(parsed))
			configs = parsed
		}
	} else {
		slog.Info("No proxies file found")
	}

	// 若无 proxies 文件则尝试 v2rayn
	if len(configs) == 0 {
		if v2raynContent, ok := files["v2rayn"]; ok {
			parsed, err := ParseV2RayN(v2raynContent)
			if err != nil {
				slog.Warn("Failed to parse v2rayn file", "error", err)
			} else {
				configs = parsed
			}
		}
	}

	// 为解析结果附加原始配置
	rawConfigs := p.collectRawConfigs(files)
	for i := range configs {
		// 按名称匹配并附加对应的原始配置
		configs[i].RawConfigs = p.matchRawConfigs(configs[i], rawConfigs, files)
	}

	return &SubscribeData{
		Configs:     configs,
		ContentHash: contentHash,
	}, nil
}

// HasChanged 判断订阅目录内容是否有变化。
func (p *Parser) HasChanged() (bool, error) {
	data, err := p.Parse()
	if err != nil {
		return false, err
	}

	if data.ContentHash != p.lastHash {
		p.lastHash = data.ContentHash
		return true, nil
	}
	return false, nil
}

// calculateHash 计算所有文件内容的 MD5 Hash。
func (p *Parser) calculateHash(files map[string][]byte) string {
	// 对 key 排序保证 Hash 一致性
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := md5.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write(files[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// collectRawConfigs 按格式收集原始配置内容。
func (p *Parser) collectRawConfigs(files map[string][]byte) map[FormatType]string {
	result := make(map[FormatType]string)

	formatMap := map[string]FormatType{
		"v2rayn":         FormatV2RayN,
		"proxies":        FormatClash,
		"sing-box-pc":    FormatSingBoxPC,
		"sing-box-phone": FormatSingBoxPhone,
		"shadowrocket":   FormatShadowrocket,
		"neko":           FormatNeko,
	}

	for filename, content := range files {
		if format, ok := formatMap[filename]; ok {
			result[format] = string(content)
		}
	}

	return result
}

// matchRawConfigs 按协议匹配并提取原始配置条目。
func (p *Parser) matchRawConfigs(config ClientConfig, rawConfigs map[FormatType]string, files map[string][]byte) map[string]string {
	result := make(map[string]string)

	// 对 v2rayn 格式，提取与协议名称匹配的单行
	if v2rayn, ok := rawConfigs[FormatV2RayN]; ok {
		line := extractV2RayNLine(v2rayn, config.Name)
		if line != "" {
			result[string(FormatV2RayN)] = line
		}
	}

	// 对 sing-box 格式，提取与 tag 匹配的 outbound
	if singboxPC, ok := files["sing-box-pc"]; ok {
		outbound := extractSingBoxOutbound(singboxPC, config.Name)
		if outbound != "" {
			result[string(FormatSingBoxPC)] = outbound
		}
	}

	if singboxPhone, ok := files["sing-box-phone"]; ok {
		outbound := extractSingBoxOutbound(singboxPhone, config.Name)
		if outbound != "" {
			result[string(FormatSingBoxPhone)] = outbound
		}
	}

	// 对 Clash 格式，提取对应代理条目
	if clash, ok := rawConfigs[FormatClash]; ok {
		entry := extractClashProxy(clash, config.Name)
		if entry != "" {
			result[string(FormatClash)] = entry
		}
	}

	return result
}
