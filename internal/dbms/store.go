package dbms

import (
	"encoding/binary"
	"os"
)

const slotSize = 8
const metaSize = 4 * slotSize
const quantizeMaxN = 64

// Store is single-owner: only the actor goroutine may call its methods (DoD/ECS style).
type Store struct {
	dataPath  string
	metaPath  string
	quantPath string
	data      *os.File
	slots     int64
	lastID    int64
	dirty     bool
	saveSec   int64
	quantUnit [quantizeMaxN]byte
	lastCall  map[int64]lastCallInfo
}

type lastCallInfo struct {
	ts int64
	id int64
}

func OpenStore(dataPath, metaPath, quantPath string, slots int64) (*Store, error) {
	if slots <= 0 {
		slots = 1024 * 1024
	}
	size := slots * slotSize

	createIfNotExists := func(path string, sz int64) error {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		info, _ := f.Stat()
		if info.Size() < sz {
			if err := f.Truncate(sz); err != nil {
				return err
			}
		}
		return nil
	}
	if err := createIfNotExists(dataPath, size); err != nil {
		return nil, err
	}

	data, err := os.OpenFile(dataPath, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	s := &Store{
		dataPath:  dataPath,
		metaPath:  metaPath,
		quantPath: quantPath,
		data:      data,
		slots:     slots,
		lastCall:  make(map[int64]lastCallInfo),
	}
	if err := s.loadMeta(); err != nil {
		data.Close()
		return nil, err
	}
	s.loadQuantize()
	return s, nil
}

func (s *Store) loadMeta() error {
	b := make([]byte, metaSize)
	f, err := os.Open(s.metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.lastID = 0
			s.saveSec = 60
			return nil
		}
		return err
	}
	defer f.Close()
	n, _ := f.Read(b)
	if n >= slotSize {
		s.lastID = int64(binary.LittleEndian.Uint64(b[0:8]))
	}
	if n >= 2*slotSize {
		s.saveSec = int64(binary.LittleEndian.Uint64(b[8:16]))
		if s.saveSec <= 0 {
			s.saveSec = 60
		}
	}
	return nil
}

func (s *Store) saveMeta() error {
	b := make([]byte, metaSize)
	binary.LittleEndian.PutUint64(b[0:8], uint64(s.lastID))
	binary.LittleEndian.PutUint64(b[8:16], uint64(s.saveSec))
	return os.WriteFile(s.metaPath, b, 0644)
}

func (s *Store) loadQuantize() {
	b, err := os.ReadFile(s.quantPath)
	if err != nil || len(b) < quantizeMaxN {
		return
	}
	for i := 0; i < quantizeMaxN && i < len(b); i++ {
		s.quantUnit[i] = b[i]
	}
}

func (s *Store) saveQuantize() error {
	return os.WriteFile(s.quantPath, s.quantUnit[:], 0644)
}

func (s *Store) Close() error {
	s.flushLocked()
	return s.data.Close()
}

func (s *Store) inRange(id int64) bool {
	return id >= 0 && id < s.slots
}

func (s *Store) Read(id int64) (int64, bool) {
	if !s.inRange(id) {
		return 0, false
	}
	b := make([]byte, slotSize)
	_, err := s.data.ReadAt(b, id*slotSize)
	if err != nil {
		return 0, false
	}
	return int64(binary.LittleEndian.Uint64(b)), true
}

func (s *Store) Write(id int64, value int64) bool {
	if !s.inRange(id) {
		return false
	}
	b := make([]byte, slotSize)
	binary.LittleEndian.PutUint64(b, uint64(value))
	_, err := s.data.WriteAt(b, id*slotSize)
	if err != nil {
		return false
	}
	s.dirty = true
	return true
}

func (s *Store) LastID() int64 {
	return s.lastID
}

func (s *Store) SetLastID(id int64) bool {
	if id < 0 || id >= s.slots {
		return false
	}
	s.lastID = id
	s.dirty = true
	return true
}

func (s *Store) IncLastID() bool {
	next := s.lastID + 1
	if next >= s.slots {
		return false
	}
	s.lastID = next
	s.dirty = true
	return true
}

func (s *Store) QuantizeUnit(n int64) byte {
	if n < 0 || n >= quantizeMaxN {
		return 0
	}
	return s.quantUnit[n]
}

func (s *Store) SetQuantizeUnit(n int64, unit byte) bool {
	if n < 0 || n >= quantizeMaxN || unit > 62 {
		return false
	}
	s.quantUnit[n] = unit
	_ = s.saveQuantize()
	return true
}

func (s *Store) SetLastCall(cmdType int64, ts, id int64) {
	s.lastCall[cmdType] = lastCallInfo{ts: ts, id: id}
}

func (s *Store) LastCall(cmdType int64) (ts, id int64, ok bool) {
	lc, ok := s.lastCall[cmdType]
	if !ok {
		return 0, 0, false
	}
	return lc.ts, lc.id, true
}

func (s *Store) SaveIntervalSec() int64 {
	return s.saveSec
}

func (s *Store) Flush() error {
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
	if s.data != nil {
		_ = s.data.Sync()
	}
	if s.dirty {
		s.dirty = false
		return s.saveMeta()
	}
	return nil
}
