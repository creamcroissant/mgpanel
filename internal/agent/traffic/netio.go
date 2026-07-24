package traffic

import (
	"context"
	"log/slog"
	"sync"

	"github.com/shirou/gopsutil/v3/net"
)

// NetIOCountersFetcher defines a function signature for fetching network IO counters.
type NetIOCountersFetcher func(ctx context.Context, pernic bool) ([]net.IOCountersStat, error)

// NetIOCollector 使用 gopsutil 采集节点网络流量。
// 记录累计字节数，并计算每次采集的增量。
type NetIOCollector struct {
	iface       string
	fetcher     NetIOCountersFetcher
	mu          sync.Mutex
	lastSent    uint64
	lastRecv    uint64
	initialized bool
}

// NewNetIOCollector 创建网络流量采集器。
// 如果 iface 为空则汇总全部网卡流量。
func NewNetIOCollector(iface string) *NetIOCollector {
	return &NetIOCollector{
		iface:   iface,
		fetcher: net.IOCountersWithContext,
	}
}

// SetFetcher sets a custom fetcher for testing purposes.
func (c *NetIOCollector) SetFetcher(fetcher NetIOCountersFetcher) {
	c.fetcher = fetcher
}

// CollectCumulative 返回当前累计流量的原始字节数（不计算增量）。
func (c *NetIOCollector) CollectCumulative(ctx context.Context) (uploadTotal, downloadTotal uint64, err error) {
	var totalSent, totalRecv uint64

	if c.iface != "" {
		counters, err := c.fetcher(ctx, true)
		if err != nil {
			return 0, 0, err
		}
		for _, counter := range counters {
			if counter.Name == c.iface {
				totalSent = counter.BytesSent
				totalRecv = counter.BytesRecv
				break
			}
		}
	} else {
		counters, err := c.fetcher(ctx, false)
		if err != nil {
			return 0, 0, err
		}
		if len(counters) > 0 {
			totalSent = counters[0].BytesSent
			totalRecv = counters[0].BytesRecv
		}
	}
	return totalSent, totalRecv, nil
}

// NetIODelta 描述相邻两次采集的流量增量。
type NetIODelta struct {
	Upload   uint64 // 上次采集以来的上传字节数
	Download uint64 // 上次采集以来的下载字节数
}

// CollectDelta 返回自上次采集以来的流量增量。
func (c *NetIOCollector) CollectDelta(ctx context.Context) (*NetIODelta, error) {
	var totalSent, totalRecv uint64

	if c.iface != "" {
		// 采集指定网卡的流量
		counters, err := c.fetcher(ctx, true)
		if err != nil {
			return nil, err
		}

		for _, counter := range counters {
			if counter.Name == c.iface {
				totalSent = counter.BytesSent
				totalRecv = counter.BytesRecv
				break
			}
		}
	} else {
		// 汇总所有网卡的流量
		counters, err := c.fetcher(ctx, false)
		if err != nil {
			return nil, err
		}

		if len(counters) > 0 {
			totalSent = counters[0].BytesSent
			totalRecv = counters[0].BytesRecv
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		c.lastSent = totalSent
		c.lastRecv = totalRecv
		c.initialized = true
		slog.Debug("NetIOCollector initialized",
			"interface", c.iface,
			"initial_sent", totalSent,
			"initial_recv", totalRecv)
		return &NetIODelta{Upload: 0, Download: 0}, nil
	}

	// 处理计数器回绕，确保增量不为负
	var uploadDelta, downloadDelta uint64
	if totalSent >= c.lastSent {
		uploadDelta = totalSent - c.lastSent
	} else {
		slog.Warn("network counter wrapped or reset",
			"interface", c.iface,
			"last_sent", c.lastSent,
			"current_sent", totalSent)
		uploadDelta = 0
	}
	if totalRecv >= c.lastRecv {
		downloadDelta = totalRecv - c.lastRecv
	} else {
		slog.Warn("network counter wrapped or reset",
			"interface", c.iface,
			"last_recv", c.lastRecv,
			"current_recv", totalRecv)
		downloadDelta = 0
	}

	c.lastSent = totalSent
	c.lastRecv = totalRecv

	slog.Debug("NetIO traffic delta collected",
		"upload", uploadDelta,
		"download", downloadDelta)

	return &NetIODelta{
		Upload:   uploadDelta,
		Download: downloadDelta,
	}, nil
}
