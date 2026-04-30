//go:build js

package libp2pwebtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/connmgr"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	tpt "github.com/libp2p/go-libp2p/core/transport"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/security/noise/pb"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multihash"
	"github.com/sourcenetwork/goji/web_transport"
)

const webtransportHTTPEndpoint = "/.well-known/libp2p-webtransport"

// Option matches the native package's Option signature so the constructor can
// accept variadic options. All options are no-ops on js/wasm.
type Option func(*transport) error

var _ tpt.Transport = (*transport)(nil)

type transport struct {
	privKey ic.PrivKey
	pid     peer.ID

	rcmgr network.ResourceManager
	gater connmgr.ConnectionGater

	noise *noise.Transport
}

// New creates a wasm-only WebTransport client. The signature mirrors the
// native package so libp2p's transport DI provides the same parameters; the
// connManager is unused since wasm cannot listen.
func New(key ic.PrivKey, psk pnet.PSK, _ *quicreuse.ConnManager, gater connmgr.ConnectionGater, rcmgr network.ResourceManager, _ ...Option) (tpt.Transport, error) {
	if len(psk) > 0 {
		return nil, errors.New("WebTransport doesn't support private networks yet")
	}
	if rcmgr == nil {
		rcmgr = &network.NullResourceManager{}
	}
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, err
	}
	n, err := noise.New(noise.ID, key, nil)
	if err != nil {
		return nil, err
	}
	return &transport{
		pid:     id,
		privKey: key,
		rcmgr:   rcmgr,
		gater:   gater,
		noise:   n,
	}, nil
}

func (t *transport) Protocols() []int {
	return []int{ma.P_WEBTRANSPORT}
}

func (t *transport) Proxy() bool {
	return false
}

// Listen returns a no-op listener. The browser cannot accept inbound
// WebTransport sessions, but returning a stub listener keeps default
// configurations that include a webtransport listen address from erroring.
func (t *transport) Listen(addr ma.Multiaddr) (tpt.Listener, error) {
	return newNoopListener(addr), nil
}

func (t *transport) CanDial(addr ma.Multiaddr) bool {
	ok, _ := IsWebtransportMultiaddr(addr)
	return ok
}

func (t *transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.CapableConn, error) {
	scope, err := t.rcmgr.OpenConnection(network.DirOutbound, false, raddr)
	if err != nil {
		return nil, err
	}
	c, err := t.dialWithScope(ctx, raddr, p, scope)
	if err != nil {
		scope.Done()
		return nil, err
	}
	return c, nil
}

func (t *transport) dialWithScope(ctx context.Context, raddr ma.Multiaddr, p peer.ID, scope network.ConnManagementScope) (tpt.CapableConn, error) {
	_, host, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}
	certHashes, err := extractCertHashes(raddr)
	if err != nil {
		return nil, err
	}
	if len(certHashes) == 0 {
		return nil, errors.New("can't dial webtransport without certhashes")
	}
	if err := scope.SetPeer(p); err != nil {
		return nil, err
	}
	sni, _ := extractSNI(raddr)
	maddr, _ := ma.SplitFunc(raddr, func(c ma.Component) bool { return c.Protocol().Code == ma.P_WEBTRANSPORT })
	sess, err := t.dial(ctx, maddr, host, sni, certHashes)
	if err != nil {
		return nil, err
	}
	sconn, err := t.upgrade(ctx, sess, p, certHashes)
	if err != nil {
		sess.close()
		return nil, err
	}
	if t.gater != nil && !t.gater.InterceptSecured(network.DirOutbound, p, sconn) {
		sess.close()
		return nil, fmt.Errorf("secured connection gated")
	}
	return newConn(t, sess, scope, sconn), nil
}

func (t *transport) dial(ctx context.Context, maddr ma.Multiaddr, host, sni string, certHashes []multihash.DecodedMultihash) (*session, error) {
	dialHost := host
	if sni != "" {
		_, port, err := net.SplitHostPort(host)
		if err != nil {
			return nil, err
		}
		dialHost = net.JoinHostPort(sni, port)
	}
	url := fmt.Sprintf("https://%s%s?type=noise", dialHost, webtransportHTTPEndpoint)
	certHashValues := make([]web_transport.CertificateHashValue, 0)
	for _, hash := range certHashes {
		certHashValues = append(certHashValues, web_transport.CertificateHash("sha-256", hash.Digest))
	}
	wt := web_transport.WebTransport.New(url, web_transport.WebTransportOptions.WithServerCertificateHashes(certHashValues...))
	sess := newSession(wt, maddr, url)
	return sess, sess.ready(ctx)
}

func (t *transport) upgrade(ctx context.Context, sess *session, p peer.ID, certHashes []multihash.DecodedMultihash) (*connSecurityMultiaddrs, error) {
	s, err := sess.openStream(ctx)
	if err != nil {
		return nil, err
	}

	var verified bool
	n, err := t.noise.WithSessionOptions(noise.EarlyData(newEarlyDataReceiver(func(b *pb.NoiseExtensions) error {
		if b == nil {
			return errors.New("missing webtransport certificate hashes")
		}
		decodedCertHashes, err := decodeCertHashesFromProtobuf(b.WebtransportCerthashes)
		if err != nil {
			return err
		}
		for _, sent := range certHashes {
			var found bool
			for _, rcvd := range decodedCertHashes {
				if sent.Code == rcvd.Code && bytes.Equal(sent.Digest, rcvd.Digest) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("missing cert hash: %v", sent)
			}
		}
		verified = true
		return nil
	}), nil))
	if err != nil {
		return nil, errors.Join(s.Close(), fmt.Errorf("failed to create Noise transport: %w", err))
	}
	c, err := n.SecureOutbound(ctx, s, p)
	if err != nil {
		return nil, errors.Join(s.Close(), err)
	}
	if !verified {
		return nil, errors.Join(s.Close(), errors.New("didn't verify"))
	}
	return &connSecurityMultiaddrs{
		ConnSecurity:   c,
		ConnMultiaddrs: &connMultiaddrs{local: webtransportMA, remote: sess.maddr},
	}, nil
}

// extractSNI returns the SNI value from the multiaddr, if any.
func extractSNI(maddr ma.Multiaddr) (sni string, foundSniComponent bool) {
	ma.ForEach(maddr, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_SNI:
			sni = c.Value()
			foundSniComponent = true
			return false
		case ma.P_DNS, ma.P_DNS4, ma.P_DNS6, ma.P_DNSADDR:
			sni = c.Value()
			return true
		}
		return true
	})
	return sni, foundSniComponent
}

func decodeCertHashesFromProtobuf(b [][]byte) ([]multihash.DecodedMultihash, error) {
	hashes := make([]multihash.DecodedMultihash, 0, len(b))
	for _, h := range b {
		dh, err := multihash.Decode(h)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hash: %w", err)
		}
		hashes = append(hashes, *dh)
	}
	return hashes, nil
}
