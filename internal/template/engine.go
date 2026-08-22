package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// Engine 负责模板渲染。
type Engine struct {
	funcMap template.FuncMap
}

// NewEngine 创建模板引擎并注册内置函数。
func NewEngine() *Engine {
	return &Engine{
		funcMap: DefaultFuncMap(),
	}
}

// Render 渲染模板并校验输出 JSON。
func (e *Engine) Render(tmplContent string, ctx *TemplateContext) ([]byte, error) {
	tmpl, err := template.New("config").Funcs(e.funcMap).Parse(tmplContent)
	if err != nil {
		return nil, &TemplateError{
			Type:    ErrTemplateSyntax,
			Message: err.Error(),
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, &TemplateError{
			Type:    ErrTemplateExecution,
			Message: err.Error(),
		}
	}

	// 校验 JSON 输出
	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		return nil, &TemplateError{
			Type:    ErrInvalidJSON,
			Message: err.Error(),
			Details: buf.String(),
		}
	}

	// 格式化输出
	prettyJSON, err := json.MarshalIndent(jsonCheck, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format JSON: %w", err)
	}

	return prettyJSON, nil
}

// RenderRaw 渲染模板但不校验 JSON（用于调试）。
func (e *Engine) RenderRaw(tmplContent string, ctx *TemplateContext) ([]byte, error) {
	tmpl, err := template.New("config").Funcs(e.funcMap).Parse(tmplContent)
	if err != nil {
		return nil, &TemplateError{
			Type:    ErrTemplateSyntax,
			Message: err.Error(),
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, &TemplateError{
			Type:    ErrTemplateExecution,
			Message: err.Error(),
		}
	}

	return buf.Bytes(), nil
}

// PreviewRender 渲染模板并填充示例数据。
func (e *Engine) PreviewRender(tmplContent string) ([]byte, error) {
	sampleCtx := e.createSampleContext()
	return e.Render(tmplContent, sampleCtx)
}

// createSampleContext 生成模板预览的示例上下文。
func (e *Engine) createSampleContext() *TemplateContext {
	return &TemplateContext{
		Inbounds: []InboundConfig{
			{
				Type:       "vless",
				Tag:        "vless-in",
				Listen:     "::",
				ListenPort: 443,
				Users: []InboundUser{
					{UUID: "00000000-0000-0000-0000-000000000001", Name: "user@example.com"},
				},
				TLS: &TLSConfig{
					Enabled:    true,
					ServerName: "example.com",
					Reality: &RealityConfig{
						Enabled:    true,
						PrivateKey: "sample-private-key",
						ShortIDs:   []string{"0123456789abcdef"},
						Handshake: &HandshakeConfig{
							Server:     "www.google.com",
							ServerPort: 443,
						},
					},
				},
			},
			{
				Type:       "shadowsocks",
				Tag:        "ss-in",
				Listen:     "::",
				ListenPort: 8388,
				Users: []InboundUser{
					{Name: "user@example.com", Password: "password123", Method: "2022-blake3-aes-128-gcm"},
				},
			},
		},
		Outbounds: []OutboundConfig{
			{Type: "direct", Tag: "direct"},
			{Type: "block", Tag: "block"},
		},
		Users: []UserConfig{
			{ID: 1, UUID: "00000000-0000-0000-0000-000000000001", Email: "user@example.com", Enabled: true},
			{ID: 2, UUID: "00000000-0000-0000-0000-000000000002", Email: "user2@example.com", Enabled: true},
		},
		Agent: AgentInfo{
			ID:           1,
			Name:         "sample-agent",
			Host:         "127.0.0.1",
			CoreType:     "sing-box",
			CoreVersion:  "1.10.0",
			Capabilities: []string{"reality", "multiplex", "v2ray_api"},
			BuildTags:    []string{"with_v2ray_api", "with_quic"},
		},
		Server: ServerInfo{
			LogLevel:     "info",
			ListenAddr:   "::",
			DNSServer:    "8.8.8.8",
			StatsEnabled: true,
		},
	}
}

// AddFunc 注册模板函数。
func (e *Engine) AddFunc(name string, fn interface{}) {
	e.funcMap[name] = fn
}
