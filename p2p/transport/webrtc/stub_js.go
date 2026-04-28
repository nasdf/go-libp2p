//go:build js

// Package libp2pwebrtc is stubbed out on the js/wasm target. The Pion-based
// implementation depends on platform features that are not available in the
// browser. New returns an error and the package only exposes the type
// signatures required for the rest of go-libp2p to compile.
package libp2pwebrtc

import (
	"context"
	"errors"
	"net"

	"github.com/libp2p/go-libp2p/core/connmgr"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	tpt "github.com/libp2p/go-libp2p/core/transport"

	ma "github.com/multiformats/go-multiaddr"
)

var errNotSupported = errors.New("webrtc transport is not supported on js/wasm")

// WebRTCTransport is a placeholder type. It implements transport.Transport so
// code referencing the type still type-checks, but no method is functional.
type WebRTCTransport struct{}

var _ tpt.Transport = &WebRTCTransport{}

// Option mirrors the option type of the real package.
type Option func(*WebRTCTransport) error

// ListenUDPFn mirrors the type used by the real package and referenced by
// config wiring.
type ListenUDPFn func(network string, laddr *net.UDPAddr) (net.PacketConn, error)

// New always returns errNotSupported on js/wasm.
func New(_ ic.PrivKey, _ pnet.PSK, _ connmgr.ConnectionGater, _ network.ResourceManager, _ ListenUDPFn, _ ...Option) (*WebRTCTransport, error) {
	return nil, errNotSupported
}

func (*WebRTCTransport) Protocols() []int          { return nil }
func (*WebRTCTransport) Proxy() bool               { return false }
func (*WebRTCTransport) CanDial(ma.Multiaddr) bool { return false }
func (*WebRTCTransport) Listen(ma.Multiaddr) (tpt.Listener, error) {
	return nil, errNotSupported
}
func (*WebRTCTransport) Dial(_ context.Context, _ ma.Multiaddr, _ peer.ID) (tpt.CapableConn, error) {
	return nil, errNotSupported
}
