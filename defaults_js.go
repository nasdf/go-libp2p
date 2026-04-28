//go:build js

package libp2p

import (
	webtransport "github.com/libp2p/go-libp2p/p2p/transport/webtransport"
)

// DefaultTransports configures the wasm-only webtransport client transport.
// Listening is not supported in the browser; this transport only dials.
var DefaultTransports = ChainOptions(
	Transport(webtransport.New),
)

// DefaultPrivateTransports is empty on js/wasm. WebTransport does not yet
// support private networks.
var DefaultPrivateTransports Option = func(*Config) error { return nil }

// DefaultListenAddrs configures no listen addresses on js/wasm.
var DefaultListenAddrs Option = func(*Config) error { return nil }
