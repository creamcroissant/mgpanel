// 文件路径: internal/api/router.go
// 模块说明: 这是 internal 模块里的 router 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/api/handler"
	"github.com/creamcroissant/xboard/internal/api/middleware"
	"github.com/creamcroissant/xboard/internal/async"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/creamcroissant/xboard/internal/config"
)

func resolveRateLimitConfig() (middleware.RateLimitConfig, bool) {
	config := middleware.RateLimitConfig{
		Limit:     100,
		Window:    time.Minute,
		SkipPaths: []string{"/health", "/healthz", "/_internal/ready", "/metrics"},
	}
	enabled := true

	if raw := strings.TrimSpace(os.Getenv("XBOARD_RATE_LIMIT_DISABLED")); raw != "" {
		if raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") {
			enabled = false
		}
	}

	if raw := strings.TrimSpace(os.Getenv("XBOARD_RATE_LIMIT_LIMIT")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			if value <= 0 {
				enabled = false
			} else {
				config.Limit = value
			}
		}
	}

	if raw := strings.TrimSpace(os.Getenv("XBOARD_RATE_LIMIT_WINDOW_SECONDS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			config.Window = time.Duration(value) * time.Second
		}
	}

	return config, enabled
}

func corsConfigFromOptions(opts routerOptions) middleware.CORSConfig {
	if len(opts.corsAllowedOrigins) > 0 {
		return middleware.CORSConfig{
			AllowedOrigins:   opts.corsAllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
			ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
			AllowCredentials: false,
			MaxAge:           86400,
		}
	}
	return middleware.DefaultCORSConfig()
}

type Services struct {
	Config                  service.ConfigService
	User                    service.UserService
	UserStat                service.UserStatService
	UserKnowledge           service.UserKnowledgeService
	UserNotice              service.UserNoticeService
	Auth                    service.AuthService
	AdminPath               service.AdminPathService
	Install                 service.InstallService
	AdminServer             service.AdminServerService
	AdminNotice             service.AdminNoticeService
	AdminKnowledge          service.AdminKnowledgeService
	ServerAuth              service.ServerAuthService
	ServerNode              service.ServerNodeService
	Traffic                 service.ServerTrafficService
	Telemetry               service.ServerTelemetryService
	Verify                  service.VerificationService
	Password                service.PasswordService
	Register                service.RegistrationService
	MailLink                service.MailLinkService
	Comm                    service.CommService
	AdminPlan               service.AdminPlanService
	AdminUser               service.AdminUserService
	AdminStat               service.AdminStatService
	AdminNodeStat           service.AdminNodeStatService
	AdminSystem             service.AdminSystemService
	AdminSystemSettings     service.AdminSystemSettingsService
	AgentHost               service.AgentHostService
	AgentCore               service.AgentCoreService
	Forwarding              service.ForwardingService
	AccessLog               service.AccessLogService
	InboundSpec             service.InboundSpecService
	CoreConfigItem          service.CoreConfigItemService
	DriftAndDiff            service.DriftAndDiffService
	ApplyOrchestrator       service.ApplyOrchestratorService
	OperationLog            service.OperationLogService
	AgentLifecycleOperation service.AgentLifecycleOperationService
	AgentTrafficLifecycle   service.AgentTrafficLifecycleService
	BinaryVersion           service.BinaryVersionService
	Plan                    service.PlanService
	Server                  service.ServerService
	Subscription            service.SubscriptionService
	SubscriptionFilter      service.SubscriptionFilterService
	SubscriptionSource      service.SubscriptionSourceService
	UserSelection           service.UserServerSelectionService
	ShortLink               service.ShortLinkService
	MCPApiKeys              service.MCPApiKeyService
	CDN                     service.CDNService
	Mesh                    service.AgentMeshService
	TrafficQueue            *async.TrafficQueue
	SubLogQueue             *async.SubscriptionLogQueue
	I18n                    *i18n.Manager
}

// NewRouter wires minimal endpoints；其余 handler 会在后续逐步补齐。
func NewRouter(logger *slog.Logger, services Services, metricsCfg config.MetricsConfig, opts ...RouterOption) http.Handler {
	var options routerOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	// Validate required services — nil pointers cause NPE in handler constructors
	required := map[string]interface{}{
		"AccessLog":               services.AccessLog,
		"AdminKnowledge":          services.AdminKnowledge,
		"AdminNodeStat":           services.AdminNodeStat,
		"AdminNotice":             services.AdminNotice,
		"AdminPath":               services.AdminPath,
		"AdminPlan":               services.AdminPlan,
		"AdminServer":             services.AdminServer,
		"AdminStat":               services.AdminStat,
		"AdminSystem":             services.AdminSystem,
		"AdminSystemSettings":     services.AdminSystemSettings,
		"AdminUser":               services.AdminUser,
		"AgentCore":               services.AgentCore,
		"AgentHost":               services.AgentHost,
		"AgentLifecycleOperation": services.AgentLifecycleOperation,
		"AgentTrafficLifecycle":   services.AgentTrafficLifecycle,
		"ApplyOrchestrator":       services.ApplyOrchestrator,
		"Auth":                    services.Auth,
		"BinaryVersion":           services.BinaryVersion,
		"CDN":                     services.CDN,
		"Comm":                    services.Comm,
		"Config":                  services.Config,
		"CoreConfigItem":          services.CoreConfigItem,
		"DriftAndDiff":            services.DriftAndDiff,
		"Forwarding":              services.Forwarding,
		"I18n":                    services.I18n,
		"MCPApiKeys":           services.MCPApiKeys,
		"InboundSpec":             services.InboundSpec,
		"Install":                 services.Install,
		"MailLink":                services.MailLink,
		"Mesh":                    services.Mesh,
		"OperationLog":            services.OperationLog,
		"Password":                services.Password,
		"Plan":                    services.Plan,
		"Register":                services.Register,
		"Server":                  services.Server,
		"ServerAuth":              services.ServerAuth,
		"ServerNode":              services.ServerNode,
		"ShortLink":               services.ShortLink,
		"Subscription":            services.Subscription,
		"SubscriptionFilter":      services.SubscriptionFilter,
		"SubscriptionSource":      services.SubscriptionSource,
		"Telemetry":               services.Telemetry,
		"Traffic":                 services.Traffic,
		"TrafficQueue":            services.TrafficQueue,
		"User":                    services.User,
		"UserKnowledge":           services.UserKnowledge,
		"UserNotice":              services.UserNotice,
		"UserSelection":           services.UserSelection,
		"UserStat":                services.UserStat,
		"Verify":                  services.Verify,
	}
	for name, svc := range required {
		if svc == nil {
			panic(fmt.Sprintf("router: required service %q is nil", name))
		}
	}

	r := chi.NewRouter()

	// Initialize Prometheus metrics
	mCfg := middleware.DefaultMetricsConfig()
	if metricsCfg.Namespace != "" {
		mCfg.Namespace = metricsCfg.Namespace
	}
	if metricsCfg.Subsystem != "" {
		mCfg.Subsystem = metricsCfg.Subsystem
	}
	if len(metricsCfg.Buckets) > 0 {
		mCfg.Buckets = metricsCfg.Buckets
	}

	var metrics *middleware.Metrics
	if metricsCfg.Enabled {
		metrics = middleware.NewMetrics(mCfg)
	}

	rateLimitConfig, rateLimitEnabled := resolveRateLimitConfig()

	r.Use(
		chiMiddleware.RequestID,
		chiMiddleware.RealIP,
	)

	if metricsCfg.Enabled {
		r.Use(metrics.Middleware(mCfg))
	}

	middlewares := []func(http.Handler) http.Handler{
		middleware.SecurityHeaders,
		middleware.CORS(corsConfigFromOptions(options)),
		middleware.BodyLimit(middleware.BodyLimitConfig{
			MaxBytes: 10 * 1024 * 1024, // 10MB
		}),
	}

	if rateLimitEnabled {
		middlewares = append(middlewares, middleware.RateLimit(rateLimitConfig))
	}

	middlewares = append(middlewares,
		middleware.StructuredLogger(middleware.LoggingConfig{
			Logger:        logger,
			SlowThreshold: 500 * time.Millisecond,
			SkipPaths:     []string{"/health", "/healthz", "/_internal/ready", "/metrics"},
		}),
		chiMiddleware.Recoverer,
		chiMiddleware.Compress(5),
		middleware.I18n(services.I18n),
		middleware.InstallGuard(logger, services.Install),
	)

	r.Use(middlewares...)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"ts":     time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	// Alias for Docker health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"ts":     time.Now().UTC().Format(time.RFC3339Nano),
		})
	})

	r.Get("/_internal/ready", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	// Prometheus metrics endpoint
	if metricsCfg.Enabled {
		// If token is set, guard the metrics endpoint
		if metricsCfg.Token != "" {
			r.With(middleware.MetricsGuard(metricsCfg.Token)).Handle("/metrics", promhttp.Handler())
		} else {
			r.Handle("/metrics", promhttp.Handler())
		}
	}

	registerAPIRoutes(r, services)

	// Short link redirect route (public, no auth required)
	if services.ShortLink != nil {
		shortLinkHandler := handler.NewShortLinkHandler(services.ShortLink, services.Subscription, services.I18n)
		r.Get("/s/{code}", shortLinkHandler.HandleRedirect)
	}

	// 使用统一的 SPA 路由处理器，避免 chi 动态路由参数 /{securePath}
	// 错误匹配 User SPA 的静态资源请求（如 /assets/*）
	var adminHandler *adminSPAHandler
	var userHandler *userSPAHandler

	if options.adminUI.Enabled {
		handler, err := newAdminSPAHandler(logger, services.AdminPath, options.adminUI)
		if err != nil {
			logger.Error("admin ui disabled", "error", err)
		} else {
			adminHandler = handler
		}
	}

	if options.userUI.Enabled {
		handler, err := newUserSPAHandler(logger, options.userUI)
		if err != nil {
			logger.Error("user ui disabled", "error", err)
		} else {
			userHandler = handler
		}
	}

	// 只有当至少有一个 SPA handler 可用时才注册路由
	if adminHandler != nil || userHandler != nil {
		registerSPARoutes(r, userHandler, adminHandler)
	}

	registerInstallRoutes(r, logger, services.Install, options, userHandler)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		logger.Warn("unmapped route hit", "method", req.Method, "path", req.URL.Path)
		http.NotFound(w, req)
	})

	return r
}

func registerInstallRoutes(root chi.Router, logger *slog.Logger, install service.InstallService, options routerOptions, userHandler *userSPAHandler) {
	installHandler := handler.NewInstallHandler(install)
	root.Route("/api/install", func(api chi.Router) {
		api.Get("/status", installHandler.Status)
		api.Post("/", installHandler.Create)
	})
	if userHandler != nil {
		return
	}
	if !options.installUI.Enabled {
		return
	}
	page, err := newInstallPageHandler(options.installUI.Dir)
	if err != nil {
		if logger != nil {
			logger.Error("install ui disabled", "error", err)
		}
		return
	}
	root.Get("/install", page.serveIndex)
	root.Handle("/install/*", http.HandlerFunc(page.serveAssets))
}

func registerAPIRoutes(root chi.Router, services Services) {
	root.Route("/api", func(api chi.Router) {
		// V1/V2 是历史遗留的版本号，两个同时保留，确保旧客户端还能访问。
		registerV2Routes(api, services)
		registerV1Routes(api, services)
	})
}

func registerV2Routes(api chi.Router, services Services) {
	api.Route("/v2", func(v2 chi.Router) {
		registerV2AdminRoutes(v2, services.Config, services.Auth, services.AdminPath, services.Plan, services.AdminPlan, services.AdminUser, services.AdminServer, services.AdminStat, services.AdminNodeStat, services.AdminSystem, services.AdminSystemSettings, services.AdminNotice, services.AdminKnowledge, services.AgentHost, services.AgentCore, services.AgentLifecycleOperation, services.AgentTrafficLifecycle, services.BinaryVersion, services.Forwarding, services.CDN, services.AccessLog, services.InboundSpec, services.CoreConfigItem, services.DriftAndDiff, services.ApplyOrchestrator, services.OperationLog, services.SubscriptionFilter, services.SubscriptionSource, services.MCPApiKeys, services.Mesh, services.I18n)
		registerV2UserRoutes(v2, services.User, services.Auth, services.I18n)
		registerV2PassportRoutes(v2, services.Auth, services.Verify, services.Password, services.Register, services.MailLink, services.Comm, services.I18n)
		registerV2ServerRoutes(v2, services.ServerAuth, services.ServerNode, services.Telemetry, services.Traffic, services.TrafficQueue, services.I18n)
		registerV2GuestRoutes(v2, services.I18n)
	})
}

func registerV2GuestRoutes(v2 chi.Router, i18nManager *i18n.Manager) {
	// Guest routes (including i18n) don't need auth
	guestHandler := handler.NewGuestHandler(nil, i18nManager)

	v2.Route("/guest", func(guest chi.Router) {
		mountHandler(guest, "/i18n/{lang}", http.HandlerFunc(guestHandler.HandleI18n))
	})
}

func registerV2AdminRoutes(v2 chi.Router, configService service.ConfigService, auth service.AuthService, adminPath service.AdminPathService, plan service.PlanService, adminPlan service.AdminPlanService, adminUser service.AdminUserService, adminServer service.AdminServerService, adminStat service.AdminStatService, adminNodeStat service.AdminNodeStatService, adminSystem service.AdminSystemService, adminSystemSettings service.AdminSystemSettingsService, adminNotice service.AdminNoticeService, adminKnowledge service.AdminKnowledgeService, agentHost service.AgentHostService, agentCore service.AgentCoreService, agentLifecycleOperation service.AgentLifecycleOperationService, agentTrafficLifecycle service.AgentTrafficLifecycleService, binaryVersion service.BinaryVersionService, forwarding service.ForwardingService, cdn service.CDNService, accessLog service.AccessLogService, inboundSpec service.InboundSpecService, coreConfigItem service.CoreConfigItemService, driftAndDiff service.DriftAndDiffService, applyOrchestrator service.ApplyOrchestratorService, operationLog service.OperationLogService, subscriptionFilter service.SubscriptionFilterService, subscriptionSource service.SubscriptionSourceService, mcpApiKeys service.MCPApiKeyService, meshService service.AgentMeshService, i18nManager *i18n.Manager) {
	adminHandler := handler.NewAdminHandler(configService)
	adminPlanHandler := handler.NewAdminPlanHandler(plan, adminPlan, i18nManager)
	adminUserHandler := handler.NewAdminUserHandler(adminUser)
	adminServerHandler := handler.NewAdminServerHandler(adminServer)
	adminStatHandler := handler.NewAdminStatHandler(adminStat, i18nManager)
	adminNodeStatHandler := handler.NewAdminNodeStatHandler(adminNodeStat, i18nManager)
	adminSystemHandler := handler.NewAdminSystemSettingsHandler(adminSystem, adminSystemSettings)
	adminNoticeHandler := handler.NewAdminNoticeHandler(adminNotice)
	adminKnowledgeHandler := handler.NewAdminKnowledgeHandler(adminKnowledge, i18nManager)
	agentHostHandler := handler.NewAgentHostHandler(agentHost, i18nManager)
	adminForwardingHandler := handler.NewAdminForwardingHandler(forwarding, i18nManager)
	adminCDNHandler := handler.NewAdminCDNHandler(cdn, i18nManager)
	cdnProbeHandler := handler.NewCDNProbeHandler(cdn)
	var cdnReportHandler *handler.AdminCDNReportHandler
	if cdn != nil {
		cdnReportHandler = handler.NewAdminCDNReportHandler(cdn, i18nManager)
	}
		adminMCPKeyHandler := handler.NewAdminMCPKeyHandler(mcpApiKeys)
	adminMeshHandler := handler.NewAdminAgentMeshHandler(meshService, i18nManager)
	adminAgentCoreHandler := handler.NewAdminAgentCoreHandler(agentCore, i18nManager)
	adminAgentConfigHandler := handler.NewAdminAgentConfigHandler(agentHost, i18nManager)
	adminAgentLifecycleHandler := handler.NewAdminAgentLifecycleHandler(agentLifecycleOperation, binaryVersion, i18nManager)
	adminAgentTrafficHandler := handler.NewAdminAgentTrafficHandler(agentTrafficLifecycle, i18nManager)
	adminAgentVersionHandler := handler.NewAdminAgentVersionHandler(binaryVersion, i18nManager)
	adminSubscriptionHandler := handler.NewAdminSubscriptionHandler(subscriptionFilter, subscriptionSource, i18nManager)
	adminAccessLogHandler := handler.NewAdminAccessLogHandler(accessLog)
	adminConfigCenterSpecHandler := handler.NewAdminConfigCenterSpecHandler(inboundSpec, i18nManager)
	adminConfigCenterCoreConfigHandler := handler.NewAdminConfigCenterCoreConfigHandler(coreConfigItem)
	adminConfigCenterDiffHandler := handler.NewAdminConfigCenterDiffHandler(driftAndDiff, i18nManager)
	adminConfigCenterDriftHandler := handler.NewAdminConfigCenterDriftHandler(driftAndDiff, i18nManager)
	adminConfigCenterApplyHandler := handler.NewAdminConfigCenterApplyHandler(applyOrchestrator, i18nManager)
	operationLogHandler := handler.NewOperationLogHandler(operationLog, i18nManager)

	v2.Route("/{securePath}", func(admin chi.Router) {
		admin.Use(middleware.AdminGuard(auth, adminPath))
		mountHandler(admin, "/config", adminHandler)
		mountHandler(admin, "/plan", adminPlanHandler)
		// Plan RESTful endpoints
		admin.Get("/plan", adminPlanHandler.List)
		admin.Post("/plan", adminPlanHandler.Create)
		admin.Get("/plan/{id:[0-9]+}", adminPlanHandler.Get)
		admin.Put("/plan/{id:[0-9]+}", adminPlanHandler.Update)
		admin.Delete("/plan/{id:[0-9]+}", adminPlanHandler.Delete)
		mountHandler(admin, "/server/group", adminServerHandler)
		mountHandler(admin, "/server/route", adminServerHandler)
		mountHandler(admin, "/server/manage", adminServerHandler)
		mountHandler(admin, "/user", adminUserHandler)
		// User RESTful endpoints
		admin.Post("/user", adminUserHandler.Create)
		admin.Get("/user", adminUserHandler.List)
		admin.Get("/user/{id:[0-9]+}", adminUserHandler.Get)
		admin.Put("/user/{id:[0-9]+}", adminUserHandler.Update)
		admin.Delete("/user/{id:[0-9]+}", adminUserHandler.Delete)
		mountHandler(admin, "/stat", adminStatHandler)
		// Node statistics endpoints
		admin.Get("/nodes/stat/fetch", adminNodeStatHandler.GetServerStats)
		admin.Get("/nodes/stat/traffic", adminNodeStatHandler.GetTotalTraffic)
		admin.Get("/nodes/stat/rank", adminNodeStatHandler.GetTopServers)
		mountHandler(admin, "/system", adminSystemHandler)
		// System RESTful endpoints
		admin.Get("/system/status", adminSystemHandler.Status)
		mountHandler(admin, "/notice", adminNoticeHandler)
		// Notice RESTful endpoints
		admin.Get("/notice", adminNoticeHandler.List)
		admin.Post("/notice", adminNoticeHandler.Create)
		admin.Get("/notice/{id:[0-9]+}", adminNoticeHandler.Get)
		admin.Put("/notice/{id:[0-9]+}", adminNoticeHandler.Update)
		admin.Delete("/notice/{id:[0-9]+}", adminNoticeHandler.Delete)
		mountHandler(admin, "/knowledge", adminKnowledgeHandler)

		// Agent Host management endpoints
		admin.Get("/agent-hosts", agentHostHandler.List)
		admin.Post("/agent-hosts", agentHostHandler.Create)
		admin.Post("/agent-hosts/refresh", agentHostHandler.RefreshAll) // Must be before {id} routes
		admin.Get("/agent-hosts/{id}", agentHostHandler.Get)
		admin.Put("/agent-hosts/{id}", agentHostHandler.Update)
		admin.Delete("/agent-hosts/{id}", agentHostHandler.Delete)
		admin.Post("/agent-hosts/{id}/refresh", agentHostHandler.Refresh)

		// Agent core management endpoints
		admin.Get("/agent-hosts/{id}/cores", adminAgentCoreHandler.ListCores)
		admin.Get("/agent-hosts/{id}/core-instances", adminAgentCoreHandler.ListInstances)
		admin.Get("/agent-hosts/{id}/core-operations", adminAgentCoreHandler.ListOperations)
		admin.Post("/agent-hosts/{id}/core-instances", adminAgentCoreHandler.CreateInstance)
		admin.Delete("/agent-hosts/{id}/core-instances/{instance_id}", adminAgentCoreHandler.DeleteInstance)
		admin.Post("/agent-hosts/{id}/core-switch", adminAgentCoreHandler.SwitchCore)
		admin.Post("/agent-hosts/{id}/core-install", adminAgentCoreHandler.InstallCore)
		admin.Post("/agent-hosts/{id}/core-convert", adminAgentCoreHandler.ConvertConfig)
		admin.Get("/agent-hosts/{id}/core-switch-logs", adminAgentCoreHandler.ListSwitchLogs)
		admin.Get("/agent-hosts/{id}/versions", adminAgentVersionHandler.ListVersions)
		admin.Post("/agent-hosts/{id}/versions/{component}/refresh", adminAgentVersionHandler.RefreshVersion)
		admin.Get("/agent-hosts/{id}/lifecycle-operations", adminAgentLifecycleHandler.ListOperations)
		admin.Post("/agent-hosts/{id}/update-check", adminAgentLifecycleHandler.CreateUpdateCheck)
		admin.Post("/agent-hosts/{id}/update", adminAgentLifecycleHandler.CreateUpdate)
		admin.Post("/agent-hosts/{id}/traffic-reset", adminAgentLifecycleHandler.CreateTrafficReset)
		admin.Get("/agent-hosts/{id}/traffic-policy", adminAgentTrafficHandler.GetPolicy)
		admin.Put("/agent-hosts/{id}/traffic-policy", adminAgentTrafficHandler.UpdatePolicy)
		admin.Get("/agent-hosts/{id}/traffic-status", adminAgentTrafficHandler.GetStatus)
		admin.Post("/agent-hosts/{id}/traffic-cycle/reset", adminAgentTrafficHandler.ResetCycle)

		// Agent config endpoints
		admin.Get("/agent-hosts/{id}/config", adminAgentConfigHandler.GetConfig)
		admin.Post("/agent-hosts/{id}/report-config", adminAgentConfigHandler.ReportConfig)

		// Subscription source and filter observability endpoints
		admin.Get("/subscription/sources", adminSubscriptionHandler.ListSources)
		admin.Post("/subscription/sources", adminSubscriptionHandler.CreateSource)
		admin.Get("/subscription/sources/{id:[0-9]+}", adminSubscriptionHandler.GetSource)
		admin.Put("/subscription/sources/{id:[0-9]+}", adminSubscriptionHandler.UpdateSource)
		admin.Delete("/subscription/sources/{id:[0-9]+}", adminSubscriptionHandler.DeleteSource)
		admin.Post("/subscription/sources/{id:[0-9]+}/sync", adminSubscriptionHandler.SyncSource)
		admin.Get("/subscription/filter-reasons", adminSubscriptionHandler.ListFilterReasons)
		admin.Get("/subscription/filter-summary", adminSubscriptionHandler.GetFilterSummary)

		// CDN site management endpoints
		admin.Route("/cdn", func(cdn chi.Router) {
			cdn.Get("/sites", adminCDNHandler.ListSites)
			cdn.Post("/sites", adminCDNHandler.CreateSite)
			cdn.Put("/sites/{id}", adminCDNHandler.UpdateSite)
			cdn.Delete("/sites/{id}", adminCDNHandler.DeleteSite)
			cdn.Get("/sites/{id}/edges", adminCDNHandler.ListEdges)
			cdn.Post("/sites/{id}/edges", adminCDNHandler.AssignEdge)
			cdn.Delete("/sites/{id}/edges/{edge_id}", adminCDNHandler.RemoveEdge)
			cdn.Get("/sites/{id}/rules", adminCDNHandler.ListCacheRules)
			cdn.Post("/sites/{id}/rules", adminCDNHandler.CreateCacheRule)
			cdn.Delete("/sites/{id}/rules/{rule_id}", adminCDNHandler.DeleteCacheRule)
			cdn.Post("/sites/{id}/deploy", adminCDNHandler.DeploySite)
			cdn.Post("/sites/{id}/undeploy", adminCDNHandler.UndeploySite)
			cdn.Post("/sites/{id}/sync", adminCDNHandler.SyncToProvider)
			cdn.Post("/sites/{id}/invalidate", adminCDNHandler.InvalidateCache)
			cdn.Get("/sites/{id}/provider-status", adminCDNHandler.GetProviderStatus)

			// Cloudflare integration
			cdn.Post("/cloudflare/token", adminCDNHandler.SetCloudflareAPIToken)
			cdn.Get("/cloudflare/zones", adminCDNHandler.ListCloudflareZones)
			cdn.Post("/cloudflare/zones", adminCDNHandler.AddCloudflareZone)
			cdn.Delete("/cloudflare/zones/{zone_id}", adminCDNHandler.RemoveCloudflareZone)
			cdn.Get("/cloudflare/zones/{zone_id}/dns", adminCDNHandler.ListDNSRecords)

			// CloudFront integration
			cdn.Post("/cloudfront/credentials", adminCDNHandler.SetCloudFrontCredentials)
			cdn.Get("/cloudfront/credentials", adminCDNHandler.GetCloudFrontCredentials)
			cdn.Get("/cloudfront/distributions", adminCDNHandler.ListCloudFrontDistributions)

			// Origin latency monitoring
			if cdnReportHandler != nil {
				cdn.Get("/origin-latency", cdnReportHandler.ListOriginLatencies)
				cdn.Post("/origin-latency", cdnReportHandler.ReportOriginLatency)
			}

			// Latency probe report (for CDN edge probe dashboard)
			cdn.Get("/probe-report", cdnProbeHandler.ProberResults)
		})

			// Mesh network management endpoints
			admin.Route("/agent-hosts/{id}/mesh", func(mesh chi.Router) {
				mesh.Post("/join", adminMeshHandler.Join)
				mesh.Delete("/leave", adminMeshHandler.Leave)
				mesh.Get("/status", adminMeshHandler.GetStatus)
			})
			admin.Get("/mesh/network/{networkID}", adminMeshHandler.ListNetwork)

		// Forwarding rules management endpoints
		admin.Route("/forwarding", func(fwd chi.Router) {
			fwd.Get("/rules", adminForwardingHandler.ListRules)
			fwd.Post("/rules", adminForwardingHandler.CreateRule)
			fwd.Put("/rules/{id}", adminForwardingHandler.UpdateRule)
			fwd.Delete("/rules/{id}", adminForwardingHandler.DeleteRule)
			fwd.Get("/logs", adminForwardingHandler.ListLogs)
		})

		// Access logs endpoints
		admin.Route("/access-logs", func(logs chi.Router) {
			logs.Get("/fetch", adminAccessLogHandler.Fetch)
			logs.Get("/stats", adminAccessLogHandler.GetStats)
			logs.Post("/cleanup", adminAccessLogHandler.Cleanup)
		})

		// Config center spec endpoints
		admin.Route("/config-center/specs", func(specs chi.Router) {
			specs.Get("/", adminConfigCenterSpecHandler.ListSpecs)
			specs.Post("/", adminConfigCenterSpecHandler.Create)
			specs.Put("/{id:[0-9]+}", adminConfigCenterSpecHandler.Update)
			specs.Delete("/{id:[0-9]+}", adminConfigCenterSpecHandler.Delete)
			specs.Get("/{id:[0-9]+}/history", adminConfigCenterSpecHandler.GetHistory)
			specs.Post("/import-from-applied", adminConfigCenterSpecHandler.ImportFromApplied)
			specs.Post("/{id:[0-9]+}/bind", adminConfigCenterSpecHandler.BindSpec)
			specs.Delete("/{id:[0-9]+}/bind", adminConfigCenterSpecHandler.UnbindSpec)
			specs.Get("/{id:[0-9]+}/bind", adminConfigCenterSpecHandler.ListBoundHosts)
		})

			// Config center core config endpoints
			admin.Route("/config-center/core-configs", func(coreConf chi.Router) {
				coreConf.Get("/", adminConfigCenterCoreConfigHandler.List)
				coreConf.Post("/", adminConfigCenterCoreConfigHandler.Create)
				coreConf.Put("/{id:[0-9]+}", adminConfigCenterCoreConfigHandler.Update)
				coreConf.Delete("/{id:[0-9]+}", adminConfigCenterCoreConfigHandler.Delete)
			})

		// Config center artifact/diff endpoints
		admin.Get("/config-center/artifacts", adminConfigCenterDiffHandler.ListArtifacts)
		admin.Get("/config-center/diff/text", adminConfigCenterDiffHandler.GetTextDiff)
		admin.Get("/config-center/diff/semantic", adminConfigCenterDiffHandler.GetSemanticDiff)

		// Config center drift observability endpoints
		admin.Get("/config-center/snapshot", adminConfigCenterDriftHandler.ListAppliedSnapshot)
		admin.Get("/config-center/drift", adminConfigCenterDriftHandler.ListDriftStates)
		admin.Get("/config-center/recover", adminConfigCenterDriftHandler.ListRecoveryStates)
		admin.Post("/config-center/recover", adminConfigCenterDriftHandler.ListRecoveryStates)

		// Config center apply run endpoints
		admin.Post("/config-center/apply-runs", adminConfigCenterApplyHandler.CreateApplyRun)
		admin.Get("/config-center/apply-runs", adminConfigCenterApplyHandler.ListApplyRuns)
		admin.Get("/config-center/apply-runs/{run_id}", adminConfigCenterApplyHandler.GetApplyRunDetail)
		admin.Post("/config-center/apply-runs/{run_id}/cancel", adminConfigCenterApplyHandler.CancelApplyRun)

			// MCP API key management endpoints
			admin.Get("/mcp/keys", adminMCPKeyHandler.List)
			admin.Post("/mcp/keys", adminMCPKeyHandler.Create)
			admin.Post("/mcp/keys/{id}/revoke", adminMCPKeyHandler.Revoke)
			admin.Delete("/mcp/keys/{id}", adminMCPKeyHandler.Delete)

		// Operation log endpoints
		admin.Get("/operation-logs", operationLogHandler.List)
		admin.Get("/operation-logs/stream", operationLogHandler.Stream)

		// 已移除的商业化/占位模块不再挂载，避免 404/501 噪声。
		// mountHandler(admin, "/coupon", adminHandler)
	})
}

func registerV2UserRoutes(v2 chi.Router, userService service.UserService, auth service.AuthService, i18nManager *i18n.Manager) {
	userHandler := handler.NewUserHandler(userService, i18nManager)
	v2.Route("/user", func(user chi.Router) {
		user.Use(middleware.UserGuard(auth))
		mountHandler(user, "/", userHandler)
	})
}

func registerV2PassportRoutes(v2 chi.Router, auth service.AuthService, verify service.VerificationService, password service.PasswordService, register service.RegistrationService, mailLink service.MailLinkService, comm service.CommService, i18nMgr *i18n.Manager) {
	passportHandler := handler.NewPassportHandler(auth, verify, password, register, mailLink, comm, i18nMgr)
	v2.Route("/passport", func(passport chi.Router) {
		mountHandler(passport, "/auth", passportHandler)
		mountHandler(passport, "/comm", passportHandler)
	})
}

func registerV2ServerRoutes(v2 chi.Router, serverAuth service.ServerAuthService, nodes service.ServerNodeService, telemetry service.ServerTelemetryService, traffic service.ServerTrafficService, queue *async.TrafficQueue, i18nManager *i18n.Manager) {
	serverHandler := handler.NewServerHandler(nodes, telemetry, traffic, queue, i18nManager)
	v2.Route("/server", func(server chi.Router) {
		server.Use(middleware.ServerGuard(serverAuth, ""))
		mountHandler(server, "/config", serverHandler)
		mountHandler(server, "/user", serverHandler)
		mountHandler(server, "/push", serverHandler)
		mountHandler(server, "/alive", serverHandler)
		mountHandler(server, "/alivelist", serverHandler)
		mountHandler(server, "/status", serverHandler)
	})
}

func registerV1Routes(api chi.Router, services Services) {
	api.Route("/v1", func(v1 chi.Router) {
		registerV1ClientRoutes(v1, services.User, services.Auth, services.Subscription, services.I18n)
		registerV1GuestRoutes(v1, services.Comm, services.Plan, services.I18n)
		registerV1PassportRoutes(v1, services.Auth, services.Verify, services.Password, services.Register, services.MailLink, services.Comm, services.I18n)
		registerV1UserRoutes(v1, services.User, services.UserKnowledge, services.UserNotice, services.UserStat, services.Auth, services.Plan, services.Server, services.UserSelection, services.ShortLink, services.Subscription, services.I18n)
		registerV1AgentRoutes(v1, services.AgentHost, services.I18n)
	})
}

func registerV1ClientRoutes(v1 chi.Router, userService service.UserService, auth service.AuthService, subscription service.SubscriptionService, i18nManager *i18n.Manager) {
	userHandler := handler.NewUserHandler(userService, i18nManager)
	clientHandler := handler.NewClientHandler(subscription, i18nManager)
	v1.Route("/client", func(client chi.Router) {
		// subscribe endpoint uses token query param for auth, not JWT
		mountHandler(client, "/subscribe", clientHandler)

		// other endpoints require JWT auth
		client.Group(func(protected chi.Router) {
			protected.Use(middleware.UserGuard(auth))
			mountHandler(protected, "/", userHandler)
			mountHandler(protected, "/app", userHandler)
		})
	})
}

func registerV1GuestRoutes(v1 chi.Router, comm service.CommService, plan service.PlanService, i18nManager *i18n.Manager) {
	guestHandler := handler.NewGuestHandler(comm, i18nManager)
	guestPlanHandler := handler.NewGuestPlanHandler(plan, i18nManager)
	v1.Route("/guest", func(guest chi.Router) {
		mountHandler(guest, "/plan", guestPlanHandler)
		mountHandler(guest, "/telegram", guestHandler)
		mountHandler(guest, "/comm", guestHandler)
	})
}

func registerV1PassportRoutes(v1 chi.Router, auth service.AuthService, verify service.VerificationService, password service.PasswordService, register service.RegistrationService, mailLink service.MailLinkService, comm service.CommService, i18nMgr *i18n.Manager) {
	passportHandler := handler.NewPassportHandler(auth, verify, password, register, mailLink, comm, i18nMgr)
	v1.Route("/passport", func(passport chi.Router) {
		mountHandler(passport, "/auth", passportHandler)
		mountHandler(passport, "/comm", passportHandler)
	})
}

func registerV1UserRoutes(v1 chi.Router, userService service.UserService, knowledgeService service.UserKnowledgeService, noticeService service.UserNoticeService, statService service.UserStatService, auth service.AuthService, planService service.PlanService, serverService service.ServerService, selectionService service.UserServerSelectionService, shortLinkService service.ShortLinkService, subscriptionService service.SubscriptionService, i18nManager *i18n.Manager) {
	userHandler := handler.NewUserHandler(userService, i18nManager)
	planHandler := handler.NewUserPlanHandler(planService, i18nManager)
	userServerHandler := handler.NewUserServerHandler(serverService, selectionService, i18nManager)
	userKnowledgeHandler := handler.NewUserKnowledgeHandler(knowledgeService, i18nManager)
	userNoticeHandler := handler.NewUserNoticeHandler(noticeService, i18nManager)
	userStatHandler := handler.NewUserStatHandler(statService, i18nManager)
	shortLinkHandler := handler.NewShortLinkHandler(shortLinkService, subscriptionService, i18nManager)
	v1.Route("/user", func(user chi.Router) {
		user.Use(middleware.UserGuard(auth))
		// 这里的 mountHandler 会同时绑定 /path 和 /path/*，避免重复写路由。
		mountHandler(user, "/", userHandler)
		mountHandler(user, "/notice", userNoticeHandler)
			// Explicitly register /notice/unread to avoid chi wildcard matching edge cases
			user.Get("/notice/unread", userNoticeHandler.ServeHTTP)
		mountHandler(user, "/server", userServerHandler)
		mountHandler(user, "/telegram", userHandler)
		mountHandler(user, "/comm", userHandler)
		mountHandler(user, "/knowledge", userKnowledgeHandler)
		mountHandler(user, "/plan", planHandler)
		mountHandler(user, "/stat", userStatHandler)
		mountHandler(user, "/shortlink", shortLinkHandler)
	})
}

func mountHandler(r chi.Router, path string, h http.Handler) {
	normalized := path
	if normalized == "" {
		normalized = "/"
	}
	r.Handle(normalized, h)

	if normalized != "/" && !strings.HasSuffix(normalized, "/*") {
		wildcard := normalized
		if strings.HasSuffix(wildcard, "/") {
			wildcard += "*"
		} else {
			wildcard += "/*"
		}
		r.Handle(wildcard, h)
	} else {
		r.Handle("/*", h)
	}
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// registerV1AgentRoutes registers agent-related API endpoints.
// These endpoints are called by agents deployed on edge nodes.
func registerV1AgentRoutes(v1 chi.Router, agentHost service.AgentHostService, i18nManager *i18n.Manager) {
	if agentHost == nil {
		return // Agent host service not configured
	}
	agentHostHandler := handler.NewAgentHostHandler(agentHost, i18nManager)
	v1.Route("/agent", func(agent chi.Router) {
		// Status reporting from agent (no auth middleware, token in query param)
		agent.Post("/register", agentHostHandler.Register)
		agent.Post("/status", agentHostHandler.ReportStatus)
		agent.Post("/heartbeat", agentHostHandler.Heartbeat)
	})
}
