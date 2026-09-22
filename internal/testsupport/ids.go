package testsupport

import (
	"fmt"
	"sync"

	"premark/internal/ports"
)

var _ ports.IDGenerator = (*SeqIDs)(nil)

type SeqIDs struct {
	mu     sync.Mutex
	prefix string
	seq    int
}

func NewSeqIDs(prefix string) *SeqIDs {
	return &SeqIDs{
		prefix: prefix,
		seq:    0,
	}
}

func (s *SeqIDs) NewID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return fmt.Sprintf("%s-%d", s.prefix, s.seq)
}
