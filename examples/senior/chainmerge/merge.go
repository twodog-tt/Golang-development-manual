// Package chainmerge demonstrates a canonical-chain merge boundary for
// backfill and realtime observations.
//
// Blocks are retained by hash as immutable evidence. Canonical adoption is a
// separate operation that verifies parent continuity and refuses to rewrite a
// finalized height. Storage and transactions are in-memory for the example.
package chainmerge

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

var (
	ErrInvalidBlock        = errors.New("chainmerge: invalid block")
	ErrEvidenceConflict    = errors.New("chainmerge: same hash has conflicting evidence")
	ErrUnknownHead         = errors.New("chainmerge: head is not observed")
	ErrGap                 = errors.New("chainmerge: parent lineage has a gap")
	ErrInvalidLineage      = errors.New("chainmerge: parent height is not contiguous")
	ErrFinalizedConflict   = errors.New("chainmerge: adoption conflicts with finalized history")
	ErrStaleHead           = errors.New("chainmerge: proposed head is lower than current head")
	ErrIncompleteOverlap   = errors.New("chainmerge: source does not cover canonical overlap")
	ErrInvalidFinalization = errors.New("chainmerge: invalid finalization")
)

type Source string

const (
	SourceBackfill Source = "backfill"
	SourceRealtime Source = "realtime"
)

type Block struct {
	Height      uint64
	Hash        string
	ParentHash  string
	PayloadHash [32]byte
}

type evidence struct {
	block   Block
	sources map[Source]struct{}
}

// Change is ordered for consumers: Orphaned is descending height (rollback
// order), while Applied is ascending height (replay order).
type Change struct {
	CommonAncestor uint64
	Orphaned       []Block
	Applied        []Block
}

type Merger struct {
	mu sync.Mutex

	anchorHeight    uint64
	finalizedHeight uint64
	headHeight      uint64
	headHash        string
	evidenceByHash  map[string]*evidence
	canonical       map[uint64]string
}

// New treats anchor as an already trusted canonical/finalized boundary.
func New(anchor Block) (*Merger, error) {
	if anchor.Hash == "" || anchor.PayloadHash == [32]byte{} {
		return nil, ErrInvalidBlock
	}
	observed := &evidence{block: anchor, sources: make(map[Source]struct{})}
	return &Merger{
		anchorHeight:    anchor.Height,
		finalizedHeight: anchor.Height,
		headHeight:      anchor.Height,
		headHash:        anchor.Hash,
		evidenceByHash:  map[string]*evidence{anchor.Hash: observed},
		canonical:       map[uint64]string{anchor.Height: anchor.Hash},
	}, nil
}

// Observe records evidence without changing canonical ownership. The same
// height may legitimately have multiple hashes during a reorg or disagreement.
func (m *Merger) Observe(source Source, block Block) (bool, error) {
	if !validSource(source) || block.Hash == "" || block.PayloadHash == [32]byte{} ||
		block.Height < m.anchorHeight ||
		(block.Height > m.anchorHeight && block.ParentHash == "") {
		return false, ErrInvalidBlock
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if block.Height == m.anchorHeight && block.Hash != m.canonical[m.anchorHeight] {
		return false, ErrFinalizedConflict
	}
	if current, ok := m.evidenceByHash[block.Hash]; ok {
		if current.block != block {
			return false, ErrEvidenceConflict
		}
		if _, duplicate := current.sources[source]; duplicate {
			return false, nil
		}
		current.sources[source] = struct{}{}
		return true, nil
	}
	m.evidenceByHash[block.Hash] = &evidence{
		block:   block,
		sources: map[Source]struct{}{source: {}},
	}
	return true, nil
}

// Adopt verifies a complete hash-linked path to the current canonical chain,
// computes rollback/replay events, and commits the canonical change atomically.
func (m *Merger) Adopt(headHash string) (Change, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	head, ok := m.evidenceByHash[headHash]
	if !ok {
		return Change{}, ErrUnknownHead
	}
	if head.block.Height < m.headHeight {
		return Change{}, fmt.Errorf("%w: proposed=%d current=%d", ErrStaleHead, head.block.Height, m.headHeight)
	}
	if m.canonical[head.block.Height] == headHash {
		return Change{CommonAncestor: head.block.Height}, nil
	}

	// Do not preallocate from an externally observed height: a malicious or
	// corrupt height near MaxUint64 could otherwise trigger a huge allocation
	// before lineage validation reaches the first missing parent.
	var pathDescending []Block
	visited := make(map[string]struct{})
	cursor := head.block
	var ancestor uint64
	for {
		if _, loop := visited[cursor.Hash]; loop {
			return Change{}, ErrInvalidLineage
		}
		visited[cursor.Hash] = struct{}{}

		if canonicalHash, exists := m.canonical[cursor.Height]; exists && canonicalHash == cursor.Hash {
			ancestor = cursor.Height
			break
		}
		if cursor.Height <= m.finalizedHeight {
			return Change{}, fmt.Errorf("%w: height=%d finalized=%d", ErrFinalizedConflict, cursor.Height, m.finalizedHeight)
		}
		pathDescending = append(pathDescending, cursor)

		parent, exists := m.evidenceByHash[cursor.ParentHash]
		if !exists {
			return Change{}, fmt.Errorf("%w: child=%s parent=%s", ErrGap, cursor.Hash, cursor.ParentHash)
		}
		if parent.block.Height+1 != cursor.Height {
			return Change{}, fmt.Errorf(
				"%w: child=%s/%d parent=%s/%d",
				ErrInvalidLineage,
				cursor.Hash,
				cursor.Height,
				parent.block.Hash,
				parent.block.Height,
			)
		}
		cursor = parent.block
	}

	change := Change{CommonAncestor: ancestor}
	for height := m.headHeight; height > ancestor; height-- {
		hash, exists := m.canonical[height]
		if !exists {
			return Change{}, fmt.Errorf("%w: canonical height=%d", ErrGap, height)
		}
		change.Orphaned = append(change.Orphaned, m.evidenceByHash[hash].block)
	}
	for index := len(pathDescending) - 1; index >= 0; index-- {
		change.Applied = append(change.Applied, pathDescending[index])
	}

	for height := m.headHeight; height > ancestor; height-- {
		delete(m.canonical, height)
	}
	for _, block := range change.Applied {
		m.canonical[block.Height] = block.Hash
	}
	m.headHeight = head.block.Height
	m.headHash = head.block.Hash
	return change, nil
}

// Finalize advances a monotonic finality watermark over the canonical chain.
func (m *Merger) Finalize(height uint64, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if height < m.finalizedHeight || height > m.headHeight || hash == "" || m.canonical[height] != hash {
		return ErrInvalidFinalization
	}
	m.finalizedHeight = height
	return nil
}

// VerifyOverlap proves that every source observed the same canonical hash at
// every height in the handoff interval. A maximum seen height is not enough.
func (m *Merger) VerifyOverlap(from, to uint64, sources ...Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if from > to || len(sources) == 0 || from < m.anchorHeight || to > m.headHeight {
		return ErrIncompleteOverlap
	}
	for height := from; height <= to; height++ {
		hash, ok := m.canonical[height]
		if !ok {
			return fmt.Errorf("%w: missing canonical height=%d", ErrIncompleteOverlap, height)
		}
		observed := m.evidenceByHash[hash]
		for _, source := range sources {
			if _, ok = observed.sources[source]; !ok {
				return fmt.Errorf("%w: source=%s height=%d hash=%s", ErrIncompleteOverlap, source, height, hash)
			}
		}
		if height == ^uint64(0) {
			break
		}
	}
	return nil
}

func (m *Merger) Canonical(height uint64) (Block, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hash, ok := m.canonical[height]
	if !ok {
		return Block{}, false
	}
	return m.evidenceByHash[hash].block, true
}

func (m *Merger) Head() Block {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.evidenceByHash[m.headHash].block
}

func (m *Merger) Sources(hash string) []Source {
	m.mu.Lock()
	defer m.mu.Unlock()
	observed := m.evidenceByHash[hash]
	if observed == nil {
		return nil
	}
	sources := make([]Source, 0, len(observed.sources))
	for source := range observed.sources {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i] < sources[j] })
	return sources
}

func validSource(source Source) bool {
	return source == SourceBackfill || source == SourceRealtime
}
