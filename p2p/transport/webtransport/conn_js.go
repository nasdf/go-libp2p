//go:build js

package libp2pwebtransport

import (
	"context"
	"net"

	"github.com/libp2p/go-libp2p/core/network"
	tpt "github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
)

var _ net.Addr = (*addr)(nil)

var noAddr = addr{"https://0.0.0.0" + webtransportHTTPEndpoint + "?type=noise"}

type addr struct {
	url string
}

func (a *addr) Network() string { return "webtransport" }
func (a *addr) String() string  { return a.url }

var _ network.ConnMultiaddrs = &connMultiaddrs{}

type connSecurityMultiaddrs struct {
	network.ConnSecurity
	network.ConnMultiaddrs
}

type connMultiaddrs struct {
	local, remote ma.Multiaddr
}

func (c *connMultiaddrs) LocalMultiaddr() ma.Multiaddr  { return c.local }
func (c *connMultiaddrs) RemoteMultiaddr() ma.Multiaddr { return c.remote }

var _ tpt.CapableConn = (*conn)(nil)

type conn struct {
	*connSecurityMultiaddrs
	sess      *session
	scope     network.ConnScope
	transport *transport
}

func newConn(t *transport, sess *session, scope network.ConnScope, sconn *connSecurityMultiaddrs) *conn {
	return &conn{
		connSecurityMultiaddrs: sconn,
		transport:              t,
		scope:                  scope,
		sess:                   sess,
	}
}

func (c *conn) OpenStream(ctx context.Context) (network.MuxedStream, error) {
	return c.sess.openStream(ctx)
}

func (c *conn) AcceptStream() (network.MuxedStream, error) {
	return c.sess.acceptStream()
}

func (c *conn) Close() error {
	return c.sess.close()
}

func (c *conn) CloseWithError(_ network.ConnErrorCode) error {
	return c.Close()
}

func (c *conn) IsClosed() bool {
	return c.sess.isClosed.Load()
}

func (c *conn) Scope() network.ConnScope {
	return c.scope
}

func (c *conn) Transport() tpt.Transport {
	return c.transport
}

func (c *conn) ConnState() network.ConnectionState {
	return network.ConnectionState{Transport: "webtransport"}
}

func (c *conn) As(target any) bool {
	if cc, ok := target.(**conn); ok {
		*cc = c
		return true
	}
	return false
}
