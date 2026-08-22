package monitor

import (
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/api"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type SystemStatFetcher struct {
	CPUPercent    func(interval time.Duration, percpu bool) ([]float64, error)
	VirtualMemory func() (*mem.VirtualMemoryStat, error)
	SwapMemory    func() (*mem.SwapMemoryStat, error)
	DiskUsage     func(path string) (*disk.UsageStat, error)
	LoadAvg       func() (*load.AvgStat, error)
	HostUptime    func() (uint64, error)
	NetIOCounters func(pernic bool) ([]net.IOCountersStat, error)
	ProcessPids   func() ([]int32, error)
}

type Monitor struct {
	fetcher     SystemStatFetcher
	lastNetStat []net.IOCountersStat
	lastTime    time.Time
}

func New() *Monitor {
	return &Monitor{
		fetcher: SystemStatFetcher{
			CPUPercent:    cpu.Percent,
			VirtualMemory: mem.VirtualMemory,
			SwapMemory:    mem.SwapMemory,
			DiskUsage:     disk.Usage,
			LoadAvg:       load.Avg,
			HostUptime:    host.Uptime,
			NetIOCounters: net.IOCounters,
			ProcessPids:   process.Pids,
		},
		lastTime: time.Now(),
	}
}

// SetFetcher sets a custom fetcher for testing.
func (m *Monitor) SetFetcher(fetcher SystemStatFetcher) {
	m.fetcher = fetcher
}

func (m *Monitor) Collect() (api.StatusPayload, error) {
	stat := api.StatusPayload{}

	// CPU
	if percents, err := m.fetcher.CPUPercent(0, false); err == nil && len(percents) > 0 {
		stat.CPU = percents[0]
	}

	// Memory
	if v, err := m.fetcher.VirtualMemory(); err == nil {
		stat.Mem = api.Stats{
			Total: v.Total,
			Used:  v.Used,
		}
	}

	// Swap
	if v, err := m.fetcher.SwapMemory(); err == nil {
		stat.Swap = api.Stats{
			Total: v.Total,
			Used:  v.Used,
		}
	}

	// Disk (Root)
	if d, err := m.fetcher.DiskUsage("/"); err == nil {
		stat.Disk = api.Stats{
			Total: d.Total,
			Used:  d.Used,
		}
	}

	// Load
	if l, err := m.fetcher.LoadAvg(); err == nil {
		stat.Load1 = l.Load1
		stat.Load5 = l.Load5
		stat.Load15 = l.Load15
	}

	// Uptime
	if u, err := m.fetcher.HostUptime(); err == nil {
		stat.Uptime = u
	}

	// NetIO Speed
	now := time.Now()
	if counters, err := m.fetcher.NetIOCounters(false); err == nil && len(counters) > 0 {
		current := counters[0]
		if len(m.lastNetStat) > 0 {
			last := m.lastNetStat[0]
			duration := now.Sub(m.lastTime).Seconds()
			if duration <= 0 {
				// 时钟回退或采样间隔异常，跳过速率计算
				m.lastNetStat = counters
				m.lastTime = now
			} else {
				stat.NetIO.Up = uint64(float64(current.BytesSent-last.BytesSent) / duration)
				stat.NetIO.Down = uint64(float64(current.BytesRecv-last.BytesRecv) / duration)
			}
		}
		m.lastNetStat = counters
	}
	m.lastTime = now

	// Process Count
	if pids, err := m.fetcher.ProcessPids(); err == nil {
		stat.ProcessCount = len(pids)
	}

	// TODO: TCP/UDP Count (requires traversing /proc/net/tcp or similar)

	return stat, nil
}
