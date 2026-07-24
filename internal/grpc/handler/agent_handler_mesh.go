package handler

import (
	"context"
	"strconv"

	"github.com/creamcroissant/xboard/internal/grpc/interceptor"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JoinMesh handles agent request to join the WireGuard mesh network.
func (h *AgentHandler) JoinMesh(ctx context.Context, req *agentv1.JoinMeshRequest) (*agentv1.JoinMeshResponse, error) {
	agentHost, ok := interceptor.GetAgentHostFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no agent host in context")
	}
	if h.meshService == nil {
		return nil, status.Error(codes.FailedPrecondition, "mesh service is not available")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	networkID := req.GetNetworkId()
	if networkID == "" {
		networkID = "default"
	}

	ip, pub, err := h.meshService.JoinNetwork(ctx, agentHost.ID, networkID)
	if err != nil {
		h.logger.Error("failed to join mesh network", "agent_host_id", agentHost.ID, "error", err)
		return nil, status.Error(codes.Internal, "failed to join mesh network")
	}

	// Fetch full peer record to get private key and listen port
	peer, err := h.meshService.GetMeshPeer(ctx, agentHost.ID)
	if err != nil || peer == nil {
		return nil, status.Error(codes.Internal, "peer not found after join")
	}

	return &agentv1.JoinMeshResponse{
		Success:       true,
		WgPrivateKey:  peer.WGPrivateKey,
		WgPublicKey:   pub,
		WgIp:          ip,
		WgListenPort:  int32(peer.WGListenPort),
	}, nil
}

// GetMeshPeers returns all mesh peers for the agent's network.
func (h *AgentHandler) GetMeshPeers(ctx context.Context, req *agentv1.GetMeshPeersRequest) (*agentv1.GetMeshPeersResponse, error) {
	agentHost, ok := interceptor.GetAgentHostFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "no agent host in context")
	}
	if h.meshService == nil {
		return nil, status.Error(codes.FailedPrecondition, "mesh service is not available")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	networkID := req.GetNetworkId()
	if networkID == "" {
		networkID = "default"
	}

	peers, err := h.meshService.ListNetworkPeers(ctx, networkID)
	if err != nil {
		h.logger.Error("failed to list mesh peers", "agent_host_id", agentHost.ID, "error", err)
		return nil, status.Error(codes.Internal, "failed to list mesh peers")
	}

	var pbPeers []*agentv1.MeshPeerEntry
	for _, p := range peers {
		if p.AgentHostID == agentHost.ID {
			continue // skip self
		}
		// Resolve endpoint from agent host address
		endpoint := ""
		host, err := h.agentHostService.GetByID(ctx, p.AgentHostID)
		if err == nil && host != nil && host.Host != "" {
			endpoint = host.Host + ":" + strconv.Itoa(p.WGListenPort)
		}
		pbPeers = append(pbPeers, &agentv1.MeshPeerEntry{
			Id:         "agent-" + strconv.FormatInt(p.AgentHostID, 10),
			PublicKey:  p.WGPublicKey,
			Endpoint:   endpoint,
			AllowedIps: []string{p.WGIP + "/32"},
			Keepalive:  25,
		})
	}
	if pbPeers == nil {
		pbPeers = []*agentv1.MeshPeerEntry{}
	}

	return &agentv1.GetMeshPeersResponse{
		Success: true,
		Peers:   pbPeers,
	}, nil
}
