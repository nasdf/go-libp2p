//go:build js

package libp2pwebtransport

import (
	"context"
	"io"
	"sync/atomic"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/sourcenetwork/goji"
	"github.com/sourcenetwork/goji/streams"
	"github.com/sourcenetwork/goji/web_transport"
)

type session struct {
	wt       web_transport.WebTransportValue
	incoming streams.ReadableStreamDefaultReaderValue
	raddr    addr
	maddr    ma.Multiaddr
	done     bool
	isClosed atomic.Bool
}

func newSession(wt web_transport.WebTransportValue, maddr ma.Multiaddr, url string) *session {
	return &session{
		wt:       wt,
		incoming: wt.IncomingBidirectionalStreams().GetDefaultReader(),
		raddr:    addr{url},
		maddr:    maddr,
	}
}

func (s *session) openStream(ctx context.Context) (*stream, error) {
	res, err := goji.AwaitContext(ctx, s.wt.CreateBidirectionalStream())
	if err != nil {
		return nil, err
	}
	str := web_transport.WebTransportBidirectionalStreamValue(res[0])
	return newStream(str, s), nil
}

func (s *session) acceptStream() (*stream, error) {
	if s.done {
		return nil, io.EOF
	}
	res, err := goji.Await(s.incoming.Read())
	if err != nil {
		return nil, err
	}
	s.done = res[0].Get("done").Bool()
	if s.done {
		return nil, io.EOF
	}
	v := web_transport.WebTransportBidirectionalStreamValue(res[0].Get("value"))
	return newStream(v, s), nil
}

func (s *session) ready(ctx context.Context) error {
	_, err := goji.AwaitContext(ctx, s.wt.Ready())
	return err
}

func (s *session) close() error {
	s.wt.Close(nil)
	_, err := goji.Await(s.wt.Closed())
	s.isClosed.Store(true)
	return err
}
