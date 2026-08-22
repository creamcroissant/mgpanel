package service

import (
	"context"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// AdminNodeStatService 提供管理端节点统计能力。
type AdminNodeStatService interface {
	// GetServerStats 返回单节点历史统计。
	GetServerStats(ctx context.Context, serverID int64, recordType int, days int) ([]repository.StatServerRecord, error)
	// GetTotalTraffic 返回区间内全节点流量汇总。
	GetTotalTraffic(ctx context.Context, recordType int, startAt, endAt int64) (repository.StatServerSumResult, error)
	// GetTopServers 返回按流量排序的节点列表。
	GetTopServers(ctx context.Context, recordType int, startAt, endAt int64, limit int) ([]repository.StatServerAggregate, error)
}

// adminNodeStatService 是 AdminNodeStatService 的实现。
type adminNodeStatService struct {
	statServers repository.StatServerRepository
}

// NewAdminNodeStatService 创建管理端节点统计服务。
func NewAdminNodeStatService(statServers repository.StatServerRepository) AdminNodeStatService {
	return &adminNodeStatService{statServers: statServers}
}

// GetServerStats 返回指定节点的统计数据。
func (s *adminNodeStatService) GetServerStats(ctx context.Context, serverID int64, recordType int, days int) ([]repository.StatServerRecord, error) {
	// 默认查询 30 天，并根据统计类型调整记录上限。
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days).Unix()
	limit := days * 24 // 每小时记录数量上限
	if recordType == 1 {
		limit = days // 日统计记录数量
	}
	return s.statServers.ListByServer(ctx, serverID, recordType, since, limit)
}

func (s *adminNodeStatService) GetTotalTraffic(ctx context.Context, recordType int, startAt, endAt int64) (repository.StatServerSumResult, error) {
	filter := repository.StatServerSumFilter{
		RecordType: recordType,
		StartAt:    startAt,
		EndAt:      endAt,
	}
	return s.statServers.SumByRange(ctx, filter)
}

func (s *adminNodeStatService) GetTopServers(ctx context.Context, recordType int, startAt, endAt int64, limit int) ([]repository.StatServerAggregate, error) {
	filter := repository.StatServerTopFilter{
		RecordType: recordType,
		StartAt:    startAt,
		EndAt:      endAt,
		Limit:      limit,
	}
	return s.statServers.TopByRange(ctx, filter)
}
