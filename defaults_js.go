//go:build js

package libp2p

// DefaultTransports is empty on js/wasm. The host comes up with no transports
// configured by default; callers should add the transports they want.
var DefaultTransports Option = func(*Config) error { return nil }

// DefaultPrivateTransports is empty on js/wasm.
var DefaultPrivateTransports Option = func(*Config) error { return nil }

// DefaultListenAddrs configures no listen addresses on js/wasm.
var DefaultListenAddrs Option = func(*Config) error { return nil }
