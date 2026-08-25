// 文件路径: internal/service/server_traffic.go
// 模块说明: 这是 internal 模块里的 server_traffic 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ServerTrafficService 负责持久化节点上报的流量增量。
type ServerTrafficService interface {
	Apply(ctx context.Context, server *repository.Server, samples []UniProxyPushSample) error
}

// serverTrafficService 组合用户仓储与统计收集器。
type serverTrafficService struct {
	users     repository.UserRepository
	collector TrafficStatCollector
}

// TrafficStatCollector 收集原始流量增量以便后续聚合。
type TrafficStatCollector interface {
	Collect(userID int64, uploadDelta, downloadDelta int64)
}

// NewServerTrafficService 组装流量服务依赖。
func NewServerTrafficService(userRepo repository.UserRepository, collector TrafficStatCollector) ServerTrafficService {
	return &serverTrafficService{users: userRepo, collector: collector}
}

// Apply 处理节点上报样本，并按倍率累加到用户流量。
func (s *serverTrafficService) Apply(ctx context.Context, server *repository.Server, samples []UniProxyPushSample) error {
	if err := ensureServer(server); err != nil {
		return err
	}
	if len(samples) == 0 {
		return nil
	}
	if s.users == nil {
		return errors.New("server traffic: user repository unavailable / 节点流量用户仓储不可用")
	}
	rate := parseServerRate(server)
	deltas := aggregateTraffic(samples, rate)
	// 批量收集增量，先在内存中过滤掉零值
	batchDeltas := make(map[int64][2]int64, len(deltas))
	for userID, delta := range deltas {
		if delta.Upload == 0 && delta.Download == 0 {
			continue
		}
		batchDeltas[userID] = [2]int64{delta.Upload, delta.Download}
	}
	if len(batchDeltas) > 0 {
		// 单事务批量写入（类型断言，sqlite 实现支持批量）
		if batchRepo, ok := s.users.(interface{ IncrementTrafficBatch(ctx context.Context, deltas map[int64][2]int64) error }); ok {
			if err := batchRepo.IncrementTrafficBatch(ctx, batchDeltas); err != nil {
				return err
			}
		} else {
			// 回退到逐条更新（测试 mock 等不支持批量）
			for userID, delta := range batchDeltas {
				if err := s.users.IncrementTraffic(ctx, userID, delta[0], delta[1]); err != nil {
					if errors.Is(err, repository.ErrNotFound) {
						continue
					}
					return err
				}
			}
		}
		// 将增量交给统计收集器做后续聚合
		for userID, delta := range deltas {
			if delta.Upload == 0 && delta.Download == 0 {
				continue
			}
			s.collect(userID, delta.Upload, delta.Download)
		}
	}
	return nil
}

// collect 将增量传递给统计收集器（若存在）。
func (s *serverTrafficService) collect(userID int64, uploadDelta, downloadDelta int64) {
	if s == nil || s.collector == nil {
		return
	}
	s.collector.Collect(userID, uploadDelta, downloadDelta)
}

// trafficDelta 表示单个用户的上传/下载增量。
type trafficDelta struct {
	Upload   int64
	Download int64
}

// aggregateTraffic 将样本按用户聚合，并应用倍率缩放。
func aggregateTraffic(samples []UniProxyPushSample, rate float64) map[int64]trafficDelta {
	totals := make(map[int64]trafficDelta, len(samples))
	for _, sample := range samples {
		if sample.UserID <= 0 {
			continue
		}
		upload := scaleTraffic(sample.Upload, rate)
		download := scaleTraffic(sample.Download, rate)
		if upload == 0 && download == 0 {
			continue
		}
		delta := totals[sample.UserID]
		delta.Upload += upload
		delta.Download += download
		totals[sample.UserID] = delta
	}
	return totals
}

// scaleTraffic 根据倍率对流量进行四舍五入。
func scaleTraffic(value int64, rate float64) int64 {
	if value <= 0 {
		return 0
	}
	scaled := float64(value) * rate
	return int64(math.Round(scaled))
}

// parseServerRate 读取并解析节点倍率，非法/空值则回退为 1。
func parseServerRate(server *repository.Server) float64 {
	if server == nil {
		return 1
	}
	rate := strings.TrimSpace(server.Rate)
	if rate == "" {
		return 1
	}
	if parsed, err := strconv.ParseFloat(rate, 64); err == nil {
		if parsed > 0 {
			return parsed
		}
	}
	return 1
}
