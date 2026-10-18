package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/access"
	"github.com/creamcroissant/mgpanel/internal/agent/capability"
	"github.com/creamcroissant/mgpanel/internal/agent/cdn"
	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/config"
	"github.com/creamcroissant/mgpanel/internal/agent/configcenter"
	"github.com/creamcroissant/mgpanel/internal/agent/core"
	"github.com/creamcroissant/mgpanel/internal/agent/egressroute"
	"github.com/creamcroissant/mgpanel/internal/agent/forwarding"
	"github.com/creamcroissant/mgpanel/internal/agent/geoip"
	agentgrpc "github.com/creamcroissant/mgpanel/internal/agent/grpc"
	"github.com/creamcroissant/mgpanel/internal/agent/initsys"
	"github.com/creamcroissant/mgpanel/internal/agent/loguploader"
	"github.com/creamcroissant/mgpanel/internal/agent/mesh"
	"github.com/creamcroissant/mgpanel/internal/agent/monitor"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol/subscribe"
	"github.com/creamcroissant/mgpanel/internal/agent/proxy"
	"github.com/creamcroissant/mgpanel/internal/agent/relayroute"
	"github.com/creamcroissant/mgpanel/internal/agent/secrets"
	"github.com/creamcroissant/mgpanel/internal/agent/server"
	"github.com/creamcroissant/mgpanel/internal/agent/syncer"
	"github.com/creamcroissant/mgpanel/internal/agent/traffic"
	"github.com/creamcroissant/mgpanel/internal/agent/transport"
	"github.com/creamcroissant/mgpanel/internal/agent/unlock"
	"github.com/creamcroissant/mgpanel/internal/agent/v2rayapi"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

type Agent struct {
	cfg             *config.Config
	grpc            *transport.GRPCClient
	coreOperations  coreOperationClient
	operationEvents operationEventReporter
	agentCommands   agentCommandClient
	commandQueue    *command.Queue
	updater         agentUpdater
	conn            *transport.ConnectionManager
	forward         *forwarding.Manager
	syncer          *syncer.Syncer
	monitor         *monitor.Monitor
	traffic         traffic.Collector
	netio           *traffic.NetIOCollector // Node-level network traffic
	access          *access.Manager         // Access log manager
	protoMgr        *protocol.Manager
	coreMgr         *core.Manager
	switcher        *proxy.Switcher
	server          *server.Server
	grpcServer      *agentgrpc.Server
	subParse        *subscribe.Parser    // Subscribe directory parser
	capDet          *capability.Detector // Capability detector

	cdnManager  *cdn.Manager  // CDN / Caddy manager
	meshManager *mesh.Manager // WireGuard mesh network manager
	meshProber  *MeshProber   // Mesh peer latency prober
	// meshMu 保护 meshManager/meshProber/cachedAdvertiseHost：
	// OnStateChange 回调 goroutine 与主循环会并发读写这三者，
	// 所有运行期访问必须经 getMesh/setMesh 等 accessor，禁止直接触字段。
	meshMu sync.Mutex

	logUploader *loguploader.Uploader // Periodic log uploader
	unlockMgr   *unlock.Manager       // 流媒体解锁检测管理器
	geoReporter *geoip.Reporter       // 公网 IP + 国家地区周期探测器
	v2rayapiMgr *v2rayapi.Manager     // Runtime user management (HandlerService via v2ray-api)

	cachedAdvertiseHost string              // 缓存探测到的公网 IP，避免每次 sync 都调外网 API
	relayRouteMgr       *relayroute.Manager // 中继链路内核路由管理器（惰性构造，meshMu 保护）
	egressRouteMgr      egressRouteManager  // 出口集内核分发管理器（惰性构造，meshMu 保护）

	batchApplier              applyBatchRunner
	inventoryScanner          *configcenter.AgentInventoryScanner
	applyRevision             atomic.Int64
	syncInFlight              atomic.Bool
	batchSyncInFlight         atomic.Bool
	coreOperationSyncInFlight atomic.Bool
	stateChangeInFlight       atomic.Bool

	configETag     string
	usersETag      string
	userEmailMu    sync.RWMutex
	userIDByEmail  map[string]int64
	reportMu       sync.Mutex
	capsMu         sync.RWMutex
	cachedCaps     *capability.DetectedCapabilities // Cached capabilities
	capsDetectedAt int64                            // Last capability detection time

	// Dynamic intervals
	currentSyncInterval   atomic.Int32
	currentReportInterval atomic.Int32
	updateTickerCh        chan struct{}

	serverWg sync.WaitGroup

	// forwardWg waits for the forwarding sync goroutine during shutdown.
	forwardWg sync.WaitGroup

	// onStateChangeWg waits for OnStateChange goroutines during shutdown.
	onStateChangeWg sync.WaitGroup

	cdnProbeInFlight atomic.Bool
	cdnProbeWg       sync.WaitGroup
	watchdogWg       sync.WaitGroup

	cdnProbeResults   []*agentv1.OriginLatencyEntry
	cdnProbeResultsMu sync.Mutex

	// ctx is the agent's main lifecycle context, set in Run().
	ctx context.Context

	configFilePath    string
	configFileContent string
	configFileMu      sync.RWMutex
}

type applyBatchRunner interface {
	SyncOnce(ctx context.Context, currentRevision int64) (int64, error)
}

func New(cfg *config.Config) (*Agent, error) {
	tCollector, err := traffic.NewCollector(cfg.Traffic)
	if err != nil {
		return nil, err
	}

	retryCfg := transport.RetryConfig{}

	// Initialize NetIO collector for node-level traffic
	var netioCollector *traffic.NetIOCollector
	if cfg.Traffic.Type == "netio" {
		netioCollector = traffic.NewNetIOCollector(cfg.Traffic.Interface)
	}

	// Initialize init system
	initSysCfg := initsys.Config{
		Type:        cfg.Protocol.InitSystem,
		ServiceName: cfg.Protocol.ServiceName,
		Custom: initsys.CustomCommands{
			Start:   cfg.Protocol.CustomCommands.Start,
			Stop:    cfg.Protocol.CustomCommands.Stop,
			Restart: cfg.Protocol.CustomCommands.Restart,
			Reload:  cfg.Protocol.CustomCommands.Reload,
			Status:  cfg.Protocol.CustomCommands.Status,
			Enable:  cfg.Protocol.CustomCommands.Enable,
			Disable: cfg.Protocol.CustomCommands.Disable,
		},
	}
	initSys, err := initsys.New(initSysCfg)
	if err != nil {
		return nil, err
	}
	initSysSingBox := initSys
	initSysXray := initSys
	if generic, ok := initSys.(*initsys.Generic); ok {
		singInit := *generic
		singInit.BinaryPath = cfg.Core.SingBoxBinaryPath
		singInit.Args = []string{"run", "-c", filepath.Join(cfg.Protocol.ConfigDir, "config.json")}
		initSysSingBox = &singInit

		xrayInit := *generic
		xrayInit.BinaryPath = cfg.Core.XrayBinaryPath
		xrayInit.Args = []string{"run", "-config", filepath.Join(cfg.Protocol.ConfigDir, "config.json")}
		initSysXray = &xrayInit
	}

	// Initialize protocol manager
	protoCfg := protocol.Config{
		ConfigDir:        cfg.Protocol.ConfigDir,
		LegacyConfigDir:  cfg.Protocol.LegacyConfigDir,
		ManagedConfigDir: cfg.Protocol.ManagedConfigDir,
		MergeOutputFile:  cfg.Protocol.MergeOutputFile,
		ServiceName:      cfg.Protocol.ServiceName,
		ValidateCmd:      cfg.Protocol.ValidateCmd,
		ServiceAction:    cfg.Protocol.ServiceAction,
		AutoRestart:      cfg.Protocol.AutoRestart,
		PreHook:          cfg.Protocol.PreHook,
		PostHook:         cfg.Protocol.PostHook,
	}
	protoMgr := protocol.NewManager(protoCfg, initSys)

	// 本机 Reality 密钥：按配置（inbound tag）独立生成并持久化到本地文件，
	// 下发配置写盘前注入。私钥永不出机器，也不在机器/配置间复用。
	secretMgr := secrets.NewManager(cfg.Protocol.SecretsFile)
	if err := secretMgr.LoadOrCreate(); err != nil {
		slog.Error("failed to load or create agent secrets", "path", cfg.Protocol.SecretsFile, "error", err)
	} else {
		protoMgr.SetSecrets(secretMgr)
	}

	// Initialize WireGuard mesh manager
	var meshMgr *mesh.Manager
	if cfg.Mesh.Enabled {
		meshDir := cfg.Mesh.ConfigDir
		if meshDir == "" {
			meshDir = filepath.Join(cfg.Core.CoreInstallDir, "..", "mesh")
			meshDir = filepath.Clean(meshDir)
		}
		listenPort := cfg.Mesh.ListenPort
		if listenPort <= 0 {
			listenPort = 51820
		}
		wgBinary := cfg.Mesh.WgBinary
		if wgBinary == "" {
			wgBinary = "wg"
		}
		networkCIDR := cfg.Mesh.NetworkCIDR
		if networkCIDR == "" {
			networkCIDR = "10.144.0.0/24"
		}
		meshMgr = mesh.NewManager(mesh.Config{
			ConfigDir:     meshDir,
			InterfaceName: "wgmesh0",
			ListenPort:    listenPort,
			WgBinary:      wgBinary,
			NetworkCIDR:   networkCIDR,
		}, slog.Default())
	}

	// Initialize mesh peer latency prober
	var meshProber *MeshProber
	if meshMgr != nil {
		probeInterval := time.Duration(cfg.Mesh.Probe.Interval) * time.Second
		if probeInterval <= 0 {
			probeInterval = 30 * time.Second
		}
		probeTimeout := time.Duration(cfg.Mesh.Probe.Timeout) * time.Second
		if probeTimeout <= 0 {
			probeTimeout = 5 * time.Second
		}
		windowSize := cfg.Mesh.Probe.WindowSize
		if windowSize <= 0 {
			windowSize = 10
		}
		meshProber = NewMeshProber(meshMgr, probeInterval, probeTimeout, windowSize, slog.Default())
	}

	capDet := capability.NewDetector(cfg.Core.SingBoxBinaryPath, cfg.Core.XrayBinaryPath)
	coreMgr := core.NewManager()
	coreMgr.Register(core.NewSingBoxCore(initSysSingBox, capDet, cfg.Protocol.ServiceName, cfg.Protocol.ConfigDir))
	coreMgr.Register(core.NewXrayCore(initSysXray, capDet, cfg.Protocol.ServiceName, cfg.Traffic.Address, cfg.Protocol.ConfigDir))

	var switcher *proxy.Switcher
	if cfg.Proxy.Enabled {
		switcherOpts := proxy.SwitcherOptions{
			CoreManager: coreMgr,
			OutputPath:  cfg.Core.OutputPath,
			Logger:      slog.Default(),
			Config: proxy.SwitcherConfig{
				PortRangeStart: cfg.Proxy.PortRangeStart,
				PortRangeEnd:   cfg.Proxy.PortRangeEnd,
				MaxRetries:     cfg.Proxy.MaxRetries,
				HealthTimeout:  cfg.Proxy.HealthTimeout,
				HealthInterval: cfg.Proxy.HealthInterval,
				DrainTimeout:   cfg.Proxy.DrainTimeout,
				NftBin:         cfg.Proxy.NftBin,
				ConntrackBin:   cfg.Proxy.ConntrackBin,
				NftTableName:   cfg.Proxy.NftTableName,
				PIDDir:         cfg.Proxy.PIDDir,
				CgroupBasePath: cfg.Proxy.CgroupBasePath,
			},
		}
		created, err := proxy.NewSwitcher(switcherOpts)
		if err != nil {
			return nil, err
		}
		if err := created.Initialize(context.Background()); err != nil {
			return nil, err
		}
		switcher = created
	}

	var srv *server.Server
	if cfg.Server.Enabled {
		srvCfg := server.Config{
			Listen:    cfg.Server.Listen,
			AuthToken: cfg.Server.AuthToken,
		}
		srv = server.NewServer(srvCfg, protoMgr)
	}

	agentUpdater, err := newAgentUpdater(cfg.Update)
	if err != nil {
		return nil, err
	}
	if err := agentUpdater.RecordStartup(); err != nil {
		slog.Warn("agent updater startup recovery failed", "error", err)
	}

	agent := &Agent{
		cfg:         cfg,
		syncer:      syncer.New(cfg.Core),
		monitor:     monitor.New(),
		traffic:     tCollector,
		netio:       netioCollector,
		protoMgr:    protoMgr,
		coreMgr:     coreMgr,
		switcher:    switcher,
		server:      srv,
		subParse:    subscribe.NewParser(cfg.Protocol.SubscribeDir),
		capDet:      capDet,
		updater:     agentUpdater,
		meshProber:  meshProber,
		meshManager: meshMgr,

		userIDByEmail:  make(map[string]int64),
		updateTickerCh: make(chan struct{}, 1),
	}
	agent.currentSyncInterval.Store(int32(cfg.Interval.Sync))
	agent.currentReportInterval.Store(int32(cfg.Interval.Report))

	// Legacy agent-side passive gRPC server retired; keep config fields inert for transition period.
	if cfg.GRPC.Retry != nil {
		retryCfg = transport.RetryConfig{
			Enabled:         cfg.GRPC.Retry.Enabled,
			MaxRetries:      cfg.GRPC.Retry.MaxRetries,
			InitialInterval: cfg.GRPC.Retry.InitialInterval,
			MaxInterval:     cfg.GRPC.Retry.MaxInterval,
			Multiplier:      cfg.GRPC.Retry.Multiplier,
		}
	}
	timeoutCfg := transport.TimeoutConfig{
		Default: cfg.GRPC.Timeout.Default,
		Connect: cfg.GRPC.Timeout.Connect,
	}

	grpcCfg := transport.Config{
		Address: cfg.GRPC.Address,
		Token:   cfg.Panel.HostToken,
		Keepalive: &transport.KeepaliveConfig{
			Time:    cfg.GRPC.Keepalive.Time,
			Timeout: cfg.GRPC.Keepalive.Timeout,
		},
		Retry:   retryCfg,
		Timeout: timeoutCfg,
	}

	if cfg.GRPC.TLS.Enabled {
		grpcCfg.TLS = &transport.TLSConfig{
			Enabled:            true,
			CertFile:           cfg.GRPC.TLS.CertFile,
			KeyFile:            cfg.GRPC.TLS.KeyFile,
			CAFile:             cfg.GRPC.TLS.CAFile,
			InsecureSkipVerify: cfg.GRPC.TLS.InsecureSkipVerify,
		}
	}

	grpcClient, err := transport.NewGRPCClient(grpcCfg)
	if err != nil {
		return nil, err
	}
	agent.grpc = grpcClient
	agent.coreOperations = grpcClient
	agent.operationEvents = grpcClient
	agent.agentCommands = grpcClient
	commandQueue, err := newAgentCommandQueue(grpcClient)
	if err != nil {
		return nil, err
	}
	agent.commandQueue = commandQueue
	if err := agent.registerAgentUpdateHandlers(); err != nil {
		return nil, err
	}
	if cfg.CDN.Enabled {
		agent.cdnManager = cdn.NewManagerFromConfig(cfg.CDN)
		if err := agent.registerCDNHandlers(); err != nil {
			return nil, err
		}
		slog.Info("CDN management enabled", "bin_path", cfg.CDN.BinPath, "config_dir", cfg.CDN.ConfigDir)
	}
	agent.conn = transport.NewConnectionManager(grpcClient, slog.Default())

	// Initialize log uploader
	if cfg.Log.Upload.Enabled {
		agent.logUploader = loguploader.NewUploader(
			agent.grpc,
			cfg.Log.Dir,
			loguploader.LogUploadConfig{
				Enabled:         cfg.Log.Upload.Enabled,
				MaxLines:        cfg.Log.Upload.MaxLines,
				IntervalSeconds: cfg.Log.Upload.IntervalSeconds,
				Source:          cfg.Log.Upload.Source,
			},
			slog.Default(),
		)
		agent.logUploader.Start()
	}

	agent.conn.SetOnStateChange(func(state transport.ConnectionState) {
		slog.Info("grpc connection state changed", "state", state.String())
		if state == transport.StateConnected {
			if agent.stateChangeInFlight.CompareAndSwap(false, true) {
				agent.onStateChangeWg.Add(1)
				go func() {
					defer agent.onStateChangeWg.Done()
					defer func() {
						if r := recover(); r != nil {
							slog.Error("agent: OnStateChange goroutine panicked",
								"panic", r,
								"stack", string(debug.Stack()))
						}
						agent.stateChangeInFlight.Store(false)
					}()
					ctx, cancel := context.WithTimeout(agent.ctx, 1*time.Minute)
					defer cancel()
					agent.sync(ctx)
					agent.report(ctx)
				}()
			}
		}
	})
	if agent.cfg.Forwarding.Enabled {
		interval := agent.cfg.Forwarding.SyncInterval
		executor := forwarding.NewNFTablesExecutor(agent.cfg.Forwarding.TableName, agent.cfg.Forwarding.NftBin)
		agent.forward = forwarding.NewManager(agent.grpc, executor, interval, slog.Default())
	}

	agent.access = access.NewManager(agent.grpc, agent.coreMgr, cfg.Protocol.ConfigDir, slog.Default())
	// 初始化 GeoIP 周期探测器（每 168h 探测公网 IP + 国家/地区）
	agent.geoReporter = geoip.NewReporter(cfg, slog.Default())

	// 初始化解锁检测管理器（启用时每日自查）
	if cfg.Unlock.Enabled {
		var reporter unlock.Reporter
		baseURL := resolvePanelHTTPBase(cfg)
		if baseURL != "" && strings.TrimSpace(cfg.Panel.HostToken) != "" {
			reporter = unlock.NewHTTPReporter(baseURL, cfg.Panel.HostToken, slog.Default())
		}
		detector := unlock.NewDetector(0)
		interval := time.Duration(cfg.Unlock.IntervalHours) * time.Hour
		if cfg.Unlock.IntervalHours <= 0 {
			interval = 24 * time.Hour
		}
		agent.unlockMgr = unlock.NewManager(detector, reporter, slog.Default(), toUnlockServices(cfg.Unlock.Services), interval)
	}

	agent.inventoryScanner, err = configcenter.NewAgentInventoryScanner(cfg.Protocol, nil)
	if err != nil {
		return nil, err
	}

	applyCoreType := protocol.NormalizeCoreType(cfg.Protocol.ServiceName)
	if applyCoreType == "" {
		applyCoreType = protocol.NormalizeCoreType(protoMgr.DetectCoreType())
	}
	if applyCoreType == "" {
		return nil, fmt.Errorf("unable to determine apply core type from protocol.service_name or current config")
	}
	agent.batchApplier, err = configcenter.NewAgentBatchApplier(cfg.Protocol, applyCoreType, agent.grpc, protoMgr, slog.Default())
	if err != nil {
		return nil, err
	}

	// Runtime user management via v2ray-api (unified HandlerService).
	// Endpoint differs by core: xray reuses Traffic.Address (gRPC API), sing-box
	// uses the injected experimental.v2ray_api listen fragment.
	agent.v2rayapiMgr = v2rayapi.NewManager(applyCoreType, v2rayAPIEndpoint(cfg, applyCoreType))

	if agent.commandQueue != nil {
		if err := agent.registerRoutingTableHandler(); err != nil {
			if err := agent.registerGeoRefreshHandler(); err != nil {
				return nil, fmt.Errorf("register geo refresh handler: %w", err)
			}
			return nil, fmt.Errorf("register routing table handler: %w", err)
		}
		if err := agent.registerUnlockProbeHandler(); err != nil {
			return nil, fmt.Errorf("register unlock probe handler: %w", err)
		}
		if err := agent.registerSyncUsersHandler(); err != nil {
			return nil, fmt.Errorf("register sync users handler: %w", err)
		}
	}

	if err := agent.registerResetLinksHandler(); err != nil {
		return nil, fmt.Errorf("register reset links handler: %w", err)
	}
	if err := agent.registerReportConfigHandler(); err != nil {
		return nil, fmt.Errorf("register report config handler: %w", err)
	}
	if err := agent.registerPushConfigHandler(); err != nil {
		return nil, fmt.Errorf("register push config handler: %w", err)
	}

	// ctx will be set in Run() with the agent lifecycle context

	return agent, nil
}

func (a *Agent) Run(ctx context.Context) {
	a.ctx = ctx

	// Determine mode
	mode := "agent-host"

	panelAddr := a.cfg.GRPC.Address

	slog.Info("Agent started",
		"mode", mode,
		"transport", "grpc",
		"panel", panelAddr,
		"interval_sync", a.cfg.Interval.Sync,
		"interval_report", a.cfg.Interval.Report,
		"init_system", a.protoMgr.InitSystemType(),
	)

	// Start Agent gRPC server if enabled
	if a.grpcServer != nil {
		a.serverWg.Add(1)
		go func() {
			defer a.serverWg.Done()
			if err := a.grpcServer.Start(); err != nil {
				slog.Error("Agent gRPC server error", "error", err)
			}
		}()
		slog.Info("Agent gRPC server enabled", "listen", a.cfg.GRPCServer.Listen)
	}

	// Start HTTP server if enabled
	if a.server != nil {
		a.serverWg.Add(1)
		go func() {
			defer a.serverWg.Done()
			if err := a.server.Start(); err != nil {
				slog.Error("HTTP server error", "error", err)
			}
		}()
		slog.Info("Agent HTTP server enabled", "listen", a.cfg.Server.Listen)
	}

	// Start forwarding sync if enabled
	if a.forward != nil {
		a.forwardWg.Add(1)
		go func() {
			defer a.forwardWg.Done()
			a.forward.Run(ctx)
		}()
	}

	// Start access log collector
	if a.access != nil {
		a.access.Start()
	}

	// Start unlock probe daily self-check
	if a.unlockMgr != nil {
		a.unlockMgr.Start()
	}

	// Start CDN if enabled
	if a.cdnManager != nil {
		if err := a.cdnManager.Start(ctx); err != nil {
			slog.Warn("failed to start caddy", "error", err)
		}
	}

	// Start WireGuard mesh interface（快照后操作，避免与动态启用竞争）
	mgr, prober := a.getMesh()
	if mgr != nil {
		if err := mgr.Start(ctx); err != nil {
			slog.Warn("mesh: failed to start WireGuard interface", "error", err)
		}
	}

	// Start mesh peer latency prober
	if prober != nil {
		prober.Start(ctx)
	}

	if a.commandQueue != nil {
		a.commandQueue.Start(ctx)
		// Start GeoIP 周期探测器（启动时跑一次 + 每 168h）
		if a.geoReporter != nil {
			a.geoReporter.Start(ctx)
		}

		// Inject the sing-box experimental.v2ray_api fragment so runtime user
		// management (HandlerService) is exposed. Xray reuses its own api endpoint.
		if a.v2rayapiMgr != nil && a.v2rayapiMgr.CoreType() == v2rayapi.CoreTypeSingBox {
			if err := ensureV2RayAPIConfig(a.cfg); err != nil {
				slog.Warn("failed to ensure sing-box v2ray_api fragment", "error", err)
			} else {
				slog.Info("sing-box v2ray_api fragment ensured", "dir", a.cfg.Protocol.ConfigDir, "listen", a.cfg.Protocol.V2RayAPIListen)
			}
		}
	}

	// Initial sync
	a.sync(ctx)

	syncTicker := time.NewTicker(time.Duration(a.currentSyncInterval.Load()) * time.Second)
	reportTicker := time.NewTicker(time.Duration(a.currentReportInterval.Load()) * time.Second)

	defer syncTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			if a.logUploader != nil {
				a.logUploader.Stop()
			}
			slog.Info("Agent stopping...")
			if a.grpcServer != nil {
				a.grpcServer.Stop()
			}
			if a.server != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				a.server.Shutdown(shutdownCtx)
				cancel()
			}
			a.serverWg.Wait()
			// passive gRPC server retired
			a.forwardWg.Wait()
			a.onStateChangeWg.Wait()
			a.cdnProbeWg.Wait()
			// Stop commandQueue first so watchdog goroutines can exit immediately
			// via the Stopped() channel instead of waiting for the 2-minute timer.
			if a.commandQueue != nil {
				a.commandQueue.Stop()
			}
			a.watchdogWg.Wait()
			if a.access != nil {
				a.access.Stop()
			}
			if a.unlockMgr != nil {
				a.unlockMgr.Stop()
			}
			if a.grpc != nil {
				a.grpc.Close()
			}
			if a.cdnManager != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				a.cdnManager.Stop(shutdownCtx)
				if a.geoReporter != nil {
					a.geoReporter.Stop()
				}
				cancel()
			}
			if a.switcher != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = a.switcher.Shutdown(shutdownCtx)
				cancel()
			}
			// Stop mesh services（快照后操作）
			smgr, sprober := a.getMesh()
			if sprober != nil {
				sprober.Stop()
			}
			// Stop WireGuard mesh interface
			if smgr != nil {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := smgr.Stop(stopCtx); err != nil {
					slog.Warn("mesh: failed to stop WireGuard interface", "error", err)
				}
				stopCancel()
			}
			return
		case <-a.updateTickerCh:
			syncInterval := a.currentSyncInterval.Load()
			reportInterval := a.currentReportInterval.Load()
			slog.Info("Updating intervals", "sync", syncInterval, "report", reportInterval)
			syncTicker.Reset(time.Duration(syncInterval) * time.Second)
			reportTicker.Reset(time.Duration(reportInterval) * time.Second)
		case <-syncTicker.C:
			a.sync(ctx)
		case <-reportTicker.C:
			// Launch CDN origin probe asynchronously — don't block status report
			if a.cdnProbeInFlight.CompareAndSwap(false, true) {
				a.cdnProbeWg.Add(1)
				go func() {
					defer a.cdnProbeWg.Done()
					defer a.cdnProbeInFlight.Store(false)
					a.probeAndReportOriginLatency(ctx)
				}()
			}
			a.report(ctx)
		}
	}
}

// egressRouteManager 抽象出口集分发内核管理器（真实实现 *egressroute.Manager；测试可注入 fake）。
type egressRouteManager interface {
	Apply(ctx context.Context, assignments []egressroute.Assignment, appliedRevision int64) error
	Probe(ctx context.Context, assignments []egressroute.Assignment) ([]egressroute.MemberState, error)
	// CommitRemovals 拆除 Apply 阶段延迟记账的陈旧规则/表/隧道（GAP-2）：
	// 仅当 currentRevision 大于记账时刻的 revision（自记账以来成功换过配置）才真正拆除。
	CommitRemovals(ctx context.Context, currentRevision int64) error
}
