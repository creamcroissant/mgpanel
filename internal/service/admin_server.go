// 文件路径: internal/service/admin_server.go
// 模块说明: 这是 internal 模块里的 admin_server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminServerService 提供管理员节点/分组/路由列表接口。
type AdminServerService interface {
	Groups(ctx context.Context) ([]AdminServerGroupView, error)
	Routes(ctx context.Context) ([]AdminServerRouteView, error)
	Nodes(ctx context.Context) ([]AdminServerNodeView, error)
	SaveNode(ctx context.Context, input AdminServerNodeSaveInput) error
	DeleteNode(ctx context.Context, id int64) error
	I18n() *i18n.Manager
}

// AdminServerNodeSaveInput 定义保存节点的请求参数。
type AdminServerNodeSaveInput struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	GroupID    int64           `json:"group_id"`
	RouteID    int64           `json:"route_id"`
	ParentID   int64           `json:"parent_id"`
	Rate       string          `json:"rate"`
	Host       string          `json:"host"`
	Port       int             `json:"port"`
	ServerPort int             `json:"server_port"`
	Cipher     string          `json:"cipher"`
	Obfs       string          `json:"obfs"`
	Show       int             `json:"show"`
	Sort       int64           `json:"sort"`
	Status     int             `json:"status"`
	Type       string          `json:"type"`
	Tags       json.RawMessage `json:"tags"`
	Settings   json.RawMessage `json:"settings"`
}

// AdminServerGroupView 对齐管理端期望的服务器分组响应。
type AdminServerGroupView struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Sort      int64  `json:"sort"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// AdminServerRouteView 提供管理端展示所需的路由信息。
type AdminServerRouteView struct {
	ID          int64           `json:"id"`
	Remarks     string          `json:"remarks"`
	Match       json.RawMessage `json:"match"`
	Action      string          `json:"action"`
	ActionValue string          `json:"action_value"`
	CreatedAt   int64           `json:"created_at"`
	UpdatedAt   int64           `json:"updated_at"`
}

// AdminServerNodeView 表示 /server/manage/fetch 返回的节点数据。
type AdminServerNodeView struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	GroupID    int64           `json:"group_id"`
	RouteID    int64           `json:"route_id"`
	ParentID   int64           `json:"parent_id"`
	Rate       string          `json:"rate"`
	Host       string          `json:"host"`
	Port       int             `json:"port"`
	ServerPort int             `json:"server_port"`
	Cipher     string          `json:"cipher"`
	Obfs       string          `json:"obfs"`
	Show       int             `json:"show"`
	Sort       int64           `json:"sort"`
	Status     int             `json:"status"`
	Type       string          `json:"type"`
	Tags       json.RawMessage `json:"tags"`
	Settings   json.RawMessage `json:"settings"`
	CreatedAt  int64           `json:"created_at"`
	UpdatedAt  int64           `json:"updated_at"`
}

type adminServerService struct {
	groups  repository.ServerGroupRepository
	routes  repository.ServerRouteRepository
	servers repository.ServerRepository
	i18n    *i18n.Manager
}

// NewAdminServerService 组装管理端节点管理所需仓储。
func NewAdminServerService(groups repository.ServerGroupRepository, routes repository.ServerRouteRepository, servers repository.ServerRepository, i18nMgr *i18n.Manager) AdminServerService {
	return &adminServerService{groups: groups, routes: routes, servers: servers, i18n: i18nMgr}
}

func (s *adminServerService) I18n() *i18n.Manager {
	return s.i18n
}

func (s *adminServerService) Groups(ctx context.Context) ([]AdminServerGroupView, error) {
	if s == nil || s.groups == nil {
		return nil, fmt.Errorf("admin server service not configured / 管理节点服务未配置")
	}
	groups, err := s.groups.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AdminServerGroupView, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		views = append(views, AdminServerGroupView{
			ID:        group.ID,
			Name:      group.Name,
			Type:      group.Type,
			Sort:      group.Sort,
			CreatedAt: group.CreatedAt,
			UpdatedAt: group.UpdatedAt,
		})
	}
	return views, nil
}

func (s *adminServerService) Routes(ctx context.Context) ([]AdminServerRouteView, error) {
	if s == nil || s.routes == nil {
		return nil, fmt.Errorf("admin server service not configured / 管理节点服务未配置")
	}
	routes, err := s.routes.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AdminServerRouteView, 0, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		views = append(views, AdminServerRouteView{
			ID:          route.ID,
			Remarks:     route.Remarks,
			Match:       route.Match,
			Action:      route.Action,
			ActionValue: route.ActionValue,
			CreatedAt:   route.CreatedAt,
			UpdatedAt:   route.UpdatedAt,
		})
	}
	return views, nil
}

func (s *adminServerService) Nodes(ctx context.Context) ([]AdminServerNodeView, error) {
	if s == nil || s.servers == nil {
		return nil, fmt.Errorf("admin server service not configured / 管理节点服务未配置")
	}
	servers, err := s.servers.FindAllVisible(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AdminServerNodeView, 0, len(servers))
	for _, node := range servers {
		views = append(views, toAdminServerNodeView(node))
	}
	return views, nil
}

func (s *adminServerService) SaveNode(ctx context.Context, input AdminServerNodeSaveInput) error {
	if s == nil || s.servers == nil {
		return fmt.Errorf("admin server service not configured / 管理节点服务未配置")
	}
	
	server := &repository.Server{
		ID:         input.ID,
		Name:       input.Name,
		GroupID:    input.GroupID,
		RouteID:    input.RouteID,
		ParentID:   input.ParentID,
		Rate:       input.Rate,
		Host:       input.Host,
		Port:       input.Port,
		ServerPort: input.ServerPort,
		Cipher:     input.Cipher,
		Obfs:       input.Obfs,
		Show:       input.Show,
		Sort:       input.Sort,
		Status:     input.Status,
		Type:       input.Type,
		Tags:       input.Tags,
		Settings:   input.Settings,
	}

	if input.ID > 0 {
		return s.servers.Update(ctx, server)
	}
	return s.servers.Create(ctx, server)
}

func (s *adminServerService) DeleteNode(ctx context.Context, id int64) error {
	if s == nil || s.servers == nil {
		return fmt.Errorf("admin server service not configured / 管理节点服务未配置")
	}
	return s.servers.Delete(ctx, id)
}

func toAdminServerNodeView(node *repository.Server) AdminServerNodeView {
	if node == nil {
		return AdminServerNodeView{}
	}
	return AdminServerNodeView{
		ID:         node.ID,
		Name:       node.Name,
		GroupID:    node.GroupID,
		RouteID:    node.RouteID,
		ParentID:   node.ParentID,
		Rate:       node.Rate,
		Host:       node.Host,
		Port:       node.Port,
		ServerPort: node.ServerPort,
		Cipher:     node.Cipher,
		Obfs:       node.Obfs,
		Show:       node.Show,
		Sort:       node.Sort,
		Status:     node.Status,
		Type:       node.Type,
		Tags:       node.Tags,
		Settings:   node.Settings,
		CreatedAt:  node.CreatedAt,
		UpdatedAt:  node.UpdatedAt,
	}
}
