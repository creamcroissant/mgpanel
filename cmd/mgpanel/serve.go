package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creamcroissant/mgpanel/internal/api"
	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/bootstrap"
	"github.com/creamcroissant/mgpanel/internal/config"
	internalgrpc "github.com/creamcroissant/mgpanel/internal/grpc"
	"github.com/creamcroissant/mgpanel/internal/grpc/handler"
	"github.com/creamcroissant/mgpanel/internal/grpc/interceptor"
	"github.com/creamcroissant/mgpanel/internal/job"
	"github.com/creamcroissant/mgpanel/internal/mcp"
	"github.com/creamcroissant/mgpanel/internal/mcp/tools"
	"github.com/creamcroissant/mgpanel/internal/migrations"
	"github.com/creamcroissant/mgpanel/internal/protocol"
	"github.com/creamcroissant/mgpanel/internal/repository/sqlite"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/creamcroissant/mgpanel/internal/support/logging"
	"github.com/creamcroissant/mgpanel/internal/template"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MGPanel server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

// meshRoutingJob implements job.Runnable for periodic mesh routing table computation.
type meshRoutingJob struct {
	meshService service.AgentMeshService
	logger      *slog.Logger
}

func (j *meshRoutingJob) Name() string { return "mesh_routing_compute" }

func (j *meshRoutingJob) Run(ctx context.Context) error {
	if err := j.meshService.ComputeRoutingTables(ctx); err != nil {
		return err
	}
	// Push computed routes as agent commands
	if pusher, ok := j.meshService.(interface{ PushRoutingTables(context.Context) error }); ok {
		if err := pusher.PushRoutingTables(ctx); err != nil {
			j.logger.Warn("failed to push routing tables", "error", err)
		}
	}
	return nil
}

func registerSchedulerJob(scheduler *job.Scheduler, field, spec string, runnable job.Runnable) error {
	if _, err := scheduler.Register(spec, runnable); err != nil {
		return fmt.Errorf("register scheduler %s: %w", field, err)
	}
	return nil
}

func runServe(cmd *cobra.Command, args []string) error {
	bootTime := time.Now().UTC()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadWithOptions(config.LoadOptions{ConfigPath: configPath})
	if err != nil {
		return err
	}

	service.SetAgentTaskTimeouts(
		cfg.AgentTask.CoreOperationClaimTimeout,
		cfg.AgentTask.LifecycleOperationClaimTimeout,
		cfg.AgentTask.ApplyRunClaimTimeout,
	)

	runtimeVersion := resolveRuntimeVersion()

	resolvedDBPath, err := bootstrap.ResolveSQLitePath(cfg.DB.Path)
	if err != nil {
		return err
	}
	cfg.DB.Path = resolvedDBPath

	logger := logging.New(logging.Options{
		Level:     cfg.Log.SlogLevel(),
		Format:    cfg.Log.Format,
		AddSource: cfg.Log.AddSource,
		LogDir:    cfg.Log.LogDir,
		MaxDays:   cfg.Log.MaxDays,
	})
	logger.Info("database path resolved", "path", cfg.DB.Path)

	db, err := bootstrap.OpenSQLite(cfg.DB.Path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrations.Up(db); err != nil {
		return err
	}

	resolvedSigningKey, signingKeySource, err := bootstrap.ResolveJWTSigningKey(ctx, db, cfg.Auth.SigningKey, time.Now)
	if err != nil {
		return err
	}
	cfg.Auth.SigningKey = resolvedSigningKey

	switch signingKeySource {
	case bootstrap.JWTSigningKeySourceConfig:
		logger.Info("jwt signing key loaded", "source", "config")
	case bootstrap.JWTSigningKeySourceSettings:
		logger.Info("jwt signing key loaded", "source", "settings")
	case bootstrap.JWTSigningKeySourceGenerated:
		logger.Info("jwt signing key generated", "source", "generated-and-persisted")
	default:
		logger.Info("jwt signing key loaded", "source", "unknown")
	}

	signingKey := cfg.Auth.SigningKey // 显式读取（已在 L89 被 ResolveJWTSigningKey 变异）
	legacyCfg := buildBootstrapConfig(cfg, runtimeVersion, signingKey)

	infra, err := bootstrap.BuildInfrastructure(legacyCfg, logger)
	if err != nil {
		return err
	}

	store := sqlite.NewStore(db)

	// Services initialization
	captchaService := service.NewCaptchaService(store.Settings(), nil)
	notificationQueue := async.NewNotificationQueue()
	queuedNotifier := async.NewQueueNotifier(notificationQueue)
	verifyService := service.NewVerificationService(infra.Cache, queuedNotifier, store.Settings(), store.Users(), captchaService)
	passwordService := service.NewPasswordService(store.Users(), infra.Hasher, verifyService, infra.Cache)
	registrationService := service.NewRegistrationService(store.Users(), store.Settings(), infra.Hasher, verifyService, infra.Cache)
	mailLinkService := service.NewMailLinkService(store.Users(), store.Settings(), queuedNotifier, infra.Cache)
	commService := service.NewCommService(store.Settings(), store.Plugins())
	planService := service.NewPlanService(store.Plans(), store.Users(), store.Settings(), store.ServerGroups(), logger)
	i18nManager, err := i18n.NewManager(
		i18n.WithLogger(logger),
		i18n.WithDefaultLang("en-US"),
	)
	if err != nil {
		return err
	}

	adminPlanService := service.NewAdminPlanService(store.Plans(), i18nManager)
	serverTelemetryService := service.NewServerTelemetryServiceWithLogger(infra.Cache, store.Settings(), store.Servers(), store.StatServers(), logger)
	adminUserService := service.NewAdminUserService(
		store.Users(),
		store.Plans(),
		store.ServerGroups(),
		store.Settings(),
		serverTelemetryService,
		infra.Hasher,
		i18nManager,
	)
	adminServerService := service.NewAdminServerService(store.ServerGroups(), store.ServerRoutes(), store.Servers(), i18nManager)
	adminStatService := service.NewAdminStatService(store.StatUsers(), store.Users())
	adminNodeStatService := service.NewAdminNodeStatService(store.StatServers())
	adminNoticeService := service.NewAdminNoticeService(store.Notices(), i18nManager)
	adminKnowledgeService := service.NewAdminKnowledgeService(store.Knowledge(), i18nManager)
	userKnowledgeService := service.NewUserKnowledgeService(store.Knowledge(), store.Users(), store.Settings())
	userNoticeService := service.NewUserNoticeService(store.Notices(), store.UserNoticeReads())
	userStatService := service.NewUserStatService(store.StatUsers())
	protocolManager := protocol.NewManager(
		protocol.NewGeneralBuilder(),
		protocol.NewClashBuilder(),
		protocol.NewSurgeBuilder(),
		protocol.NewSingboxBuilder(),
	)
	serverAuthService := service.NewServerAuthService(store.Settings(), store.Servers())
	serverNodeService := service.NewServerNodeService(store.Users(), store.ServerRoutes(), store.Settings())

	// Multi-accumulator for multi-granularity statistics (hourly, daily, monthly)
	multiAccumulator := job.NewMultiAccumulator(3) // 0=hourly, 1=daily, 2=monthly
	serverTrafficService := service.NewServerTrafficService(store.Users(), multiAccumulator)
	userTrafficService := service.NewUserTrafficServiceWithCollector(store.UserTraffic(), store.Users(), store.Plans(), multiAccumulator, notificationQueue, store.Settings())
	userServerSelectionService := service.NewUserServerSelectionService(store.UserTraffic())
	trafficQueue := async.NewTrafficQueue()
	subLogQueue := async.NewSubscriptionLogQueue(store.SubscriptionLogs(), logger)
	installService := service.NewInstallService(store.Users(), infra.Hasher, i18nManager)

	adminSystemService := service.NewAdminSystemService(service.AdminSystemOptions{
		Version:           runtimeVersion,
		Environment:       cfg.Log.Environment,
		StartedAt:         bootTime,
		NotificationQueue: notificationQueue,
		TrafficQueue:      trafficQueue,
		Users:             store.Users(),
		Servers:           store.Servers(),
		AgentHosts:        store.AgentHosts(),
		I18n:              i18nManager,
	})
	adminSystemSettingsService := service.NewAdminSystemSettingsService(service.AdminSystemSettingsOptions{
		Settings:          store.Settings(),
		NotificationQueue: notificationQueue,
		Audit:             infra.Audit,
	})

	agentHostService := service.NewAgentHostServiceWithOptions(store.AgentHosts(), store.Servers(), store.ServerClientConfigs(), store.ConfigTemplates(), store.Users(), store.Settings(), service.AgentHostServiceOptions{Cache: infra.Cache, Logger: logger})
	agentService := service.NewAgentService(store.Servers(), store.Users())
	forwardingService := service.NewForwardingServiceWithLogger(store.ForwardingRules(), store.ForwardingRuleLogs(), store.AgentHosts(), logger)
	converterRegistry := template.NewConverterRegistry(&template.SingBoxConverter{}, &template.XrayConverter{})
	agentOperationGuard := service.NewAgentOperationGuard(store.CoreOperations(), store.ApplyRuns(), infra.Audit, store.AgentLifecycleOperations())
	agentCoreService := service.NewAgentCoreServiceWithOptions(
		store.AgentHosts(),
		store.AgentCoreInstances(),
		store.AgentCoreSwitchLogs(),
		store.ConfigTemplates(),
		converterRegistry,
		logger,
		service.AgentCoreServiceOptions{Operations: store.CoreOperations(), OperationGuard: agentOperationGuard, BinaryVersionStates: store.BinaryVersionStates()},
	)
	accessLogService := service.NewAccessLogService(store)
	artifactCompilerService := service.NewArtifactCompilerService(store.InboundSpecs(), store.DesiredArtifacts(), store.CoreConfigItems(), store.AgentMeshPeers(), store.ExitNodeSets(), store.RoutingPolicies())
	inboundSpecService := service.NewInboundSpecService(store.InboundSpecs(), store.InboundSpecRevisions(), store.InboundIndexes(), store.SpecHostBindings(), artifactCompilerService)
	coreConfigItemService := service.NewCoreConfigItemService(store.CoreConfigItems(), artifactCompilerService)
	driftAndDiffService := service.NewDriftAndDiffService(store.DesiredArtifacts(), store.AgentConfigInventories(), store.InboundIndexes(), store.DriftStates())
	inventoryIngestService := service.NewInventoryIngestService(store.AgentConfigInventories(), store.InboundIndexes())
	applyOrchestratorService := service.NewApplyOrchestratorServiceWithGuard(store.DesiredArtifacts(), store.ApplyRuns(), driftAndDiffService, agentOperationGuard)
	operationLogService := service.NewOperationLogService(store.OperationLogs(), logger)
	agentLifecycleOperationService := service.NewAgentLifecycleOperationService(store.AgentLifecycleOperations(), agentOperationGuard, operationLogService, infra.Audit)
	unlockProbeService := service.NewUnlockProbeService(store.UnlockProbeResults(), store.AgentHosts(), agentLifecycleOperationService)
	exitNodeSetService := service.NewExitNodeSetService(store.ExitNodeSets(), store.AgentHosts(), store.UnlockProbeResults(), logger)
	routingPolicyService := service.NewRoutingPolicyService(store.RoutingPolicies(), logger)

	cdnService := service.NewCDNService(
		store.CDNSites(), store.CDNEdges(), store.CDNCacheRules(),
		store.Settings(),
		store.CloudflareZones(), store.CloudflareDNSRecords(), store.CloudFrontDistributions(),
		cfg.Auth.SigningKey,
		store.AgentHosts(), agentLifecycleOperationService, store.CDNOriginLatencies(),
	)

	inboundSpecService.SetCDNService(cdnService)
	agentTrafficLifecycleService := service.NewAgentTrafficLifecycleService(store.AgentTrafficStates(), operationLogService, service.AgentTrafficLifecycleOptions{
		Policies:            store.AgentTrafficPolicies(),
		AgentHosts:          store.AgentHosts(),
		Servers:             store.Servers(),
		SubscriptionReasons: store.SubscriptionFilterReasons(),
		LifecycleOperations: agentLifecycleOperationService,
		Logger:              logger,
	})
	binaryVersionService := service.NewBinaryVersionServiceWithOptions(store.BinaryVersionStates(), store.AgentHosts(), buildBinaryVersionProvider(), service.BinaryVersionServiceOptions{CoreOperations: store.CoreOperations()})
	shortLinkService := service.NewShortLinkService(store.ShortLinks(), store.Users(), store.Settings())
	subscriptionSourceService := service.NewSubscriptionSourceService(store.SubscriptionSources(), service.SubscriptionSourceServiceOptions{})
	subscriptionFilterService := service.NewSubscriptionFilterService(store.Servers(), store.SubscriptionSources(), store.SubscriptionFilterReasons(), store.Plans(), userServerSelectionService, serverTelemetryService)
	coreOperationService := service.NewCoreOperationService(store.CoreOperations(), agentOperationGuard)
	coreSnapshotService := service.NewCoreSnapshotService(store.AgentHosts(), store.AgentCoreInstances())
	meshService := service.NewAgentMeshService(store.AgentMeshPeers(), store.AgentHosts(), agentLifecycleOperationService)

	scheduler := job.NewScheduler(logger)

	// Multi-granularity stat user jobs for traffic aggregation
	// Each job uses its own accumulator from the multi-accumulator
	// Hourly: runs every 5 minutes, aggregates to hourly buckets
	statUserJobHourly := job.NewStatUserJobWithType(multiAccumulator.Get(job.RecordTypeHourly), store.StatUsers(), logger, job.RecordTypeHourly)
	if err := registerSchedulerJob(scheduler, "scheduler.stat_user_hourly", cfg.Scheduler.StatUserHourly, statUserJobHourly); err != nil {
		return err
	}
	// Daily: runs every hour, aggregates to daily buckets
	statUserJobDaily := job.NewStatUserJobWithType(multiAccumulator.Get(job.RecordTypeDaily), store.StatUsers(), logger, job.RecordTypeDaily)
	if _, err := scheduler.Register("@every 1h", statUserJobDaily); err != nil {
		return err
	}
	// Monthly: runs at 00:05 every day, aggregates to monthly buckets
	statUserJobMonthly := job.NewStatUserJobWithType(multiAccumulator.Get(job.RecordTypeMonthly), store.StatUsers(), logger, job.RecordTypeMonthly)
	if _, err := scheduler.Register("5 0 * * *", statUserJobMonthly); err != nil {
		return err
	}
	trafficFetchJob := job.NewTrafficFetchJob(trafficQueue, serverTrafficService, logger)
	if err := registerSchedulerJob(scheduler, "scheduler.traffic_fetch", cfg.Scheduler.TrafficFetch, trafficFetchJob); err != nil {
		return err
	}
	emailJob := job.NewSendEmailJob(notificationQueue, infra.Notifier, logger)
	if err := registerSchedulerJob(scheduler, "scheduler.email_notify", cfg.Scheduler.EmailNotify, emailJob); err != nil {
		return err
	}
	telegramJob := job.NewSendTelegramJob(notificationQueue, infra.Notifier, logger)
	if err := registerSchedulerJob(scheduler, "scheduler.telegram_notify", cfg.Scheduler.TelegramNotify, telegramJob); err != nil {
		return err
	}
	heartbeatJob := job.NewNodeHeartbeatJob(store.Servers(), notificationQueue, store.Settings(), logger)
	if _, err := scheduler.Register("@every 1m", heartbeatJob); err != nil {
		return err
	}
	trafficPeriodResetJob := job.NewTrafficPeriodResetJob(userTrafficService, logger)
	if _, err := scheduler.Register("0 0 0 * * *", trafficPeriodResetJob); err != nil {
		return err
	}
	agentTrafficResetJob := job.NewAgentTrafficResetJob(agentTrafficLifecycleService, logger)
	if _, err := scheduler.Register("@every 5m", agentTrafficResetJob); err != nil {
		return err
	}
	accessLogCleanupJob := job.NewAccessLogCleanupJob(accessLogService, logger)
	if _, err := scheduler.Register("@every 1h", accessLogCleanupJob); err != nil {
		return err
	}
	agentHostMetricsFlushJob := job.NewAgentHostMetricsFlushJob(agentHostService)
	if _, err := scheduler.Register("@every 3s", agentHostMetricsFlushJob); err != nil {
		return err
	}
	// Schedule mesh routing table computation and push every 60 seconds
	if err := registerSchedulerJob(scheduler, "mesh_routing_compute", "@every 60s", &meshRoutingJob{meshService: meshService, logger: logger}); err != nil {
		return err
	}

	// 每日解锁检测：对所有 agent 下发探索命令
	unlockProbeDailyJob := job.NewUnlockProbeDailyJob(unlockProbeService, store.AgentHosts(), logger)
	if _, err := scheduler.Register("0 6 * * *", unlockProbeDailyJob); err != nil {
		return err
	}

	scheduler.Start()

	services := api.Services{
		Config:                  service.NewConfigService(store.Settings(), i18nManager),
		User:                    service.NewUserService(store.Users(), store.Settings(), infra.Hasher),
		UserStat:                userStatService,
		Auth:                    service.NewAuthService(store.Users(), store.Settings(), store.LoginLogs(), store.Tokens(), infra.Hasher, infra.Token, infra.RateLimiter, infra.Audit, infra.Cache),
		AdminPath:               service.NewAdminPathService(store.Settings()),
		Install:                 installService,
		AdminPlan:               adminPlanService,
		AdminUser:               adminUserService,
		AdminServer:             adminServerService,
		AdminStat:               adminStatService,
		AdminNodeStat:           adminNodeStatService,
		AdminSystem:             adminSystemService,
		AdminSystemSettings:     adminSystemSettingsService,
		AdminNotice:             adminNoticeService,
		AdminKnowledge:          adminKnowledgeService,
		UserKnowledge:           userKnowledgeService,
		UserNotice:              userNoticeService,
		ServerAuth:              serverAuthService,
		ServerNode:              serverNodeService,
		Traffic:                 serverTrafficService,
		Telemetry:               serverTelemetryService,
		Verify:                  verifyService,
		Password:                passwordService,
		Register:                registrationService,
		MailLink:                mailLinkService,
		Comm:                    commService,
		Plan:                    planService,
		Server:                  service.NewServerService(store.Users(), store.Servers(), store.Plans()),
		Subscription:            service.NewSubscriptionService(store.Users(), store.Servers(), store.Settings(), store.Plans(), store.SubscriptionTemplates(), subscriptionSourceService, protocolManager, serverTelemetryService, subLogQueue, cfg.Security.SubscribeObfuscation, userServerSelectionService, i18nManager, subscriptionFilterService),
		SubscriptionFilter:      subscriptionFilterService,
		SubscriptionSource:      subscriptionSourceService,
		AgentHost:               agentHostService,
		AgentCore:               agentCoreService,
		Forwarding:              forwardingService,
		AccessLog:               accessLogService,
		UnlockProbe:             unlockProbeService,
		ExitNodeSet:             exitNodeSetService,
		RoutingPolicy:           routingPolicyService,
		InboundSpec:             inboundSpecService,
		CoreConfigItem:          coreConfigItemService,
		DriftAndDiff:            driftAndDiffService,
		ApplyOrchestrator:       applyOrchestratorService,
		OperationLog:            operationLogService,
		AgentLifecycleOperation: agentLifecycleOperationService,
		AgentTrafficLifecycle:   agentTrafficLifecycleService,
		BinaryVersion:           binaryVersionService,
		UserSelection:           userServerSelectionService,
		ShortLink:               shortLinkService,
		MCPApiKeys:              service.NewMCPApiKeyService(store.MCPApiKeys()),
		CDN:                     cdnService,
		Mesh:                    meshService,
		TrafficQueue:            trafficQueue,
		SubLogQueue:             subLogQueue,
		I18n:                    i18nManager,
	}

	corsOrigins := deriveCORSOrigins(cfg)
	router := api.NewRouter(
		logger,
		services,
		cfg.Metrics,
		api.WithCORSAllowedOrigins(corsOrigins),
		api.WithAdminUI(api.AdminUIOptions{
			Enabled:         cfg.UI.Admin.Enabled,
			Dir:             cfg.UI.Admin.Dir,
			BaseURL:         cfg.UI.Admin.BaseURL,
			Title:           cfg.UI.Admin.Title,
			Version:         runtimeVersion,
			Logo:            cfg.UI.Admin.Logo,
			DeployScriptURL: cfg.UI.Admin.DeployScriptURL,
			HiddenModules:   cfg.UI.Admin.HiddenModules,
		}),
		api.WithUserUI(api.UserUIOptions{
			Enabled: cfg.UI.User.Enabled,
			Dir:     cfg.UI.User.Dir,
			BaseURL: cfg.UI.User.BaseURL,
			Title:   cfg.UI.User.Title,
		}),
		api.WithInstallUI(api.InstallUIOptions{
			Enabled: cfg.UI.Install.Enabled,
			Dir:     cfg.UI.Install.Dir,
		}),
	)

	// Initialize AgentLogCache (shared by gRPC and MCP)
	agentLogCache := service.NewAgentLogCache(infra.Cache, cfg.MCP.MaxAgentLogLines)

	// Check runtime MCP enabled state from DB settings (overrides config file)
	if mcpSettings, err := adminSystemSettingsService.GetByCategory(ctx, "mcp"); err == nil {
		if mcpSettings["enabled"] == "true" {
			cfg.MCP.Enabled = true
		} else if mcpSettings["enabled"] == "false" {
			cfg.MCP.Enabled = false
		}
	}

	// Initialize MCP API key service and MCP server
	mcpAPIKeySvc := service.NewMCPApiKeyService(store.MCPApiKeys())
	mcpKeyValidator := service.KeyCheckFunc(func(rawKey string) (bool, error) {
		_, err := mcpAPIKeySvc.Validate(context.Background(), rawKey)
		return err == nil, nil
	})
	if cfg.MCP.Enabled {
		mcpRegistry := tools.NewRegistry()

		// Register all tool handlers
		mcpRegistry.Register(tools.NewSystemStatusHandler(adminSystemService))
		mcpRegistry.Register(tools.NewSystemSettingsHandler(adminSystemSettingsService))
		mcpRegistry.Register(tools.NewAgentListHandler(agentHostService))
		mcpRegistry.Register(tools.NewAgentStatusHandler(agentHostService))
		mcpRegistry.Register(tools.NewAgentConfigYAMLHandler(agentHostService))
		mcpRegistry.Register(tools.NewAgentLogsFetchHandler(agentLogCache))
		mcpRegistry.Register(tools.NewServerListHandler(adminServerService))
		mcpRegistry.Register(tools.NewServerStatsHandler(adminNodeStatService))
		mcpRegistry.Register(tools.NewUserListHandler(adminUserService))
		mcpRegistry.Register(tools.NewUserDetailHandler(adminUserService))
		mcpRegistry.Register(tools.NewPlanListHandler(planService))
		mcpRegistry.Register(tools.NewCDNSiteListHandler(cdnService))
		mcpRegistry.Register(tools.NewMeshNetworkHandler(meshService))
		mcpRegistry.Register(tools.NewOperationLogsListHandler(operationLogService))
		mcpRegistry.Register(tools.NewAccessLogsListHandler(accessLogService))
		mcpRegistry.Register(tools.NewServerLogHandler(cfg.Log.LogDir, cfg.MCP.ServerLogMaxLines))
		mcpRegistry.Register(tools.NewConfigArtifactsHandler(driftAndDiffService))
		mcpRegistry.Register(tools.NewServerLogTailHandler(cfg.Log.LogDir, cfg.MCP.ServerLogMaxLines))

		mcpServer := mcp.NewServer(mcp.Config{
			APIKey:            cfg.MCP.APIKey,
			LogDir:            cfg.Log.LogDir,
			ServerLogMaxLines: cfg.MCP.ServerLogMaxLines,
		}, mcpRegistry, logger, mcp.WithKeyValidator(mcpKeyValidator))

		// Mount MCP routes on the chi router
		if r, ok := router.(chi.Router); ok {
			mcpServer.Mount(r)
		} else {
			logger.Warn("cannot mount MCP server: router is not a chi.Router")
		}
	}

	server := bootstrap.NewHTTPServer(legacyCfg, router)

	// Start gRPC server if enabled
	var grpcServer *internalgrpc.Server
	if cfg.GRPC.Enabled {
		authInterceptor := interceptor.NewAuthInterceptor(agentHostService)
		agentHandler := handler.NewAgentHandlerWithCoreServices(
			agentHostService,
			agentService,
			serverTelemetryService,
			serverNodeService,
			userTrafficService,
			store.TrafficReportDedups(),
			forwardingService,
			accessLogService,
			adminSystemSettingsService,
			inventoryIngestService,
			applyOrchestratorService,
			coreOperationService,
			coreSnapshotService,
			operationLogService,
			agentLifecycleOperationService,
			agentTrafficLifecycleService,
			binaryVersionService,
			cdnService,
			meshService,
			agentLogCache,
			logger,
		)

		grpcCfg := internalgrpc.Config{
			Address: cfg.GRPC.Addr,
		}
		if cfg.GRPC.TLS.Enabled {
			grpcCfg.TLS = &internalgrpc.TLSConfig{
				Enabled:  true,
				CertFile: cfg.GRPC.TLS.CertFile,
				KeyFile:  cfg.GRPC.TLS.KeyFile,
			}
		}

		var err error
		grpcServer, err = internalgrpc.NewServer(grpcCfg, agentHandler, authInterceptor, logger)
		if err != nil {
			return err
		}
	}

	if cfg.GRPC.Enabled && cfg.GRPC.ReuseHTTPPort {
		server.WriteTimeout = 0
		muxHandler := h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if internalgrpc.IsGRPCRequest(r) {
				grpcServer.Handler().ServeHTTP(w, r)
				return
			}
			router.ServeHTTP(w, r)
		}), &http2.Server{})
		server.Handler = muxHandler

		lis, err := net.Listen("tcp", cfg.HTTP.Addr)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", cfg.HTTP.Addr, err)
		}
		defer lis.Close() // redundant with server.Shutdown below, but safe to close twice

		go func() {
			logger.Info("http+grpc mux server starting", "addr", cfg.HTTP.Addr, "env", cfg.Log.Environment)
			if err := server.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("http+grpc mux server failed", "error", err)
				stop()
			}
		}()
	} else {
		if cfg.GRPC.Enabled {
			go func() {
				logger.Info("gRPC server starting", "addr", cfg.GRPC.Addr)
				if err := grpcServer.Start(); err != nil {
					logger.Error("gRPC server failed", "error", err)
					stop()
				}
			}()
		}

		go func() {
			logger.Info("http server starting", "addr", cfg.HTTP.Addr, "env", cfg.Log.Environment)
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("http server failed", "error", err)
				stop()
			}
		}()
	}

	<-ctx.Done()
	shutdownTimeout := cfg.HTTP.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}
	stopCtx := scheduler.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(shutdownTimeout):
		logger.Warn("scheduler stop timed out, forcing shutdown")
	}
	// Each shutdown gets its own independent timeout so they don't
	// compete for the same deadline window.
	grpcShutdownCtx, grpcShutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer grpcShutdownCancel()

	httpShutdownCtx, httpShutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer httpShutdownCancel()

	var wg sync.WaitGroup

	// Shutdown gRPC server (concurrent with HTTP)
	if grpcServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("shutting down gRPC server")
			done := make(chan struct{})
			go func() {
				grpcServer.GracefulStop()
				close(done)
			}()
			select {
			case <-done:
				logger.Info("gRPC server gracefully stopped")
			case <-grpcShutdownCtx.Done():
				logger.Warn("gRPC server GracefulStop timeout, forcing stop")
				grpcServer.Stop()
			}
		}()
	}

	// Shutdown HTTP server (concurrent with gRPC)
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("shutting down http server")
		if err := server.Shutdown(httpShutdownCtx); err != nil {
			logger.Error("http server shutdown error", "error", err)
		}
	}()

	wg.Wait()
	logger.Info("server exited cleanly")
	return nil
}

// deriveCORSOrigins 返回 CORS 允许的来源列表，优先使用配置项，否则返回 nil（使用默认行为）。
func deriveCORSOrigins(cfg *config.Config) []string {
	if cfg != nil && len(cfg.Security.CORSAllowedOrigins) > 0 {
		return cfg.Security.CORSAllowedOrigins
	}
	return nil
}
