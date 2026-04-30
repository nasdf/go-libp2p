//go:build js

package libp2pwebtransport

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/sourcenetwork/goji/streams"
	"github.com/sourcenetwork/goji/web_transport"
)

// errInputStream is what Chromium throws on a WebTransport bidirectional
// stream's reader when the peer's STOP_SENDING (sent by the peer's
// Close -> CancelRead path) is processed, even after the payload has
// already been delivered. Per spec, STOP_SENDING applies only to the
// writer half and should not affect the reader, but Chrome surfaces it
// here. Once we've already delivered bytes to the caller we treat this
// as a clean EOF rather than propagating a spurious failure.
const errInputStream = "TypeError: Error in input stream"

var _ network.MuxedStream = (*stream)(nil)

type stream struct {
	sess          *session
	reader        *streams.Reader
	writer        *streams.Writer
	readAny       bool // true once at least one byte has been delivered to the caller
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
	if s.done {
		return 0, io.EOF
	}
	n, err := s.read(b)
	if n > 0 {
		s.readAny = true
	}
	if err != nil {
		if s.readAny && err.Error() == errInputStream {
			s.done = true
			if n > 0 {
				return n, nil
			}
			return 0, io.EOF
		}
		return n, err
	}
	return n, nil
}

func (s *stream) read(b []byte) (int, error) {
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
	return errors.Join(s.CloseWrite(), s.CloseRead())
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
