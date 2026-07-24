package service

import (
	"context"
	"encoding/json"
	"log/slog"

	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

// PushRoutingTables iterates all mesh peers and serializes their cached
// routing tables into SetRoutingTablesPayload for command dispatch.
func (s *agentMeshService) PushRoutingTables(ctx context.Context) error {
	s.peerLatMu.RLock()
	defer s.peerLatMu.RUnlock()

	peers, err := s.peers.ListByNetworkID(ctx, "default")
	if err != nil {
		return err
	}

	for _, p := range peers {
		routes, ok := s.router.GetRoutes(p.AgentHostID)
		if !ok || len(routes) == 0 {
			continue
		}
		pbRoutes := make([]*agentv1.RouteEntry, 0, len(routes))
		for _, r := range routes {
			pbRoutes = append(pbRoutes, &agentv1.RouteEntry{
				PeerId:     r.PeerID,
				Priority:   int32(r.Priority),
				PeerWgIp:   r.PeerWGIP,
				PeerPort:   int32(r.PeerPort),
				LatencyMs:  r.LatencyMs,
				PacketLoss: r.PacketLoss,
			})
		}
		payload := &agentv1.SetRoutingTablesPayload{Routes: pbRoutes}
		data, err := json.Marshal(payload)
		if err != nil {
			slog.Warn("failed to marshal routing table", "agent_host_id", p.AgentHostID, "error", err)
			continue
		}

		_, err = s.lifecycleOps.Create(ctx, CreateAgentLifecycleOperationRequest{
			AgentHostID:    p.AgentHostID,
			OperationType:  "set_routing_table",
			RequestPayload: json.RawMessage(data),
			Source:         "system",
		})
		if err != nil {
			slog.Warn("route push: create operation failed", "agent_host_id", p.AgentHostID, "error", err)
			continue
		}
		slog.Info("route push: created set_routing_table command",
			"agent_host_id", p.AgentHostID, "routes", len(routes), "payload_bytes", len(data))
	}
	return nil
}
