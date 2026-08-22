// 文件路径: internal/job/stat_user_accumulator.go
// 模块说明: 流量累加器，收集流量增量并支持多节点聚合
package job

import "sync"

// StatUserKey 表示流量累加的唯一键（AgentHostID + UserID）。
type StatUserKey struct {
	AgentHostID int64
	UserID      int64
}

// StatUserDelta 表示用户流量增量。
type StatUserDelta struct {
	Upload   int64
	Download int64
}

// StatUserAccumulator 收集流量增量并等待写入存储。
type StatUserAccumulator struct {
	mu     sync.Mutex
	totals map[StatUserKey]StatUserDelta
}

// NewStatUserAccumulator 创建空的累加器。
func NewStatUserAccumulator() *StatUserAccumulator {
	return &StatUserAccumulator{totals: make(map[StatUserKey]StatUserDelta)}
}

// Collect 满足 service.TrafficStatCollector（兼容旧接口，agentHostID=0）。
func (a *StatUserAccumulator) Collect(userID int64, uploadDelta, downloadDelta int64) {
	a.CollectWithHost(0, userID, uploadDelta, downloadDelta)
}

// CollectWithHost 收集带有节点维度的流量增量。
func (a *StatUserAccumulator) CollectWithHost(agentHostID, userID int64, uploadDelta, downloadDelta int64) {
	if userID <= 0 {
		return
	}
	if uploadDelta == 0 && downloadDelta == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := StatUserKey{AgentHostID: agentHostID, UserID: userID}
	delta := a.totals[key]
	delta.Upload += uploadDelta
	delta.Download += downloadDelta
	a.totals[key] = delta
}

// Flush 返回当前累计结果并清空累加器。
func (a *StatUserAccumulator) Flush() map[StatUserKey]StatUserDelta {
	a.mu.Lock()
	defer a.mu.Unlock()
	snapshot := a.totals
	a.totals = make(map[StatUserKey]StatUserDelta)
	return snapshot
}

// Merge 将一批增量合并回累加器。
func (a *StatUserAccumulator) Merge(pending map[StatUserKey]StatUserDelta) {
	if len(pending) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, delta := range pending {
		if delta.Upload == 0 && delta.Download == 0 {
			continue
		}
		existing := a.totals[key]
		existing.Upload += delta.Upload
		existing.Download += delta.Download
		a.totals[key] = existing
	}
}

// Pending 返回当前累计的条目数量。
func (a *StatUserAccumulator) Pending() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.totals)
}

// MultiAccumulator 将流量增量广播到多个累加器。
// 用于支持多粒度（小时/日/月）的统计。
type MultiAccumulator struct {
	accumulators []*StatUserAccumulator
}

// NewMultiAccumulator 创建指定数量的累加器集合。
func NewMultiAccumulator(count int) *MultiAccumulator {
	accumulators := make([]*StatUserAccumulator, count)
	for i := range accumulators {
		accumulators[i] = NewStatUserAccumulator()
	}
	return &MultiAccumulator{accumulators: accumulators}
}

// Get 返回指定索引的累加器。
func (m *MultiAccumulator) Get(index int) *StatUserAccumulator {
	if index < 0 || index >= len(m.accumulators) {
		return nil
	}
	return m.accumulators[index]
}

// Collect 向所有累加器广播（兼容旧接口，agentHostID=0）。
func (m *MultiAccumulator) Collect(userID int64, uploadDelta, downloadDelta int64) {
	m.CollectWithHost(0, userID, uploadDelta, downloadDelta)
}

// CollectWithHost 向所有累加器广播流量增量。
func (m *MultiAccumulator) CollectWithHost(agentHostID, userID int64, uploadDelta, downloadDelta int64) {
	for _, acc := range m.accumulators {
		acc.CollectWithHost(agentHostID, userID, uploadDelta, downloadDelta)
	}
}
