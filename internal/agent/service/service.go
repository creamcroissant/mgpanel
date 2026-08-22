package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/creamcroissant/mgpanel/internal/agent/access"
	"github.com/creamcroissant/mgpanel/internal/agent/api"
	"github.com/creamcroissant/mgpanel/internal/agent/capability"
	"github.com/creamcroissant/mgpanel/internal/agent/cdn"
	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/config"
	"github.com/creamcroissant/mgpanel/internal/agent/configcenter"
	"github.com/creamcroissant/mgpanel/internal/agent/core"
	"github.com/creamcroissant/mgpanel/internal/agent/forwarding"
	agentgrpc "github.com/creamcroissant/mgpanel/internal/agent/grpc"
	"github.com/creamcroissant/mgpanel/internal/agent/initsys"
	"github.com/creamcroissant/mgpanel/internal/agent/loguploader"
	"github.com/creamcroissant/mgpanel/internal/agent/mesh"
	"github.com/creamcroissant/mgpanel/internal/agent/monitor"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol/subscribe"
	"github.com/creamcroissant/mgpanel/internal/agent/proxy"
	"github.com/creamcroissant/mgpanel/internal/agent/server"
	"github.com/creamcroissant/mgpanel/internal/agent/syncer"
	"github.com/creamcroissant/mgpanel/internal/agent/traffic"
	"github.com/creamcroissant/mgpanel/internal/agent/transport"
	"github.com/creamcroissant/mgpanel/internal/agent/unlock"
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

	logUploader *loguploader.Uploader // Periodic log uploader
	unlockMgr   *unlock.Manager       // 流媒体解锁检测管理器

	cachedAdvertiseHost string // 缓存探测到的公网 IP，避免每次 sync 都调外网 API

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

	if agent.commandQueue != nil {
		if err := agent.registerRoutingTableHandler(); err != nil {
			return nil, fmt.Errorf("register routing table handler: %w", err)
		}
		if err := agent.registerUnlockProbeHandler(); err != nil {
			return nil, fmt.Errorf("register unlock probe handler: %w", err)
		}
	}

	if err := agent.registerResetLinksHandler(); err != nil {
		return nil, fmt.Errorf("register reset links handler: %w", err)
	}
	if err := agent.registerReportConfigHandler(); err != nil {
		return nil, fmt.Errorf("register report config handler: %w", err)
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

	// Start WireGuard mesh interface
	if a.meshManager != nil {
		if err := a.meshManager.Start(ctx); err != nil {
			slog.Warn("mesh: failed to start WireGuard interface", "error", err)
		}
	}

	// Start mesh peer latency prober
	if a.meshProber != nil {
		a.meshProber.Start(ctx)
	}

	if a.commandQueue != nil {
		a.commandQueue.Start(ctx)
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
				cancel()
			}
			if a.switcher != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = a.switcher.Shutdown(shutdownCtx)
				cancel()
			}
			// Stop mesh services
			if a.meshProber != nil {
				a.meshProber.Stop()
			}
			// Stop WireGuard mesh interface
			if a.meshManager != nil {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := a.meshManager.Stop(stopCtx); err != nil {
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

func (a *Agent) sync(ctx context.Context) {
	if !a.beginSync() {
		slog.Debug("sync already in flight, skip re-entry")
		return
	}
	defer a.endSync()

	a.rollbackExpiredUpdateIfNeeded()
	if a.conn != nil {
		state := a.conn.CheckConnection(ctx)
		if state != transport.StateConnected {
			slog.Warn("gRPC connection not ready, skip sync", "state", state.String())
			return
		}
	}
	a.syncGRPC(ctx)
}

func (a *Agent) syncGRPC(ctx context.Context) {
	a.syncAgentHost(ctx)
	a.syncApplyBatch(ctx)
	a.syncCoreOperations(ctx)
	a.syncAgentCommands(ctx)

	// Sync mesh configuration — auto-initialize if agent has a mesh record
	if a.meshManager == nil {
		a.tryEnableMesh(ctx)
	}
	if a.meshManager != nil {
		a.syncMeshConfig(ctx)
	}

	// Update mesh probe targets and sync latencies
	if a.meshProber != nil {
		a.meshProber.UpdateTargets(ctx)
	}

	// NodeID kept for compatibility; gRPC identifies agent host by token
	nodeID := int32(a.cfg.Panel.NodeID)

	// Fetch Config via gRPC
	cfgResp, err := a.grpc.GetConfig(ctx, nodeID, a.configETag)
	if err != nil {
		slog.Error("Failed to fetch config via gRPC", "error", err)
		return
	}

	if !cfgResp.NotModified {
		a.configETag = cfgResp.Etag
		slog.Info("Config updated via gRPC", "version", cfgResp.Version)
		// Apply new config
		if len(cfgResp.ConfigJson) > 0 {
			if err := a.protoMgr.ApplyConfigWithCore(ctx, "", "config.json", cfgResp.ConfigJson); err != nil {
				slog.Error("Failed to apply config", "error", err)
			} else {
				slog.Info("Successfully applied new config", "version", cfgResp.Version)
			}
		}
	}

	// Fetch Users via gRPC
	usersResp, err := a.grpc.GetUsers(ctx, nodeID, a.usersETag, 0)
	if err != nil {
		slog.Error("Failed to fetch users via gRPC", "error", err)
		return
	}

	if !usersResp.NotModified {
		a.usersETag = usersResp.Etag
		a.refreshUserEmailMapping(usersResp.Users)
		slog.Info("Users updated via gRPC", "count", len(usersResp.Users))

		// Convert users to protocol.UserConfig and inject into config
		if err := a.applyUsers(ctx, usersResp.Users); err != nil {
			slog.Error("Failed to apply users", "error", err)
		} else {
			slog.Info("Successfully applied users to config", "count", len(usersResp.Users))
		}
	}
}

// tryEnableMesh checks if this agent has a mesh peer record on the panel.
// If yes, it dynamically initializes the mesh manager and prober so the agent
// can auto-join without requiring mesh.enabled: true in the local config.yml.
func (a *Agent) tryEnableMesh(ctx context.Context) {
	joinCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := a.grpc.JoinMesh(joinCtx, &agentv1.JoinMeshRequest{NetworkId: "default"})
	if err != nil {
		// mesh service not available on panel, or agent not in mesh — nothing to do
		return
	}
	if resp == nil || !resp.Success {
		return
	}

	meshDir := filepath.Join(a.cfg.Core.CoreInstallDir, "..", "mesh")
	meshDir = filepath.Clean(meshDir)

	a.meshManager = mesh.NewManager(mesh.Config{
		ConfigDir:     meshDir,
		InterfaceName: "wgmesh0",
		ListenPort:    51820,
		WgBinary:      "wg",
		NetworkCIDR:   "10.144.0.0/24",
	}, slog.Default())

	a.meshProber = NewMeshProber(a.meshManager, 30*time.Second, 5*time.Second, 10, slog.Default())
	// 动态创建后需显式启动 prober，否则 lifecycleCtx 为 nil，keepalive 无法启动。
	// 必须使用 agent 生命周期上下文（a.ctx），而非本轮 sync 的临时 ctx，避免随 sync 结束被取消。
	a.meshProber.Start(a.ctx)
	slog.Info("mesh: dynamically enabled after discovering panel-side mesh record",
		"ip", resp.WgIp,
	)
}

func (a *Agent) report(ctx context.Context) {
	// Collect I/O data outside lock
	var trafficUpload, trafficDownload uint64
	if a.netio != nil {
		delta, err := a.netio.CollectDelta(ctx)
		if err != nil {
			slog.Error("Failed to collect netio traffic", "error", err)
		} else {
			trafficUpload = delta.Upload
			trafficDownload = delta.Download
		}
	}

	// System Status (I/O outside lock)
	stat, err := a.monitor.Collect()
	if err != nil {
		slog.Error("Failed to collect system stats", "error", err)
		return
	}

	// Inject traffic into status payload
	stat.TrafficUpload = trafficUpload
	stat.TrafficDownload = trafficDownload

	// Collect capabilities (may execute external binary, I/O outside lock)
	caps := a.getCapabilities(ctx)

	a.reportMu.Lock()
	a.rollbackExpiredUpdateIfNeeded()
	a.reportMu.Unlock()

	if a.conn != nil {
		state := a.conn.CheckConnection(ctx)
		if state != transport.StateConnected {
			slog.Warn("gRPC connection not ready, skip report", "state", state.String())
			return
		}
	}
	a.reportGRPC(ctx, stat, caps)

	// User-level Traffic (from traffic collector, e.g., xray_api)
	a.reportUserTraffic(ctx)
}

func (a *Agent) reportGRPC(ctx context.Context, stat api.StatusPayload, caps *capability.DetectedCapabilities) {
	// Collect CDN origin latency probe results
	a.cdnProbeResultsMu.Lock()
	originLatencies := make([]*agentv1.OriginLatencyEntry, len(a.cdnProbeResults))
	copy(originLatencies, a.cdnProbeResults)
	a.cdnProbeResults = a.cdnProbeResults[:0] // Clear slice after copying
	a.cdnProbeResultsMu.Unlock()

	// Build protobuf status report
	statusReport := &agentv1.StatusReport{
		Timestamp: time.Now().Unix(),
		System: &agentv1.SystemMetrics{
			CpuUsage:        stat.CPU,
			MemoryUsage:     float64(stat.Mem.Used) / float64(stat.Mem.Total) * 100,
			MemoryTotal:     float64(stat.Mem.Total),
			MemoryUsed:      float64(stat.Mem.Used),
			DiskUsage:       float64(stat.Disk.Used) / float64(stat.Disk.Total) * 100,
			DiskTotal:       float64(stat.Disk.Total),
			DiskUsed:        float64(stat.Disk.Used),
			UptimeSeconds:   int64(stat.Uptime),
			ConnectionCount: 0, // TODO: Get connection count if available
			Load1:           stat.Load1,
			Load5:           stat.Load5,
			Load15:          stat.Load15,
			ProcessCount:    int32(stat.ProcessCount),
			TcpCount:        int32(stat.TcpCount),
			UdpCount:        int32(stat.UdpCount),
			// Core capabilities
			CoreVersion:     caps.CoreVersion,
			Capabilities:    caps.Capabilities,
			BuildTags:       caps.BuildTags,
			AgentVersion:    a.cfg.Update.CurrentVersion,
			CurrentCoreType: caps.CoreType,
			BootId:          readBootID(),
			AllCores:        buildAllCoresProto(caps.AllCores),
		},
		Network: &agentv1.NetworkMetrics{
			UploadBytes:           stat.NetIO.Up,
			DownloadBytes:         stat.NetIO.Down,
			UploadDelta:           stat.TrafficUpload,
			DownloadDelta:         stat.TrafficDownload,
			RawUploadTotalBytes:   a.collectRawUpload(ctx),
			RawDownloadTotalBytes: a.collectRawDownload(ctx),
			RawCountersPresent:    a.netio != nil,
		},
		CommandQueue:      a.commandQueueStatsProto(),
		UpdateStatus:      a.updateStatusProto(),
		OriginLatencies:   originLatencies,
		MeshPeerLatencies: a.meshPeerLatenciesProto(),
	}

	// Add core instances
	if a.coreMgr != nil {
		statusReport.Instances = buildCoreInstanceReport(a.coreMgr.ListInstances())
	}

	if a.inventoryScanner != nil {
		inventory, inboundIndex, scanErr := a.inventoryScanner.Scan()
		if scanErr != nil {
			slog.Error("Failed to scan config inventory", "error", scanErr)
		} else {
			statusReport.Inventory = inventory
			statusReport.InboundIndex = inboundIndex
		}
	}

	if configsWithDetails, err := a.protoMgr.ListConfigsWithDetailsBySource("managed"); err == nil {
		// Check global service status
		running, _ := a.protoMgr.ServiceStatus(ctx)

		protocols := make([]*agentv1.ProtocolState, 0, len(configsWithDetails))
		for _, cfg := range configsWithDetails {
			state := &agentv1.ProtocolState{
				Name:        cfg.Filename,
				Type:        "sing-box", // Default, will be overridden if detected
				Running:     running,
				ContentHash: cfg.ContentHash,
			}

			// Convert parsed details to protobuf
			if len(cfg.Protocols) > 0 {
				state.Type = cfg.Protocols[0].CoreType // Use detected core type
				details := make([]*agentv1.ProtocolDetails, 0, len(cfg.Protocols))
				for _, p := range cfg.Protocols {
					detail := &agentv1.ProtocolDetails{
						Protocol:   p.Protocol,
						Tag:        p.Tag,
						Listen:     p.Listen,
						Port:       int32(p.Port),
						SourceFile: p.SourceFile,
						CoreType:   p.CoreType,
					}

					// Transport config
					if p.Transport != nil {
						detail.Transport = &agentv1.TransportConfig{
							Type:        p.Transport.Type,
							Path:        p.Transport.Path,
							Host:        p.Transport.Host,
							ServiceName: p.Transport.ServiceName,
						}
					}

					// TLS config
					if p.TLS != nil {
						detail.Tls = &agentv1.TLSConfig{
							Enabled:    p.TLS.Enabled,
							ServerName: p.TLS.ServerName,
							Alpn:       p.TLS.ALPN,
						}
						if p.TLS.Reality != nil {
							detail.Tls.Reality = &agentv1.RealityConfig{
								Enabled:       p.TLS.Reality.Enabled,
								ShortIds:      p.TLS.Reality.ShortIDs,
								ServerName:    p.TLS.Reality.ServerName,
								Fingerprint:   p.TLS.Reality.Fingerprint,
								HandshakeAddr: p.TLS.Reality.HandshakeAddr,
								HandshakePort: int32(p.TLS.Reality.HandshakePort),
								PublicKey:     p.TLS.Reality.PublicKey,
							}
						}
					}

					// Multiplex config
					if p.Multiplex != nil {
						detail.Multiplex = &agentv1.MultiplexConfig{
							Enabled: p.Multiplex.Enabled,
							Padding: p.Multiplex.Padding,
						}
						if p.Multiplex.Brutal != nil {
							detail.Multiplex.Brutal = &agentv1.BrutalConfig{
								Enabled:  p.Multiplex.Brutal.Enabled,
								UpMbps:   int32(p.Multiplex.Brutal.UpMbps),
								DownMbps: int32(p.Multiplex.Brutal.DownMbps),
							}
						}
					}

					// Users
					for _, u := range p.Users {
						detail.Users = append(detail.Users, &agentv1.ProtocolUserInfo{
							Uuid:   u.UUID,
							Flow:   u.Flow,
							Email:  u.Email,
							Method: u.Method,
						})
					}

					details = append(details, detail)
				}
				state.Details = details
			}

			protocols = append(protocols, state)
		}
		statusReport.Protocols = protocols
	} else {
		slog.Error("Failed to list protocol configs", "error", err)
	}

	// Parse subscribe directory for client configs
	if subData, err := a.subParse.Parse(); err == nil && len(subData.Configs) > 0 {
		clientConfigs := make([]*agentv1.ClientConfig, 0, len(subData.Configs))
		for _, cfg := range subData.Configs {
			clientConfig := &agentv1.ClientConfig{
				Name:     cfg.Name,
				Protocol: cfg.Protocol,
				Server:   cfg.Server,
				Port:     int32(cfg.Port),
				// Authentication
				Uuid:     cfg.UUID,
				Password: cfg.Password,
				// Transport
				Network:     cfg.Network,
				Path:        cfg.Path,
				ServiceName: cfg.ServiceName,
				// TLS
				Tls:         cfg.TLS,
				Sni:         cfg.SNI,
				Alpn:        cfg.ALPN,
				Fingerprint: cfg.Fingerprint,
				Insecure:    cfg.Insecure,
				// Reality
				RealityEnabled:   cfg.RealityEnabled,
				RealityPublicKey: cfg.RealityPublicKey,
				RealityShortId:   cfg.RealityShortID,
				// VLESS
				Flow: cfg.Flow,
				// Hysteria2
				HopPorts:    cfg.HopPorts,
				HopInterval: int32(cfg.HopInterval),
				UpMbps:      int32(cfg.UpMbps),
				DownMbps:    int32(cfg.DownMbps),
				// TUIC
				CongestionControl: cfg.CongestionControl,
				// Shadowsocks
				Cipher: cfg.Cipher,
				// ShadowTLS
				ShadowtlsVersion:  int32(cfg.ShadowTLSVersion),
				ShadowtlsPassword: cfg.ShadowTLSPassword,
				// Multiplex
				MuxEnabled: cfg.MuxEnabled,
				MuxPadding: cfg.MuxPadding,
				// Raw configs
				RawConfigs: cfg.RawConfigs,
			}
			clientConfigs = append(clientConfigs, clientConfig)
		}

		statusReport.ClientConfigs = &agentv1.ClientConfigReport{
			Configs:     clientConfigs,
			ContentHash: subData.ContentHash,
		}
		slog.Debug("Parsed subscribe configs", "count", len(clientConfigs))
	} else if err != nil {
		slog.Warn("Failed to parse subscribe directory", "error", err)
	}

	if resp, err := a.grpc.ReportStatus(ctx, statusReport); err != nil {
		slog.Error("Failed to report status via gRPC", "error", err)
	} else {
		if resp == nil || !resp.GetSuccess() {
			message := "empty status response"
			if resp != nil {
				message = resp.GetMessage()
			}
			slog.Warn("Panel rejected status report", "message", message)
			return
		}
		a.confirmUpdaterHealthy()
		slog.Debug("Reported status via gRPC",
			"traffic_up", stat.TrafficUpload,
			"traffic_down", stat.TrafficDownload,
			"inventory_count", len(statusReport.Inventory),
			"inbound_index_count", len(statusReport.InboundIndex))

		// Check for interval updates
		updated := false
		if resp.SyncIntervalSeconds > 0 {
			if previous := a.currentSyncInterval.Swap(resp.SyncIntervalSeconds); previous != resp.SyncIntervalSeconds {
				updated = true
			}
		}
		if resp.ReportIntervalSeconds > 0 {
			if previous := a.currentReportInterval.Swap(resp.ReportIntervalSeconds); previous != resp.ReportIntervalSeconds {
				updated = true
			}
		}

		if updated {
			select {
			case a.updateTickerCh <- struct{}{}:
			default:
			}
		}
	}
}

func (a *Agent) syncApplyBatch(ctx context.Context) {
	if a.batchApplier == nil {
		return
	}
	if a.commandQueue == nil {
		a.syncApplyBatchDirect(ctx)
		return
	}
	if !a.beginBatchSync() {
		slog.Debug("apply batch sync already in flight, skip re-entry")
		return
	}

	currentRevision := a.getApplyRevision()
	task := command.Task{ID: fmt.Sprintf("config-apply-%d", currentRevision), OperationType: agentCommandActionConfigApply}

	done := make(chan struct{})

	var (
		casReleased atomic.Bool
		releaseCAS  = func() {
			if casReleased.CompareAndSwap(false, true) {
				a.endBatchSync()
			}
		}
	)

	err := a.commandQueue.SubmitWithHandler(ctx, task, func(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
		defer releaseCAS()
		defer close(done)
		_ = reporter.Report(ctx, command.Event{EventType: command.EventTypeProgress, Status: command.StatusInProgress, Phase: "applying", Level: command.LevelInfo, Message: "config apply sync started"})
		return a.runApplyBatch(ctx, currentRevision)
	})
	if err != nil {
		releaseCAS()
		close(done)
		slog.Warn("config apply rejected by command queue", "current_revision", currentRevision, "error", err)
		return
	}

	// Watchdog: release CAS if queue stops or times out
	a.watchdogWg.Add(1)
	go func() {
		watchdogTimer := time.NewTimer(2 * time.Minute)
		defer a.watchdogWg.Done()
		defer watchdogTimer.Stop()
		select {
		case <-done:
			// handler completed normally, CAS already released
		case <-a.commandQueue.Stopped():
			slog.Warn("command queue stopped during batch sync, releasing CAS")
			releaseCAS()
		case <-watchdogTimer.C:
			slog.Warn("batch sync watchdog timeout, releasing CAS")
			releaseCAS()
		}
	}()
}

func (a *Agent) syncApplyBatchDirect(ctx context.Context) {
	if !a.beginBatchSync() {
		slog.Debug("apply batch sync already in flight, skip re-entry")
		return
	}
	defer a.endBatchSync()
	_ = a.runApplyBatch(ctx, a.getApplyRevision())
}

func (a *Agent) runApplyBatch(ctx context.Context, currentRevision int64) command.Result {
	nextRevision, err := a.batchApplier.SyncOnce(ctx, currentRevision)
	if err != nil {
		slog.Error("Failed to sync apply batch", "current_revision", currentRevision, "error", err)
		return command.Result{Status: command.StatusFailed, Phase: "failed", Level: command.LevelError, Message: "config apply sync failed", ErrorMessage: err.Error()}
	}
	payload, _ := json.Marshal(map[string]any{"previous_revision": currentRevision, "revision": nextRevision})
	if nextRevision != currentRevision {
		a.setApplyRevision(nextRevision)
		slog.Info("Apply batch synced", "revision", nextRevision)
		return command.Result{Status: command.StatusSuccess, Phase: "completed", Level: command.LevelInfo, Message: "config apply synced", Payload: payload}
	}
	return command.Result{Status: command.StatusSuccess, Phase: "not_modified", Level: command.LevelInfo, Message: "config apply not modified", Payload: payload}
}

func (a *Agent) beginSync() bool {
	return a.syncInFlight.CompareAndSwap(false, true)
}

func (a *Agent) endSync() {
	a.syncInFlight.Store(false)
}

func (a *Agent) beginBatchSync() bool {
	return a.batchSyncInFlight.CompareAndSwap(false, true)
}

func (a *Agent) endBatchSync() {
	a.batchSyncInFlight.Store(false)
}

func (a *Agent) beginCoreOperationSync() bool {
	return a.coreOperationSyncInFlight.CompareAndSwap(false, true)
}

func (a *Agent) endCoreOperationSync() {
	a.coreOperationSyncInFlight.Store(false)
}

func (a *Agent) getApplyRevision() int64 {
	return a.applyRevision.Load()
}

func (a *Agent) setApplyRevision(revision int64) {
	a.applyRevision.Store(revision)
}

func (a *Agent) reportUserTraffic(ctx context.Context) {
	// Use gRPC for traffic reporting
	samples, err := a.traffic.Collect(ctx)
	if err != nil {
		slog.Error("Failed to collect traffic", "error", err)
		return
	}

	if len(samples) == 0 {
		return
	}

	// Convert to protobuf format
	userTraffic := make([]*agentv1.UserTraffic, 0, len(samples))
	unmapped := 0
	for _, s := range samples {
		userID := s.UserID
		if userID <= 0 {
			if mappedID, ok := a.resolveUserIDByUID(s.UID); ok {
				userID = mappedID
			}
		}
		if userID <= 0 {
			unmapped++
			continue
		}
		if s.Upload == 0 && s.Download == 0 {
			continue
		}

		userTraffic = append(userTraffic, &agentv1.UserTraffic{
			UserId:        userID,
			UploadBytes:   s.Upload,
			DownloadBytes: s.Download,
		})
	}

	if len(userTraffic) == 0 {
		if unmapped > 0 {
			slog.Warn("Skip traffic samples due to unresolved user mapping", "unmapped", unmapped, "samples", len(samples))
		}
		return
	}

	reportID := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	if _, err := a.grpc.ReportTraffic(ctx, userTraffic, reportID); err != nil {
		slog.Error("Failed to push traffic via gRPC", "error", err, "report_id", reportID)
	} else {
		slog.Debug("Pushed traffic samples via gRPC", "count", len(userTraffic), "source_samples", len(samples), "unmapped", unmapped, "report_id", reportID)
	}
}

func normalizeUserEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	return strings.ToLower(email)
}

func (a *Agent) refreshUserEmailMapping(users []*agentv1.UserInfo) {
	mapped := make(map[string]int64, len(users))
	for _, u := range users {
		if u == nil || u.UserId <= 0 {
			continue
		}
		email := normalizeUserEmail(u.Email)
		if email == "" {
			continue
		}
		mapped[email] = u.UserId
	}

	a.userEmailMu.Lock()
	a.userIDByEmail = mapped
	a.userEmailMu.Unlock()
}

func (a *Agent) resolveUserIDByUID(uid string) (int64, bool) {
	email := normalizeUserEmail(uid)
	if email == "" {
		return 0, false
	}

	a.userEmailMu.RLock()
	userID, ok := a.userIDByEmail[email]
	a.userEmailMu.RUnlock()
	if !ok || userID <= 0 {
		return 0, false
	}
	return userID, true
}

func buildCoreInstanceReport(instances []*core.CoreInstance) []*agentv1.CoreInstance {
	if len(instances) == 0 {
		return nil
	}

	pbInstances := make([]*agentv1.CoreInstance, 0, len(instances))
	for _, inst := range instances {
		pbInstances = append(pbInstances, &agentv1.CoreInstance{
			Id:       inst.ID,
			CoreType: string(inst.CoreType),
			Status:   string(inst.Status),
			ListenPorts: func() []int32 {
				if len(inst.ListenPorts) == 0 {
					return nil
				}
				ports := make([]int32, len(inst.ListenPorts))
				for i, port := range inst.ListenPorts {
					ports[i] = int32(port)
				}
				return ports
			}(),
			ConfigPath: inst.ConfigPath,
			ConfigHash: inst.ConfigHash,
			Pid:        int32(inst.PID),
			StartedAt:  inst.StartedAt,
			Error:      inst.Error,
		})
	}
	return pbInstances
}

// determineListenPort returns the appropriate Xray listen port based on CDN status.
// When CDN is enabled, Xray should listen on internal port 10000 (behind Caddy).
// When CDN is disabled, Xray should listen on external port 443 directly.
func (a *Agent) determineListenPort() []int {
	if a.cfg.CDN.Enabled {
		return []int{10000}
	}
	return []int{443}
}

// getCapabilities returns cached or fresh capabilities
// Capabilities are cached for 1 hour to avoid excessive command executions
func (a *Agent) getCapabilities(ctx context.Context) *capability.DetectedCapabilities {
	a.capsMu.Lock()
	defer a.capsMu.Unlock()

	now := time.Now().Unix()
	cacheExpiry := int64(3600) // 1 hour

	// Return cached if still valid
	if a.cachedCaps != nil && now-a.capsDetectedAt < cacheExpiry {
		return a.cachedCaps
	}

	// Detect fresh capabilities
	caps := a.capDet.Detect(ctx)

	// Cache the result
	a.cachedCaps = caps
	a.capsDetectedAt = now

	slog.Info("Detected core capabilities",
		"core_type", caps.CoreType,
		"version", caps.CoreVersion,
		"capabilities", caps.Capabilities,
		"build_tags", caps.BuildTags)

	return caps
}

// applyUsers converts gRPC UserInfo to protocol.UserConfig and injects them into the config.
func (a *Agent) applyUsers(ctx context.Context, users []*agentv1.UserInfo) error {
	if len(users) == 0 {
		return nil
	}

	// Convert gRPC UserInfo to protocol.UserConfig
	userConfigs := make([]protocol.UserConfig, 0, len(users))
	for _, u := range users {
		userConfigs = append(userConfigs, protocol.UserConfig{
			UUID:    u.Uuid,
			Email:   u.Email,
			Enabled: u.Enabled,
		})
	}

	// Detect core type and use appropriate injection method
	coreType := a.protoMgr.DetectCoreType()
	slog.Debug("Detected core type for user injection", "core_type", coreType)

	switch coreType {
	case "xray":
		return a.protoMgr.InjectUsersXray(ctx, userConfigs)
	case "sing-box":
		return a.protoMgr.InjectUsers(ctx, userConfigs)
	default:
		// Try Sing-box first as it's the default
		if err := a.protoMgr.InjectUsers(ctx, userConfigs); err != nil {
			// If that fails, try Xray
			return a.protoMgr.InjectUsersXray(ctx, userConfigs)
		}
		return nil
	}
}

func readBootID() string {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (a *Agent) collectRawUpload(ctx context.Context) *agentv1.MetricUInt64Value {
	if a.netio == nil {
		return nil
	}
	sent, _, err := a.netio.CollectCumulative(ctx)
	if err != nil {
		return nil
	}
	return &agentv1.MetricUInt64Value{Value: sent}
}

func (a *Agent) collectRawDownload(ctx context.Context) *agentv1.MetricUInt64Value {
	if a.netio == nil {
		return nil
	}
	_, recv, err := a.netio.CollectCumulative(ctx)
	if err != nil {
		return nil
	}
	return &agentv1.MetricUInt64Value{Value: recv}
}

// meshPeerLatenciesProto returns the current mesh peer latency probe results
// as protobuf OriginLatencyEntry slice for the status report.
func (a *Agent) meshPeerLatenciesProto() []*agentv1.OriginLatencyEntry {
	if a.meshProber == nil {
		return nil
	}
	return a.meshProber.SyncLatencies()
}

// buildAllCoresProto converts capability.DetectedCoreInfo to agentv1.CoreInfo for gRPC status reporting.
func buildAllCoresProto(cores []capability.DetectedCoreInfo) []*agentv1.CoreInfo {
	if len(cores) == 0 {
		return nil
	}
	out := make([]*agentv1.CoreInfo, 0, len(cores))
	for _, c := range cores {
		out = append(out, &agentv1.CoreInfo{
			Type:         c.Type,
			Version:      c.Version,
			Installed:    c.Version != "",
			Capabilities: c.Capabilities,
		})
	}
	return out
}

// syncAgentHost 周期向面板上报本机公网 IP，使 agent_hosts.host 保持为真实可达地址，
// 避免 pending-* 占位符导致 mesh peer endpoint / 订阅节点 host 不可解析。
// 手动配置的 advertise_host 优先；否则探测公网 IP（带缓存，避免每次 sync 都调外网 API）。
func (a *Agent) syncAgentHost(ctx context.Context) {
	if a == nil || a.cfg == nil {
		return
	}
	hostToken := strings.TrimSpace(a.cfg.Panel.HostToken)
	if hostToken == "" {
		return
	}

	advertiseHost := strings.TrimSpace(a.cfg.Panel.AdvertiseHost)
	if advertiseHost == "" {
		advertiseHost = a.cachedAdvertiseHost
		if advertiseHost == "" {
			advertiseHost = config.DetectPublicIP()
			if advertiseHost == "" {
				return // 探测失败，等下一次 sync 再试
			}
			a.cachedAdvertiseHost = advertiseHost
		}
	}

	base := resolvePanelHTTPBase(a.cfg)
	base = strings.TrimSuffix(base, "/")
	reportURL := base + "/api/v1/agent/host?token=" + url.QueryEscape(hostToken)

	body, err := json.Marshal(map[string]string{"host": advertiseHost})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reportURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("report agent host failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Debug("report agent host rejected", "status", resp.StatusCode)
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	slog.Debug("reported agent host", "host", advertiseHost)
}

// resolvePanelHTTPBase 解析面板 HTTP 基础地址。
// 优先使用 panel.url；为空时从 grpc.address 推断（与注册逻辑一致）。
func resolvePanelHTTPBase(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	base := strings.TrimSpace(cfg.Panel.URL)
	if base != "" {
		return base
	}
	grpcAddr := strings.TrimSpace(cfg.GRPC.Address)
	host, port, err := net.SplitHostPort(grpcAddr)
	if err != nil {
		return ""
	}
	scheme := "http"
	if cfg.GRPC.TLS.Enabled {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}
