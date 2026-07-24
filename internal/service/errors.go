// 文件路径: internal/service/errors.go
// 模块说明: 这是 internal 模块里的 errors 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import "errors"

var (
	// ErrNotFound indicates requested resource does not exist.
	ErrNotFound = errors.New("service: not found / 未找到资源")
	// ErrBadRequest indicates provided parameters are invalid.
	ErrBadRequest = errors.New("service: bad request / 请求参数无效")
	// ErrSystemSettingsInvalid indicates system settings payload validation failed.
	ErrSystemSettingsInvalid = errors.New("service: system settings invalid / 系统设置参数无效")
	// ErrInvalidCredentials indicates provided credentials are wrong.
	ErrInvalidCredentials = errors.New("service: invalid credentials / 凭证无效")
	// ErrRateLimited indicates caller exceeded allowed attempts.
	ErrRateLimited = errors.New("service: rate limited / 请求过于频繁")
	// ErrAccountDisabled indicates the account is disabled or banned.
	ErrAccountDisabled = errors.New("service: account disabled / 账号已禁用")
	// ErrUnauthorized indicates missing or invalid auth tokens.
	ErrUnauthorized = errors.New("service: unauthorized / 未授权")
	// ErrInvalidCommunicationKey indicates registration key mismatch.
	ErrInvalidCommunicationKey = errors.New("service: invalid communication key / 通信密钥无效")
	// ErrInvalidRefreshToken indicates refresh token problems.
	ErrInvalidRefreshToken = errors.New("service: invalid refresh token / 刷新令牌无效")
	// ErrInvalidCaptcha indicates captcha validation failed.
	ErrInvalidCaptcha = errors.New("service: invalid captcha / 验证码无效")
	// ErrEmailDomainNotAllowed indicates email suffix not permitted.
	ErrEmailDomainNotAllowed = errors.New("service: email domain not allowed / 邮箱域名不允许")
	// ErrCooldownActive indicates the action is cooling down.
	ErrCooldownActive = errors.New("service: cooldown active / 冷却中")
	// ErrInvalidEmail indicates malformed email inputs.
	ErrInvalidEmail = errors.New("service: invalid email / 邮箱无效")
	// ErrInvalidVerificationCode indicates provided verification code mismatch or expired.
	ErrInvalidVerificationCode = errors.New("service: invalid verification code / 验证码无效")
	// ErrInvalidPassword indicates password does not meet requirements.
	ErrInvalidPassword = errors.New("service: invalid password / 密码无效")
	// ErrInvalidUsername indicates username does not meet requirements.
	ErrInvalidUsername = errors.New("service: invalid username / 用户名无效")
	// ErrIdentifierRequired indicates email or username must be provided.
	ErrIdentifierRequired = errors.New("service: email or username required / 需要邮箱或用户名")
	// ErrRegistrationClosed indicates registering is disabled.
	ErrRegistrationClosed = errors.New("service: registration closed / 注册已关闭")
	// ErrEmailExists indicates email already registered.
	ErrEmailExists = errors.New("service: email already exists / 邮箱已存在")
	// ErrUsernameExists indicates username already registered.
	ErrUsernameExists = errors.New("service: username already exists / 用户名已存在")
	// ErrFeatureDisabled indicates feature is not enabled in settings.
	ErrFeatureDisabled = errors.New("service: feature disabled / 功能已禁用")
	// ErrInvalidToken indicates temporary token does not exist or expired.
	ErrInvalidToken = errors.New("service: invalid token / 令牌无效")
	// ErrInvalidServerType indicates node_type not recognized.
	ErrInvalidServerType = errors.New("service: invalid server type / 节点类型无效")
	// ErrInvalidPeriod indicates the requested plan period has no price.
	ErrInvalidPeriod = errors.New("service: invalid plan period / 套餐周期无效")
	// ErrPlanSoldOut indicates the plan capacity has been reached.
	ErrPlanSoldOut = errors.New("service: plan sold out / 套餐已售罄")
	// ErrPlanUnavailable indicates the plan cannot be purchased under current conditions.
	ErrPlanUnavailable = errors.New("service: plan unavailable / 套餐不可用")
	// ErrResetTrafficNotAllowed indicates data reset packages cannot be purchased.
	ErrResetTrafficNotAllowed = errors.New("service: reset traffic not allowed / 不允许重置流量")
	// ErrUserNotEligible indicates the user cannot access subscription data.
	ErrUserNotEligible = errors.New("service: user not eligible for subscription / 用户不满足订阅条件")
	// ErrNotImplemented indicates functionality has not been ported yet.
	ErrNotImplemented = errors.New("service: not implemented / 功能未实现")
	// ErrAlreadyInitialized indicates the install wizard should not run again.
	ErrAlreadyInitialized = errors.New("service: already initialized / 已完成初始化")
	// ErrTokenRevoked indicates a refresh token has already been rotated (reuse detected).
	ErrTokenRevoked = errors.New("service: token revoked / 令牌已被吊销")
)

// ErrLanguageRequired indicates the language field is required but missing.
var ErrLanguageRequired = errors.New("language is required")

// ErrRequiredField indicates a required field is missing.
var ErrRequiredField = errors.New("required field missing")

// ErrFieldsMissing indicates required fields are missing.
var ErrFieldsMissing = errors.New("required fields missing")
