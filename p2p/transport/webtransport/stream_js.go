//go:build js

package libp2pwebtransport

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/sourcenetwork/goji/streams"
	"github.com/sourcenetwork/goji/web_transport"
)

var _ network.MuxedStream = (*stream)(nil)

type stream struct {
	sess          *session
	reader        *streams.Reader
	writer        *streams.Writer
	done          bool
	readDeadline  atomic.Pointer[time.Time]
	writeDeadline atomic.Pointer[time.Time]
}

func newStream(s web_transport.WebTransportBidirectionalStreamValue, sess *session) *stream {
	return &stream{
		sess:   sess,
		reader: streams.NewReader(s.Readable().GetBYOBReader()),
		writer: streams.NewWriter(s.Writable().GetWriter()),
	}
}

func (s *stream) Read(b []byte) (int, error) {
	deadline := s.readDeadline.Load()
	if deadline == nil {
		return s.reader.Read(b)
	}
	ctx, cancel := context.WithDeadline(context.Background(), *deadline)
	defer cancel()
	return s.reader.ReadContext(ctx, b)
}

func (s *stream) Write(b []byte) (int, error) {
	deadline := s.writeDeadline.Load()
	if deadline == nil {
		return s.writer.Write(b)
	}
	ctx, cancel := context.WithDeadline(context.Background(), *deadline)
	defer cancel()
	return s.writer.WriteContext(ctx, b)
}

func (s *stream) RemoteAddr() net.Addr { return &s.sess.raddr }
func (s *stream) LocalAddr() net.Addr  { return &noAddr }

func (s *stream) SetDeadline(t time.Time) error {
	if t.IsZero() {
		s.readDeadline.Store(nil)
		s.writeDeadline.Store(nil)
	} else {
		s.readDeadline.Store(&t)
		s.writeDeadline.Store(&t)
	}
	return nil
}

func (s *stream) SetReadDeadline(t time.Time) error {
	if t.IsZero() {
		s.readDeadline.Store(nil)
	} else {
		s.readDeadline.Store(&t)
	}
	return nil
}

func (s *stream) SetWriteDeadline(t time.Time) error {
	if t.IsZero() {
		s.writeDeadline.Store(nil)
	} else {
		s.writeDeadline.Store(&t)
	}
	return nil
}

func (s *stream) Close() error {
	return errors.Join(s.CloseRead(), s.CloseWrite())
}

func (s *stream) CloseWrite() error {
	return s.writer.Close()
}

func (s *stream) CloseRead() error {
	return s.reader.Close()
}

func (s *stream) Reset() error {
	return errors.Join(s.CloseRead(), s.writer.Abort())
}

func (s *stream) ResetWithError(_ network.StreamErrorCode) error {
	return s.Reset()
}
