package v2rayapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	command "github.com/creamcroissant/mgpanel/pkg/pb/xray/app/proxyman/command"
	protocol "github.com/creamcroissant/mgpanel/pkg/pb/xray/common/protocol"
	serial "github.com/creamcroissant/mgpanel/pkg/pb/xray/common/serial"
	vless "github.com/creamcroissant/mgpanel/pkg/pb/xray/proxy/vless"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// Account type URL used by the sing-box fork's unified HandlerService. The
// payload is a protowire-encoded {name=1,uuid=2,password=3,flow=4} message
// covering every protocol, in addition to the official vless/vmess accounts
// that the fork also accepts.
const mgpanelManagedUserAccount = "mgpanel.user.ManagedUser"

// singboxClient implements APIClient for the sing-box fork (creamcroissant/
// sing-box_with_api) via its unified gRPC HandlerService — the same wire
// contract as Xray, with the account payload carried as
// mgpanel.user.ManagedUser.
type singboxClient struct {
	address string
	conn    *grpc.ClientConn
	client  command.HandlerServiceClient
	mu      sync.Mutex
}

// NewSingboxClient returns a client for the given HandlerService gRPC address
// (the sing-box v2ray_api listen address, e.g. "127.0.0.1:19194"). The
// connection is established lazily on the first call.
func NewSingboxClient(address string) *singboxClient {
	return &singboxClient{address: address}
}

// newSingboxClientWithConn wires an existing connection (tests / own-conn).
func newSingboxClientWithConn(conn *grpc.ClientConn) *singboxClient {
	return &singboxClient{conn: conn, client: command.NewHandlerServiceClient(conn)}
}

// Address returns the configured gRPC address.
func (c *singboxClient) Address() string {
	return c.address
}

// Close closes the underlying gRPC connection if open.
func (c *singboxClient) Close() error {
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

func (c *singboxClient) connect(ctx context.Context) error {
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
		return fmt.Errorf("connect to sing-box handler service at %s: %w", c.address, err)
	}
	c.conn = conn
	c.client = command.NewHandlerServiceClient(conn)
	return nil
}

// ListUsers returns the users attached to the inbound identified by inboundTag.
func (c *singboxClient) ListUsers(ctx context.Context, inboundTag string) ([]UserCredential, error) {
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
		uc, err := singboxUserFromProto(u)
		if err != nil {
			return nil, err
		}
		users = append(users, uc)
	}
	return users, nil
}

// AddUser attaches a user to the inbound identified by inboundTag.
func (c *singboxClient) AddUser(ctx context.Context, inboundTag string, u UserCredential) error {
	if err := c.connect(ctx); err != nil {
		return err
	}
	op := &command.AddUserOperation{
		User: &protocol.User{
			Email: u.Email,
			Account: &serial.TypedMessage{
				Type:  mgpanelManagedUserAccount,
				Value: encodeManagedUser(u),
			},
		},
	}
	return c.alterInbound(ctx, inboundTag, xrayOpTypeAddUser, op)
}

// RemoveUser detaches the user identified by email from inboundTag.
func (c *singboxClient) RemoveUser(ctx context.Context, inboundTag, email string) error {
	if err := c.connect(ctx); err != nil {
		return err
	}
	return c.alterInbound(ctx, inboundTag, xrayOpTypeRemoveUser, &command.RemoveUserOperation{Email: email})
}

func (c *singboxClient) alterInbound(ctx context.Context, inboundTag, opType string, op proto.Message) error {
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

// singboxUserFromProto converts a protocol.User into a UserCredential. The
// fork's UserService accepts mgpanel.user.ManagedUser (protowire), and for
// interop also the official vless/vmess accounts. Inbounds without an account
// degrade to email-only.
func singboxUserFromProto(u *protocol.User) (UserCredential, error) {
	if u == nil {
		return UserCredential{}, nil
	}
	acc := u.GetAccount()
	if acc == nil {
		return UserCredential{Email: u.GetEmail()}, nil
	}
	typ := trimTypePrefix(acc.GetType())
	switch typ {
	case mgpanelManagedUserAccount:
		return decodeManagedUser(acc.GetValue(), u.GetEmail())
	case xrayAccountTypeVLess:
		var a vless.Account
		if err := proto.Unmarshal(acc.GetValue(), &a); err != nil {
			return UserCredential{}, fmt.Errorf("decode vless account: %w", err)
		}
		return UserCredential{Email: u.GetEmail(), UUID: a.GetId(), Flow: a.GetFlow()}, nil
	default:
		return UserCredential{Email: u.GetEmail()}, nil
	}
}

// trimTypePrefix strips a leading "type.googleapis.com/" prefix if present so
// that both raw and full type URLs normalize to the same constant.
func trimTypePrefix(t string) string {
	return strings.TrimPrefix(t, "type.googleapis.com/")
}

// encodeManagedUser encodes {name=1,uuid=2,password=3,flow=4} using protowire.
func encodeManagedUser(u UserCredential) []byte {
	b := make([]byte, 0, 64)
	appendStr := func(field protowire.Number, s string) {
		if s == "" {
			return
		}
		b = protowire.AppendTag(b, field, protowire.BytesType)
		b = protowire.AppendString(b, s)
	}
	appendStr(1, u.Email) // name
	appendStr(2, u.UUID)
	appendStr(3, u.Password)
	appendStr(4, u.Flow)
	return b
}

// decodeManagedUser decodes a protowire {name=1,uuid=2,password=3,flow=4}
// message into a UserCredential. Unknown fields are skipped. fallbackEmail is
// used when the name field is absent.
func decodeManagedUser(v []byte, fallbackEmail string) (UserCredential, error) {
	var u UserCredential
	u.Email = fallbackEmail
	for len(v) > 0 {
		num, typ, n := protowire.ConsumeTag(v)
		if n < 0 {
			return u, protowire.ParseError(n)
		}
		v = v[n:]
		switch num {
		case 1:
			s, n := protowire.ConsumeString(v)
			if n < 0 {
				return u, protowire.ParseError(n)
			}
			u.Email = s
			v = v[n:]
		case 2:
			s, n := protowire.ConsumeString(v)
			if n < 0 {
				return u, protowire.ParseError(n)
			}
			u.UUID = s
			v = v[n:]
		case 3:
			s, n := protowire.ConsumeString(v)
			if n < 0 {
				return u, protowire.ParseError(n)
			}
			u.Password = s
			v = v[n:]
		case 4:
			s, n := protowire.ConsumeString(v)
			if n < 0 {
				return u, protowire.ParseError(n)
			}
			u.Flow = s
			v = v[n:]
		default:
			n := protowire.ConsumeFieldValue(num, typ, v)
			if n < 0 {
				return u, protowire.ParseError(n)
			}
			v = v[n:]
		}
	}
	return u, nil
}
