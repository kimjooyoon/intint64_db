package client

import (
	"encoding/binary"
	"errors"
	"net"
	"time"
)

const PacketSize = 32

// Packet is one UDP message: 4 x int64 (256 bits), little-endian.
type Packet [4]int64

// Client talks to an intint64_db DBMS over UDP.
type Client struct {
	addr   *net.UDPAddr
	conn   *net.UDPConn
	timeout time.Duration
}

// New creates a client for the given address (e.g. "127.0.0.1:7770").
func New(addr string) (*Client, error) {
	ua, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, ua)
	if err != nil {
		return nil, err
	}
	return &Client{addr: ua, conn: conn, timeout: 5 * time.Second}, nil
}

// Close closes the UDP connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// SetTimeout sets the read timeout for Query. Default 5s.
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// Send sends a command packet (fire-and-forget). Use for commands (first int64 = 0).
func (c *Client) Send(p Packet) error {
	_, err := c.conn.Write(p.bytes())
	return err
}

// Query sends a query packet and returns the response packet. Use for queries (first int64 = 1).
func (c *Client) Query(p Packet) (Packet, error) {
	if _, err := c.conn.Write(p.bytes()); err != nil {
		return Packet{}, err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	b := make([]byte, PacketSize)
	n, err := c.conn.Read(b)
	if err != nil {
		return Packet{}, err
	}
	if n < PacketSize {
		return Packet{}, ErrShortRead
	}
	return packetFromBytes(b), nil
}

// ErrShortRead is returned when the server sends fewer than 32 bytes.
var ErrShortRead = errors.New("intint64_db/client: short read")

func (p Packet) bytes() []byte {
	b := make([]byte, PacketSize)
	binary.LittleEndian.PutUint64(b[0:8], uint64(p[0]))
	binary.LittleEndian.PutUint64(b[8:16], uint64(p[1]))
	binary.LittleEndian.PutUint64(b[16:24], uint64(p[2]))
	binary.LittleEndian.PutUint64(b[24:32], uint64(p[3]))
	return b
}

func packetFromBytes(b []byte) Packet {
	var p Packet
	p[0] = int64(binary.LittleEndian.Uint64(b[0:8]))
	p[1] = int64(binary.LittleEndian.Uint64(b[8:16]))
	p[2] = int64(binary.LittleEndian.Uint64(b[16:24]))
	p[3] = int64(binary.LittleEndian.Uint64(b[24:32]))
	return p
}

// --- Helpers for library users ---

// AutoSave sends 0.0.0.value (append value at last_id+1).
func (c *Client) AutoSave(value int64) error {
	return c.Send(Packet{0, 0, 0, value})
}

// Replace sends 0.1.id.value (target replace).
func (c *Client) Replace(id, value int64) error {
	return c.Send(Packet{0, 1, id, value})
}

// ReadOne sends 1.0.0.id and returns the value at id.
func (c *Client) ReadOne(id int64) (int64, error) {
	resp, err := c.Query(Packet{1, 0, 0, id})
	if err != nil {
		return 0, err
	}
	return resp[3], nil
}

// Range sends 6.0.id1.id2 and returns values for id1 through id2 (inclusive).
// Response is one packet per id, each packet (1,0,0,value). id1 > id2 returns empty slice.
func (c *Client) Range(id1, id2 int64) ([]int64, error) {
	if id1 > id2 {
		return nil, nil
	}
	if _, err := c.conn.Write(Packet{6, 0, id1, id2}.bytes()); err != nil {
		return nil, err
	}
	n := id2 - id1 + 1
	out := make([]int64, 0, n)
	// range 응답은 패킷 여러 개이므로 타임아웃을 넉넉히 (최대 60초)
	rangeTimeout := min(c.timeout * time.Duration(n+1), 60 * time.Second)
	deadline := time.Now().Add(rangeTimeout)
	for range n {
		_ = c.conn.SetReadDeadline(deadline)
		b := make([]byte, PacketSize)
		read, err := c.conn.Read(b)
		if err != nil {
			return out, err
		}
		if read < PacketSize {
			return out, ErrShortRead
		}
		p := packetFromBytes(b)
		out = append(out, p[3])
	}
	return out, nil
}
