//go:build js

package libp2pwebtransport

import (
	"errors"
	"net"
	"sync"

	tpt "github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var errListenerClosed = errors.New("webtransport listener closed")

// noopListener is a placeholder listener for js/wasm. The browser cannot accept
// inbound WebTransport sessions, so listing this transport in the host's listen
// addresses must not error. Accept blocks until Close, mirroring how a closed
// listener behaves elsewhere in libp2p.
type noopListener struct {
	addr      ma.Multiaddr
	closeOnce sync.Once
	closed    chan struct{}
}

func newNoopListener(addr ma.Multiaddr) *noopListener {
	// basic host's address manager strips unspecified IPs (e.g. 0.0.0.0) and
	// expects to resolve them via the OS interface list — empty on wasm. To
	// keep at least one address in the host's published set so identify and
	// EvtLocalAddressesUpdated fire, swap any unspecified IP for loopback.
	if manet.IsIPUnspecified(addr) {
		if rewritten := rewriteUnspecifiedToLoopback(addr); rewritten != nil {
			addr = rewritten
		}
	}
	return &noopListener{addr: addr, closed: make(chan struct{})}
}

func rewriteUnspecifiedToLoopback(addr ma.Multiaddr) ma.Multiaddr {
	var out ma.Multiaddr
	for _, c := range addr {
		switch c.Protocol().Code {
		case ma.P_IP4:
			c, err := ma.NewComponent("ip4", "127.0.0.1")
			if err != nil {
				return nil
			}
			out = appendComponent(out, c)
		case ma.P_IP6:
			c, err := ma.NewComponent("ip6", "::1")
			if err != nil {
				return nil
			}
			out = appendComponent(out, c)
		default:
			out = appendComponent(out, &c)
		}
	}
	return out
}

func appendComponent(m ma.Multiaddr, c *ma.Component) ma.Multiaddr {
	if m == nil {
		return c.Multiaddr()
	}
	return m.Encapsulate(c)
}

func (l *noopListener) Accept() (tpt.CapableConn, error) {
	<-l.closed
	return nil, errListenerClosed
}

func (l *noopListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (l *noopListener) Addr() net.Addr      { return &noAddr }
func (l *noopListener) Multiaddr() ma.Multiaddr { return l.addr }
