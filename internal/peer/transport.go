package peer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	magic0 = 'M'
	magic1 = 'N'
	protoV = 1

	msgPing = 1
	msgPong = 2
	msgTest = 3

	maxPayload = 4096
)

// EncodePing builds a ping frame.
func EncodePing(seq uint32, payload string) []byte {
	return encode(msgPing, seq, []byte(payload))
}

// EncodeTest builds a test message frame.
func EncodeTest(seq uint32, payload string) []byte {
	return encode(msgTest, seq, []byte(payload))
}

func encode(typ byte, seq uint32, body []byte) []byte {
	if len(body) > maxPayload {
		body = body[:maxPayload]
	}
	buf := make([]byte, 10+len(body))
	buf[0] = magic0
	buf[1] = magic1
	buf[2] = protoV
	buf[3] = typ
	binary.BigEndian.PutUint32(buf[4:8], seq)
	binary.BigEndian.PutUint16(buf[8:10], uint16(len(body)))
	copy(buf[10:], body)
	return buf
}

// Frame is a decoded application message.
type Frame struct {
	Type    byte
	Seq     uint32
	Payload []byte
}

// ReadFrame reads one framed message from conn.
func ReadFrame(conn net.Conn, timeout time.Duration) (Frame, error) {
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	// ICE and UDP transports are message-oriented: read one datagram/frame per Read.
	buf := make([]byte, 10+maxPayload)
	n, err := conn.Read(buf)
	if err != nil {
		return Frame{}, err
	}
	if n < 10 {
		return Frame{}, errors.New("frame too short")
	}
	if buf[0] != magic0 || buf[1] != magic1 || buf[2] != protoV {
		return Frame{}, errors.New("invalid frame magic")
	}
	seq := binary.BigEndian.Uint32(buf[4:8])
	length := binary.BigEndian.Uint16(buf[8:10])
	if int(length) > maxPayload || n < 10+int(length) {
		return Frame{}, fmt.Errorf("invalid payload length %d", length)
	}
	body := buf[10 : 10+length]
	return Frame{Type: buf[3], Seq: seq, Payload: append([]byte(nil), body...)}, nil
}

// PingRTT sends a ping and waits for matching pong.
func PingRTT(conn net.Conn, timeout time.Duration) (time.Duration, error) {
	seq := uint32(time.Now().UnixNano())
	start := time.Now()
	if err := writeAll(conn, EncodePing(seq, "ping")); err != nil {
		return 0, err
	}
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, fmt.Errorf("ping timeout")
		}
		f, err := ReadFrame(conn, remaining)
		if err != nil {
			return 0, err
		}
		if f.Type == msgPong && f.Seq == seq {
			return time.Since(start), nil
		}
	}
}

// RespondPing returns a pong frame for a ping.
func RespondPing(frame Frame) []byte {
	if frame.Type != msgPing {
		return nil
	}
	return encode(msgPong, frame.Seq, []byte("pong"))
}

func writeAll(conn net.Conn, b []byte) error {
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := conn.Write(b)
	return err
}
