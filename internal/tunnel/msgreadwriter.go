package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/AdguardTeam/golibs/log"
)

// MaxMessageLength is the maximum length that is safe to use.
// TODO(ameshkov): Make it configurable.
const MaxMessageLength = 1280

// MsgReadWriter is a wrapper over io.ReadWriter that encodes messages written
// to and read from the base writer.
type MsgReadWriter struct {
	base io.ReadWriter
}

// NewMsgReadWriter creates a new instance of *MsgReadWriter.
func NewMsgReadWriter(base io.ReadWriter) (rw *MsgReadWriter) {
	return &MsgReadWriter{base: base}
}

// type check
var _ io.ReadWriter = (*MsgReadWriter)(nil)

// Read implements the io.ReadWriter interface for *MsgReadWriter.
func (rw *MsgReadWriter) Read(b []byte) (n int, err error) {
	var length uint16
	err = binary.Read(rw.base, binary.BigEndian, &length)
	if err != nil {
		return 0, fmt.Errorf("reading binary data: %w", err)
	}

	if length > MaxMessageLength {
		// Warn the user that this may not work correctly.
		log.Error(
			"Warning: received message of length %d larger than %d, considering reducing the MTU",
			length,
			MaxMessageLength,
		)
	}

	if len(b) < int(length) {
		return 0, fmt.Errorf("message length %d is greater than the buffer size %d", length, len(b))
	}

	n, err = io.ReadFull(rw.base, b[:length])
	if err != nil {
		return 0, fmt.Errorf("reading message: %w", err)
	}

	return n, nil
}

// Write implements the io.ReadWriter interface for *MsgReadWriter.
func (rw *MsgReadWriter) Write(b []byte) (n int, err error) {
	if len(b) > MaxMessageLength {
		// Warn the user that this may not work correctly.
		log.Error(
			"Warning: trying to write a message of length %d larger than %d, considering reducing the MTU",
			len(b),
			MaxMessageLength,
		)
	}

	msg := Pack(b)

	n, err = rw.base.Write(msg)

	if err != nil {
		return 0, err
	}

	// Subtract the prefix length.
	return n - 2, nil
}

// Pack packs the message to be sent over the tunnel.
func Pack(b []byte) (msg []byte) {
	msg = make([]byte, len(b)+2)

	binary.BigEndian.PutUint16(msg[:2], uint16(len(b)))
	copy(msg[2:], b)

	return msg
}
