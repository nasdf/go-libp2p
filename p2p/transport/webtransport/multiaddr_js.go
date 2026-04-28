//go:build js

package libp2pwebtransport

import (
	"errors"
	"fmt"
	"net"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
)

var webtransportMA = ma.StringCast("/quic-v1/webtransport")

func toWebtransportMultiaddr(na net.Addr) (ma.Multiaddr, error) {
	addr, err := manet.FromNetAddr(na)
	if err != nil {
		return nil, err
	}
	if _, err := addr.ValueForProtocol(ma.P_UDP); err != nil {
		return nil, errors.New("not a UDP address")
	}
	return addr.Encapsulate(webtransportMA), nil
}

func extractCertHashes(addr ma.Multiaddr) ([]multihash.DecodedMultihash, error) {
	certHashesStr := make([]string, 0, 2)
	ma.ForEach(addr, func(c ma.Component) bool {
		if c.Protocol().Code == ma.P_CERTHASH {
			certHashesStr = append(certHashesStr, c.Value())
		}
		return true
	})
	certHashes := make([]multihash.DecodedMultihash, 0, len(certHashesStr))
	for _, s := range certHashesStr {
		_, ch, err := multibase.Decode(s)
		if err != nil {
			return nil, fmt.Errorf("failed to multibase-decode certificate hash: %w", err)
		}
		dh, err := multihash.Decode(ch)
		if err != nil {
			return nil, fmt.Errorf("failed to multihash-decode certificate hash: %w", err)
		}
		// WebTransport only supports sha256 certificates.
		if dh.Code == multihash.SHA2_256 {
			certHashes = append(certHashes, *dh)
		}
	}
	return certHashes, nil
}

// IsWebtransportMultiaddr returns true if the given multiaddr is a well-formed
// webtransport multiaddr, along with the number of certhashes found.
func IsWebtransportMultiaddr(multiaddr ma.Multiaddr) (bool, int) {
	const (
		init = iota
		foundUDP
		foundQuicV1
		foundWebTransport
	)
	state := init
	certhashCount := 0

	ma.ForEach(multiaddr, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_UDP:
			if state == init {
				state = foundUDP
			}
		case ma.P_QUIC_V1:
			if state == foundUDP {
				state = foundQuicV1
			}
		case ma.P_WEBTRANSPORT:
			if state == foundQuicV1 {
				state = foundWebTransport
			}
		case ma.P_CERTHASH:
			if state == foundWebTransport {
				certhashCount++
			}
		}
		return true
	})
	return state == foundWebTransport, certhashCount
}
