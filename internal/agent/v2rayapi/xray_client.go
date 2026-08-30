package v2rayapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	command "github.com/creamcroissant/mgpanel/pkg/pb/xray/app/proxyman/command"
	protocol "github.com/creamcroissant/mgpanel/pkg/pb/xray/common/protocol"
	serial "github.com/creamcroissant/mgpanel/pkg/pb/xray/common/serial"
	vless "github.com/creamcroissant/mgpanel/pkg/pb/xray/proxy/vless"
	vmess "github.com/creamcroissant/mgpanel/pkg/pb/xray/proxy/vmess"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// Xray HandlerService account type URLs (official Xray-core semantics).
const (
	xrayAccountTypeVLess = "xray.proxy.vless.Account"
	xrayAccountTypeVMess = "xray.proxy.vmess.Account"

	xrayOpTypeAddUser    = "xray.app.proxyman.command.AddUserOperation"
	xrayOpTypeRemoveUser = "xray.app.proxyman.command.RemoveUserOperation"
)

// xrayClient implements APIClient for Xray via its gRPC HandlerService
// (AlterInbound / GetInboundUsers / GetInboundUsersCount). It is the user
// identity is carried in the account TypedMessage using the official Xray
// vless/vmess account protos, so the same wire contract is used against both
// Xray and the sing-box fork's unified HandlerService.
type xrayClient struct {
	address string
	conn    *grpc.ClientConn
	client  command.HandlerServiceClient
	mu      sync.Mutex
}

// NewXrayClient returns a client for the given HandlerService gRPC address
// (e.g. "127.0.0.1:10085" or "unix:///var/run/xray.sock"). The connection is
// established lazily on the first call and reused afterwards.
func NewXrayClient(address string) *xrayClient {
	return &xrayClient{address: address}
}

// newXrayClientWithConn wires an existing connection (used by tests and by
// consumers that already own a dialed conn).
func newXrayClientWithConn(conn *grpc.ClientConn) *xrayClient {
	return &xrayClient{conn: conn, client: command.NewHandlerServiceClient(conn)}
}

// Address returns the configured gRPC address.
func (c *xrayClient) Address() string {
	return c.address
}

// Close closes the underlying gRPC connection if open.
func (c *xrayClient) Close() error {
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

// connect establishes the gRPC connection lazily.
func (c *xrayClient) connect(ctx context.Context) error {
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
		return fmt.Errorf("connect to xray handler service at %s: %w", c.address, err)
	}
	c.conn = conn
	c.client = command.NewHandlerServiceClient(conn)
	return nil
}

// ListUsers returns the users attached to the inbound identified by inboundTag.
func (c *xrayClient) ListUsers(ctx context.Context, inboundTag string) ([]UserCredential, error) {
	if err := c.connect(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := c.client.GetInboundUsers(ctx, &command.GetInboundUserRequest{Tag: inboundTag})
	if err != nil {
		return nil, fmt.Errorf("list users inbound=%s: %w", inboundTag, err)
	}
	users := make([]UserCredential, 0, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		uc, err := xrayUserFromProto(u)
		if err != nil {
			return nil, err
		}
		users = append(users, uc)
	}
	return users, nil
}

// AddUser attaches a user to the inbound identified by inboundTag.
func (c *xrayClient) AddUser(ctx context.Context, inboundTag string, u UserCredential) error {
	if err := c.connect(ctx); err != nil {
		return err
	}
	accVal, err := proto.Marshal(&vless.Account{Id: u.UUID, Flow: u.Flow})
	if err != nil {
		return fmt.Errorf("marshal account: %w", err)
	}
	op := &command.AddUserOperation{
		User: &protocol.User{
			Email: u.Email,
			Account: &serial.TypedMessage{
				Type:  xrayAccountTypeVLess,
				Value: accVal,
			},
		},
	}
	return c.alterInbound(ctx, inboundTag, xrayOpTypeAddUser, op)
}

// RemoveUser detaches the user identified by email from inboundTag.
func (c *xrayClient) RemoveUser(ctx context.Context, inboundTag, email string) error {
	if err := c.connect(ctx); err != nil {
		return err
	}
	return c.alterInbound(ctx, inboundTag, xrayOpTypeRemoveUser, &command.RemoveUserOperation{Email: email})
}

// alterInbound wraps an operation message in AlterInboundRequest and sends it.
func (c *xrayClient) alterInbound(ctx context.Context, inboundTag, opType string, op proto.Message) error {
	opVal, err := proto.Marshal(op)
	if err != nil {
		return fmt.Errorf("marshal operation %s: %w", opType, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = c.client.AlterInbound(ctx, &command.AlterInboundRequest{
		Tag: inboundTag,
		Operation: &serial.TypedMessage{
			Type:  opType,
			Value: opVal,
		},
	})
	if err != nil {
		return fmt.Errorf("alter inbound=%s op=%s: %w", inboundTag, opType, err)
	}
	return nil
}

// xrayUserFromProto converts a protocol.User (with account TypedMessage) into
// a UserCredential. Supported accounts: vless (id/flow) and vmess (id).
// Inbounds without an account (password-style) degrade to email-only.
func xrayUserFromProto(u *protocol.User) (UserCredential, error) {
	if u == nil {
		return UserCredential{}, nil
	}
	acc := u.GetAccount()
	if acc == nil {
		return UserCredential{Email: u.GetEmail()}, nil
	}
	switch trimTypePrefix(acc.GetType()) {
	case xrayAccountTypeVLess:
		var a vless.Account
		if err := proto.Unmarshal(acc.GetValue(), &a); err != nil {
			return UserCredential{}, fmt.Errorf("decode vless account: %w", err)
		}
		return UserCredential{Email: u.GetEmail(), UUID: a.GetId(), Flow: a.GetFlow()}, nil
	case xrayAccountTypeVMess:
		var a vmess.Account
		if err := proto.Unmarshal(acc.GetValue(), &a); err != nil {
			return UserCredential{}, fmt.Errorf("decode vmess account: %w", err)
		}
		return UserCredential{Email: u.GetEmail(), UUID: a.GetId()}, nil
	default:
		return UserCredential{Email: u.GetEmail()}, nil
	}
}
