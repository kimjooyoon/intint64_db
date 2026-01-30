package main

import (
	"encoding/binary"
	"net"
	"time"
)

const packetSize = 32

type packet [4]int64

func parsePacket(b []byte) (p packet, ok bool) {
	if len(b) < packetSize {
		return packet{}, false
	}
	p[0] = int64(binary.LittleEndian.Uint64(b[0:8]))
	p[1] = int64(binary.LittleEndian.Uint64(b[8:16]))
	p[2] = int64(binary.LittleEndian.Uint64(b[16:24]))
	p[3] = int64(binary.LittleEndian.Uint64(b[24:32]))
	return p, true
}

func (p packet) bytes() []byte {
	b := make([]byte, packetSize)
	binary.LittleEndian.PutUint64(b[0:8], uint64(p[0]))
	binary.LittleEndian.PutUint64(b[8:16], uint64(p[1]))
	binary.LittleEndian.PutUint64(b[16:24], uint64(p[2]))
	binary.LittleEndian.PutUint64(b[24:32], uint64(p[3]))
	return b
}

// req: single channel into the actor (command or query).
type req struct {
	p    packet
	from *net.UDPAddr
}

// resp: actor sends query responses here; one writer goroutine sends UDP.
type resp struct {
	p packet
	to *net.UDPAddr
}

func runServer(store *Store, listenAddr string) error {
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	reqCh := make(chan req, 256)
	respCh := make(chan resp, 256)
	go actor(store, reqCh, respCh)
	go func() {
		for r := range respCh {
			_, _ = conn.WriteToUDP(r.p.bytes(), r.to)
		}
	}()

	buf := make([]byte, packetSize)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil || n < packetSize {
			continue
		}
		p, ok := parsePacket(buf[:n])
		if !ok {
			continue
		}
		if p[0] != 0 && p[0] != 1 {
			continue
		}
		reqCh <- req{p: p, from: from}
	}
}

// actor: single goroutine that owns Store (DoD ECS). All reads/writes go through here.
func actor(store *Store, reqCh <-chan req, respCh chan<- resp) {
	interval := store.SaveIntervalSec()
	if interval <= 0 {
		interval = 60
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case r, ok := <-reqCh:
			if !ok {
				return
			}
			switch r.p[0] {
			case 0:
				handleCommand(store, r.p)
				store.SetLastCall(r.p[1], time.Now().Unix(), r.p[2])
			case 1:
				out := handleQuery(store, r.p)
				if out != nil {
					respCh <- resp{p: *out, to: r.from}
				}
			}
		case <-ticker.C:
			_ = store.Flush()
		}
	}
}

func handleCommand(store *Store, p packet) {
	a, b, c, d := p[0], p[1], p[2], p[3]
	if a != 0 {
		return
	}
	switch b {
	case 0:
		if c == 0 {
			// 0.0.0.value: auto save
			next := store.LastID() + 1
			if store.inRange(next) {
				store.Write(next, d)
				store.IncLastID()
			}
		}
	case 1:
		// 0.1.id.value: target replace
		store.Write(c, d)
	case 2:
		// 0.2.id.value: id값 >= last_id면 스킵, else 저장
		cur, _ := store.Read(c)
		if cur >= store.LastID() {
			return
		}
		store.Write(c, d)
	case 3:
		// 0.3.id.value: id값 >= last_id면 수정+last_id=id+저장, else 스킵
		cur, _ := store.Read(c)
		if cur < store.LastID() {
			return
		}
		store.Write(c, d)
		store.SetLastID(c)
	case 4:
		// 0.4.id.value: id값 >= last_id면 수정+last_id=id+저장, else last_id만 id로
		cur, _ := store.Read(c)
		if cur >= store.LastID() {
			store.Write(c, d)
		}
		store.SetLastID(c)
	case 5:
		// 0.5.n.value: 시간 퀀타이즈 인덱스에 저장
		unit := store.QuantizeUnit(c)
		id := timeQuantizedID(unit)
		if id >= 0 {
			store.Write(id, d)
		}
	case 6:
		// 0.6.n.unit: 퀀타이즈 n의 단위 설정 (0=초,1=분,2=시,3~62=분0~59)
		if d >= 0 && d <= 62 {
			store.SetQuantizeUnit(c, byte(d))
		}
	}
}

func timeQuantizedID(unit byte) int64 {
	now := time.Now().Unix()
	switch unit {
	case 0:
		return now
	case 1:
		return now / 60
	case 2:
		return now / 3600
	default:
		if unit >= 3 && unit <= 62 {
			min := int64(unit - 3)
			hour := now / 3600
			return hour*60 + min
		}
		return -1
	}
}

func handleQuery(store *Store, p packet) *packet {
	a, b, c, d := p[0], p[1], p[2], p[3]
	if a != 1 {
		return nil
	}
	switch b {
	case 0:
		if c == 0 {
			// 1.0.0.id: read one
			val, ok := store.Read(d)
			if !ok {
				val = 0
			}
			return &packet{1, 0, 0, val}
		}
	case 9:
		// 1.9.type.0: last call timestamp, 1.9.type.1: last call id
		ts, id, ok := store.LastCall(c)
		if !ok {
			ts, id = 0, 0
		}
		if d == 0 {
			return &packet{1, 9, c, ts}
		}
		if d == 1 {
			return &packet{1, 9, c, id}
		}
	}
	return nil
}
