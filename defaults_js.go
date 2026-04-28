//go:build js

package libp2p

import (
	webtransport "github.com/libp2p/go-libp2p/p2p/transport/webtransport"

	"github.com/multiformats/go-multiaddr"
)

// DefaultTransports configures the wasm-only webtransport client transport.
// Listening is not supported in the browser; this transport only dials.
var DefaultTransports = ChainOptions(
	Transport(webtransport.New),
)

// DefaultPrivateTransports is empty on js/wasm. WebTransport does not yet
// support private networks.
var DefaultPrivateTransports Option = func(*Config) error { return nil }

// DefaultListenAddrs configures a webtransport listen address on js/wasm. The
// browser cannot actually accept inbound webtransport sessions, but the
// transport returns a no-op listener so a host configured with this address
// starts cleanly.
var DefaultListenAddrs = func(cfg *Config) error {
	addr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/udp/0/quic-v1/webtransport")
	if err != nil {
		return err
	}
	return cfg.Apply(ListenAddrs(addr))
}
