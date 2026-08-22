package traffic

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/api"
	statscommand "github.com/creamcroissant/mgpanel/pkg/pb/xray/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// XrayCollector 通过 Xray Stats API 采集用户流量。
type XrayCollector struct {
	address string
	conn    *grpc.ClientConn
	client  statscommand.StatsServiceClient
	mu      sync.Mutex
}

// NewXrayCollector 创建 Xray 流量采集器。
func NewXrayCollector(address string) (*XrayCollector, error) {
	if address == "" {
		address = "127.0.0.1:10085"
	}
	return &XrayCollector{
		address: address,
	}, nil
}

// SetClientForTest injects a mock client for testing.
// This bypasses the connect() logic.
func (c *XrayCollector) SetClientForTest(client statscommand.StatsServiceClient) {
	c.client = client
	// We need to set conn to non-nil to prevent connect() from overwriting client.
	// In tests we won't actually dial.
	c.conn = &grpc.ClientConn{}
}

// connect 建立与 Xray API 的 gRPC 连接。
func (c *XrayCollector) connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && c.client != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, c.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("connect to xray api at %s: %w", c.address, err)
	}

	c.conn = conn
	c.client = statscommand.NewStatsServiceClient(conn)
	slog.Debug("connected to xray stats api", "address", c.address)
	return nil
}

// Close 关闭 gRPC 连接。
func (c *XrayCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.client = nil
		return err
	}
	return nil
}

// Collect 从 Xray Stats API 拉取流量统计。
// 返回自上次采集以来的用户流量增量。
func (c *XrayCollector) Collect(ctx context.Context) ([]api.TrafficPayload, error) {
	if err := c.connect(ctx); err != nil {
		slog.Warn("failed to connect to xray stats api", "error", err)
		return nil, nil
	}

	// 查询所有用户流量统计
	// 统计名格式: "user>>>{email}>>>traffic>>>uplink" / "user>>>{email}>>>traffic>>>downlink"
	stats, err := c.queryStats(ctx, "user>>>", true)
	if err != nil {
		slog.Warn("failed to query xray stats", "error", err)
		// 连接可能已失效，下次重连
		c.Close()
		return nil, nil
	}

	// 按用户邮箱聚合流量
	userTraffic := make(map[string]*api.TrafficPayload)

	for name, value := range stats {
		// 解析统计名: "user>>>{email}>>>traffic>>>uplink" 或 "user>>>{email}>>>traffic>>>downlink"
		parts := strings.Split(name, ">>>")
		if len(parts) < 4 {
			continue
		}

		email := parts[1]
		direction := parts[3]

		if _, ok := userTraffic[email]; !ok {
			userTraffic[email] = &api.TrafficPayload{
				UID: email, // 暂时使用邮箱作为 UID
			}
		}

		switch direction {
		case "uplink":
			if value < 0 {
				value = 0
			}
			userTraffic[email].Upload = value
		case "downlink":
			if value < 0 {
				value = 0
			}
			userTraffic[email].Download = value
		}
	}

	// 转换为切片返回
	result := make([]api.TrafficPayload, 0, len(userTraffic))
	for _, t := range userTraffic {
		if t.Upload > 0 || t.Download > 0 {
			result = append(result, *t)
		}
	}

	slog.Debug("collected xray traffic stats", "users", len(result))
	return result, nil
}

// queryStats 查询匹配模式的 Xray 统计数据。
// reset 为 true 时读取后会重置计数。
func (c *XrayCollector) queryStats(ctx context.Context, pattern string, reset bool) (map[string]int64, error) {
	if c.client == nil {
		return nil, fmt.Errorf("xray stats client not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.QueryStats(ctx, &statscommand.QueryStatsRequest{
		Pattern: pattern,
		Reset_:  reset,
	})
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	result := make(map[string]int64, len(resp.Stat))
	for _, stat := range resp.Stat {
		result[stat.Name] = stat.Value
	}

	return result, nil
}

// GetSysStats 获取 Xray 系统统计信息。
func (c *XrayCollector) GetSysStats(ctx context.Context) (*XraySysStats, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	if c.client == nil {
		return nil, fmt.Errorf("xray stats client not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.GetSysStats(ctx, &statscommand.SysStatsRequest{})
	if err != nil {
		return nil, fmt.Errorf("get sys stats: %w", err)
	}

	return &XraySysStats{
		NumGoroutine: resp.NumGoroutine,
		NumGC:        resp.NumGC,
		Alloc:        resp.Alloc,
		TotalAlloc:   resp.TotalAlloc,
		Sys:          resp.Sys,
		Mallocs:      resp.Mallocs,
		Frees:        resp.Frees,
		LiveObjects:  resp.LiveObjects,
		PauseTotalNs: resp.PauseTotalNs,
		Uptime:       resp.Uptime,
	}, nil
}

// XraySysStats 描述 Xray 系统统计信息。
type XraySysStats struct {
	NumGoroutine uint32
	NumGC        uint32
	Alloc        uint64
	TotalAlloc   uint64
	Sys          uint64
	Mallocs      uint64
	Frees        uint64
	LiveObjects  uint64
	PauseTotalNs uint64
	Uptime       uint32
}

// GetOnlineUsers 获取 Xray 在线用户列表。
func (c *XrayCollector) GetOnlineUsers(ctx context.Context) ([]string, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	// 查询全部用户统计（不重置计数）以判断是否有流量
	stats, err := c.queryStats(ctx, "user>>>", false)
	if err != nil {
		return nil, err
	}

	// 收集唯一用户
	usersMap := make(map[string]struct{})
	for name := range stats {
		parts := strings.Split(name, ">>>")
		if len(parts) >= 2 {
			usersMap[parts[1]] = struct{}{}
		}
	}

	users := make([]string, 0, len(usersMap))
	for email := range usersMap {
		users = append(users, email)
	}

	return users, nil
}
