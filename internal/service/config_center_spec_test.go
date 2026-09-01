package service

import (
	"context"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type mockInboundSpecRepo struct {
	items        map[int64]*repository.InboundSpec
	nextID       int64
	listErr      error
	createErr    error
	updateErr    error
	countErr     error
	findByTagErr error
}

func newMockInboundSpecRepo() *mockInboundSpecRepo {
	return &mockInboundSpecRepo{
		items:  make(map[int64]*repository.InboundSpec),
		nextID: 1,
	}
}

func (m *mockInboundSpecRepo) Create(ctx context.Context, spec *repository.InboundSpec) error {
	if m.createErr != nil {
		return m.createErr
	}
	if spec.ID == 0 {
		spec.ID = m.nextID
		m.nextID++
	}
	clone := *spec
	m.items[clone.ID] = &clone
	return nil
}

func (m *mockInboundSpecRepo) Update(ctx context.Context, spec *repository.InboundSpec) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.items[spec.ID]; !ok {
		return repository.ErrNotFound
	}
	clone := *spec
	m.items[clone.ID] = &clone
	return nil
}

func (m *mockInboundSpecRepo) UpdateWithRevision(ctx context.Context, spec *repository.InboundSpec, expectedRevision int64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	stored, ok := m.items[spec.ID]
	if !ok {
		return repository.ErrNotFound
	}
	if stored.DesiredRevision != expectedRevision {
		return repository.ErrConflict
	}
	clone := *spec
	m.items[clone.ID] = &clone
	return nil
}

func (m *mockInboundSpecRepo) Delete(ctx context.Context, id int64) error {
	delete(m.items, id)
	return nil
}

func (m *mockInboundSpecRepo) FindByID(ctx context.Context, id int64) (*repository.InboundSpec, error) {
	if item, ok := m.items[id]; ok {
		clone := *item
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockInboundSpecRepo) FindByHostCoreTag(ctx context.Context, agentHostID int64, coreType, tag string) (*repository.InboundSpec, error) {
	if m.findByTagErr != nil {
		return nil, m.findByTagErr
	}
	for _, item := range m.items {
		if item.AgentHostID != nil && *item.AgentHostID == agentHostID && item.CoreType == coreType && item.Tag == tag {
			clone := *item
			return &clone, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockInboundSpecRepo) FindByCoreTag(ctx context.Context, coreType, tag string) (*repository.InboundSpec, error) {
	for _, item := range m.items {
		if item.AgentHostID == nil && item.CoreType == coreType && item.Tag == tag {
			clone := *item
			return &clone, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockInboundSpecRepo) ListByAgentHost(ctx context.Context, agentHostID int64, filter repository.InboundSpecFilter) ([]*repository.InboundSpec, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.InboundSpec, 0)
	for _, item := range m.items {
		if item.AgentHostID != nil && *item.AgentHostID == agentHostID {
		} else if item.AgentHostID == nil {
		} else {
			continue
		}
		if filter.CoreType != nil && item.CoreType != *filter.CoreType {
			continue
		}
		if filter.Tag != nil && item.Tag != *filter.Tag {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		clone := *item
		result = append(result, &clone)
	}
	return result, nil
}

func (m *mockInboundSpecRepo) CountByAgentHost(ctx context.Context, agentHostID int64, filter repository.InboundSpecFilter) (int64, error) {
	items, err := m.ListByAgentHost(ctx, agentHostID, filter)
	if err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

func (m *mockInboundSpecRepo) List(ctx context.Context, filter repository.InboundSpecFilter) ([]*repository.InboundSpec, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.InboundSpec, 0)
	for _, item := range m.items {
		if filter.AgentHostID != nil {
			if item.AgentHostID == nil || *item.AgentHostID != *filter.AgentHostID {
				continue
			}
		}
		if filter.IsTemplate != nil && *filter.IsTemplate {
			if item.AgentHostID != nil {
				continue
			}
		}
		if filter.CoreType != nil && item.CoreType != *filter.CoreType {
			continue
		}
		if filter.Tag != nil && item.Tag != *filter.Tag {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		clone := *item
		result = append(result, &clone)
	}
	return result, nil
}

func (m *mockInboundSpecRepo) Count(ctx context.Context, filter repository.InboundSpecFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	items, err := m.List(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}
