package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/agent/secrets"
)

// realitySecretProvider 提供按 inbound tag 索引的本机 Reality 密钥。
type realitySecretProvider interface {
	RealityFor(tag string) (secrets.RealitySecret, error)
}

// injectRealitySecrets 把本机为每个配置独立生成的 Reality 密钥注入渲染产物。
//
// 只补空缺：产物里已经带 private_key/privateKey 的 inbound 原样保留（例如非 spec
// 来源的配置）。这样面板可以完全不下发私钥，由 agent 按 tag 各自注入，
// 不同机器、不同配置之间密钥互不复用。
func injectRealitySecrets(content []byte, provider realitySecretProvider) ([]byte, error) {
	if provider == nil || len(content) == 0 {
		return content, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err != nil {
		// 非 JSON 对象（或数组根）不处理：上游已完成 JSON 校验。
		return content, nil
	}
	inbounds, ok := doc["inbounds"].([]any)
	if !ok || len(inbounds) == 0 {
		return content, nil
	}

	changed := false
	for _, raw := range inbounds {
		inbound, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := inbound["tag"].(string)

		// xray 形态：streamSettings.security=reality + realitySettings
		if stream, ok := inbound["streamSettings"].(map[string]any); ok {
			injected, err := injectXrayReality(stream, tag, provider)
			if err != nil {
				return nil, err
			}
			changed = changed || injected
		}
		// sing-box 形态：tls.reality
		if tlsSection, ok := inbound["tls"].(map[string]any); ok {
			injected, err := injectSingBoxReality(tlsSection, tag, provider)
			if err != nil {
				return nil, err
			}
			changed = changed || injected
		}
	}

	if !changed {
		return content, nil
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config with injected reality secrets: %w", err)
	}
	return out, nil
}

// injectSecretsIntoFiles 对快照模式的每个文件注入本机 Reality 密钥。
func (m *Manager) injectSecretsIntoFiles(files []normalizedStagedApplyFile) error {
	if m == nil || m.secrets == nil {
		return nil
	}
	for i := range files {
		injected, err := injectRealitySecrets(files[i].content, m.secrets)
		if err != nil {
			return fmt.Errorf("inject secrets into %s: %w", files[i].filename, err)
		}
		files[i].content = injected
	}
	return nil
}

// injectSecretsIntoOps 对 patch 模式的 upsert 内容注入本机 Reality 密钥。
func (m *Manager) injectSecretsIntoOps(ops []normalizedPatchOperation) error {
	if m == nil || m.secrets == nil {
		return nil
	}
	for i := range ops {
		if ops[i].typ != StagedApplyPatchOperationUpsert {
			continue
		}
		injected, err := injectRealitySecrets(ops[i].content, m.secrets)
		if err != nil {
			return fmt.Errorf("inject secrets into %s: %w", ops[i].filename, err)
		}
		ops[i].content = injected
	}
	return nil
}

// injectSingBoxReality 给 sing-box 入站的 tls.reality 补本机密钥。
func injectSingBoxReality(tlsSection map[string]any, tag string, provider realitySecretProvider) (bool, error) {
	reality, ok := tlsSection["reality"].(map[string]any)
	if !ok || reality == nil {
		return false, nil
	}
	if enabled, ok := reality["enabled"].(bool); ok && !enabled {
		return false, nil
	}
	if existing, _ := reality["private_key"].(string); strings.TrimSpace(existing) != "" {
		return false, nil
	}

	secret, err := provider.RealityFor(tag)
	if err != nil {
		return false, fmt.Errorf("generate reality secret for inbound %q: %w", tag, err)
	}
	reality["private_key"] = secret.PrivateKey
	reality["short_id"] = []any{secret.ShortID}
	return true, nil
}

// injectXrayReality 给 xray 入站的 streamSettings.realitySettings 补本机密钥。
func injectXrayReality(stream map[string]any, tag string, provider realitySecretProvider) (bool, error) {
	security, _ := stream["security"].(string)
	if !strings.EqualFold(strings.TrimSpace(security), "reality") {
		return false, nil
	}
	realitySettings, ok := stream["realitySettings"].(map[string]any)
	if !ok || realitySettings == nil {
		return false, nil
	}
	if existing, _ := realitySettings["privateKey"].(string); strings.TrimSpace(existing) != "" {
		return false, nil
	}

	secret, err := provider.RealityFor(tag)
	if err != nil {
		return false, fmt.Errorf("generate reality secret for inbound %q: %w", tag, err)
	}
	realitySettings["privateKey"] = secret.PrivateKey
	realitySettings["shortIds"] = []any{secret.ShortID}
	return true, nil
}
