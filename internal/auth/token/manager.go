// 文件路径: internal/auth/token/manager.go
// 模块说明: 这是 internal 模块里的 manager 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package token

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Manager 负责签发和校验 JWT，用于 Passport 兼容流程。
type Manager struct {
	method   jwt.SigningMethod
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	leeway   time.Duration
}

// Options 配置 Token 管理器。
type Options struct {
	SigningKey []byte
	Issuer     string
	Audience   string
	TTL        time.Duration
	Leeway     time.Duration
	SigningAlg string
}

// Claims 包含 JWT 标准声明及令牌元数据。
type Claims struct {
	jwt.RegisteredClaims
	TokenType  string         `json:"token_type,omitempty"`
	SessionID  string         `json:"sid,omitempty"`
	Attributes map[string]any `json:"attr,omitempty"`
}

// IssueInput 定义签发令牌时的可覆盖参数。
type IssueInput struct {
	Subject    string
	TokenType  string
	SessionID  string
	Audience   string
	TTL        time.Duration
	Attributes map[string]any
}

var (
	// ErrInvalidToken 表示解析或校验失败。
	ErrInvalidToken = errors.New("invalid token / 无效的 token")
	// ErrExpiredToken 表示令牌超出允许的过期宽限。
	ErrExpiredToken = errors.New("token expired / token 已过期")
)

// NewManager 组装 JWT 管理器；未指定 SigningAlg 时默认使用 HS256。
func NewManager(opts Options) (*Manager, error) {
	if len(opts.SigningKey) == 0 {
		return nil, fmt.Errorf("signing key is required / 签名密钥不能为空")
	}
	method := jwt.GetSigningMethod(strings.ToUpper(strings.TrimSpace(opts.SigningAlg)))
	if method == nil {
		method = jwt.SigningMethodHS256
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	leeway := opts.Leeway
	if leeway < 0 {
		leeway = 0
	}
	return &Manager{
		method:   method,
		secret:   append([]byte(nil), opts.SigningKey...),
		issuer:   strings.TrimSpace(opts.Issuer),
		audience: strings.TrimSpace(opts.Audience),
		ttl:      ttl,
		leeway:   leeway,
	}, nil
}

// MustManager 在参数非法时直接 panic，用于启动期默认配置。
func MustManager(opts Options) *Manager {
	m, err := NewManager(opts)
	if err != nil {
		panic(err)
	}
	return m
}

// Issue 使用默认配置签发 JWT，并支持可选覆盖项。
func (m *Manager) Issue(input IssueInput) (string, *Claims, error) {
	if m == nil {
		return "", nil, fmt.Errorf("token manager not initialized / token 管理器未初始化")
	}
	if strings.TrimSpace(input.Subject) == "" {
		return "", nil, fmt.Errorf("token subject is required / token subject 不能为空")
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = m.ttl
	}
	audience := strings.TrimSpace(input.Audience)
	if audience == "" {
		audience = m.audience
	}

	now := time.Now().UTC()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   input.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		TokenType:  input.TokenType,
		SessionID:  input.SessionID,
		Attributes: cloneMap(input.Attributes),
	}
	if audience != "" {
		claims.Audience = jwt.ClaimStrings{audience}
	}

	token := jwt.NewWithClaims(m.method, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", nil, fmt.Errorf("sign token: %w", err)
	}
	return signed, claims, nil
}

// Parse 校验 JWT 字符串并返回解析后的声明。
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	if m == nil {
		return nil, fmt.Errorf("token manager not initialized / token 管理器未初始化")
	}
	claims := &Claims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{m.method.Alg()}))
	parsed, err := parser.ParseWithClaims(tokenString, claims, func(tok *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !parsed.Valid {
		return nil, ErrInvalidToken
	}
	if err := m.validateClaims(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// Refresh 基于现有 Claims 重新签发 Token。
func (m *Manager) Refresh(claims *Claims, ttl time.Duration) (string, *Claims, error) {
	if claims == nil {
		return "", nil, fmt.Errorf("claims is required / claims 不能为空")
	}
	audience := ""
	if len(claims.Audience) > 0 {
		audience = claims.Audience[0]
	}
	input := IssueInput{
		Subject:    claims.Subject,
		TokenType:  claims.TokenType,
		SessionID:  claims.SessionID,
		Audience:   audience,
		TTL:        ttl,
		Attributes: cloneMap(claims.Attributes),
	}
	return m.Issue(input)
}

// validateClaims 校验 JWT 标准声明。
func (m *Manager) validateClaims(claims *Claims) error {
	now := time.Now().UTC()
	if claims.ExpiresAt == nil || now.After(claims.ExpiresAt.Add(m.leeway)) {
		return ErrExpiredToken
	}
	if claims.IssuedAt != nil && claims.IssuedAt.Time.After(now.Add(m.leeway)) {
		return ErrInvalidToken
	}
	if claims.NotBefore != nil && now.Add(m.leeway).Before(claims.NotBefore.Time) {
		return ErrInvalidToken
	}
	if m.issuer != "" && claims.Issuer != m.issuer {
		return ErrInvalidToken
	}
	if m.audience != "" {
		allowed := false
		for _, aud := range claims.Audience {
			if aud == m.audience {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrInvalidToken
		}
	}
	return nil
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
