package agenttrust

import (
	"encoding/json"
	"errors"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Events are decoded by a trusted receipt adapter. This fixture validates their
// configured registry, identity and canonical lineage; it is not an RPC client.
type Event struct {
	Kind        string
	Registry    common.Address
	Ref         Ref
	Actor       common.Address
	Index       uint64
	Value       string
	Decimals    uint8
	Tag         string
	Validator   common.Address
	RequestHash common.Hash
	Payload     string
	Score       uint8
}
type Block struct {
	Number       uint64
	Hash, Parent common.Hash
	Events       []Event
}
type FeedbackKey struct {
	Client common.Address
	Index  uint64
}
type Feedback struct {
	Value    string
	Decimals uint8
	Tag      string
	Revoked  bool
}
type Validation struct {
	Validator common.Address
	Score     uint8
	Tag       string
	Responded bool
}
type Projection struct {
	mu                              sync.Mutex
	ref                             Ref
	owner, repRegistry, valRegistry common.Address
	blocks                          []Block
	feedback                        map[FeedbackKey]Feedback
	validations                     map[common.Hash]Validation
}

func NewProjection(ref Ref, owner, repRegistry, valRegistry common.Address) (*Projection, error) {
	if !ref.Valid() || owner == (common.Address{}) || repRegistry == (common.Address{}) || valRegistry == (common.Address{}) {
		return nil, ErrDenied
	}
	return &Projection{ref: ref, owner: owner, repRegistry: repRegistry, valRegistry: valRegistry, feedback: map[FeedbackKey]Feedback{}, validations: map[common.Hash]Validation{}}, nil
}
func (p *Projection) rebuild(blocks []Block) (map[FeedbackKey]Feedback, map[common.Hash]Validation, error) {
	feedback := map[FeedbackKey]Feedback{}
	validations := map[common.Hash]Validation{}
	for _, block := range blocks {
		for _, e := range block.Events {
			if e.Ref != p.ref || e.Actor == (common.Address{}) {
				return nil, nil, ErrDenied
			}
			switch e.Kind {
			case "feedback":
				if e.Registry != p.repRegistry || e.Actor == p.owner || e.Index == 0 || e.Decimals > 18 || len(e.Value) > 40 || e.Tag == "" {
					return nil, nil, ErrDenied
				}
				n, ok := new(big.Int).SetString(e.Value, 10)
				if !ok || n.String() != e.Value || n.Cmp(new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))) < 0 || n.Cmp(new(big.Int).Lsh(big.NewInt(1), 127)) >= 0 {
					return nil, nil, ErrDenied
				}
				key := FeedbackKey{e.Actor, e.Index}
				if _, exists := feedback[key]; exists {
					return nil, nil, errors.New("duplicate feedback identity")
				}
				feedback[key] = Feedback{e.Value, e.Decimals, e.Tag, false}
			case "revoke":
				key := FeedbackKey{e.Actor, e.Index}
				f, exists := feedback[key]
				if e.Registry != p.repRegistry || !exists {
					return nil, nil, ErrDenied
				}
				f.Revoked = true
				feedback[key] = f
			case "request":
				if e.Registry != p.valRegistry || e.Actor != p.owner || e.Validator == (common.Address{}) || len(e.Payload) == 0 || len(e.Payload) > 16384 || crypto.Keccak256Hash([]byte(e.Payload)) != e.RequestHash {
					return nil, nil, ErrDenied
				}
				if _, exists := validations[e.RequestHash]; exists {
					return nil, nil, errors.New("request commitment reused")
				}
				validations[e.RequestHash] = Validation{Validator: e.Validator}
			case "response":
				v, exists := validations[e.RequestHash]
				if e.Registry != p.valRegistry || !exists || e.Actor != v.Validator || e.Score > 100 {
					return nil, nil, ErrDenied
				}
				v.Score = e.Score
				v.Tag = e.Tag
				v.Responded = true
				validations[e.RequestHash] = v
			default:
				return nil, nil, errors.New("unsupported event")
			}
		}
	}
	return feedback, validations, nil
}
func (p *Projection) Apply(b Block) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	raw, _ := json.Marshal(b)
	if len(raw) > 128<<10 || len(b.Events) > 256 || b.Hash == (common.Hash{}) {
		return ErrDenied
	}
	for _, old := range p.blocks {
		if old.Hash == b.Hash {
			a, _ := json.Marshal(old)
			c, _ := json.Marshal(b)
			if string(a) == string(c) {
				return nil
			}
			return errors.New("conflicting block replay")
		}
	}
	if len(p.blocks) >= 128 {
		return errors.New("fixture replay bound reached")
	}
	if len(p.blocks) == 0 {
		if b.Number != 1 || b.Parent != (common.Hash{}) {
			return ErrDenied
		}
	} else {
		prev := p.blocks[len(p.blocks)-1]
		if b.Number != prev.Number+1 || b.Parent != prev.Hash {
			return errors.New("reorg must rewind before replay")
		}
	}
	// Clone event storage: subsequent caller mutations must not rewrite evidence.
	b.Events = append([]Event(nil), b.Events...)
	blocks := append(append([]Block(nil), p.blocks...), b)
	f, v, err := p.rebuild(blocks)
	if err != nil {
		return err
	}
	p.blocks = blocks
	p.feedback = f
	p.validations = v
	return nil
}

// Rewind to zero removes every observation. A retained ancestor is required;
// unknown/deeper ancestry is an error, never a guessed rollback.
func (p *Projection) Rewind(ancestor common.Hash) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	if ancestor != (common.Hash{}) {
		n = -1
		for i, b := range p.blocks {
			if b.Hash == ancestor {
				n = i + 1
				break
			}
		}
		if n < 0 {
			return ErrDenied
		}
	}
	blocks := append([]Block(nil), p.blocks[:n]...)
	f, v, err := p.rebuild(blocks)
	if err != nil {
		return err
	}
	p.blocks = blocks
	p.feedback = f
	p.validations = v
	return nil
}

// Mean returns an exact rational; no float rounding or mixing of tags/units.
func (p *Projection) Mean(reviewers []common.Address, tag string) (string, int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(reviewers) == 0 || len(reviewers) > 128 || tag == "" {
		return "", 0, ErrDenied
	}
	allowed := map[common.Address]bool{}
	for _, a := range reviewers {
		if a == (common.Address{}) {
			return "", 0, ErrDenied
		}
		allowed[a] = true
	}
	sum := new(big.Rat)
	count := 0
	for key, f := range p.feedback {
		if allowed[key.Client] && !f.Revoked && f.Tag == tag {
			n, _ := new(big.Int).SetString(f.Value, 10)
			den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(f.Decimals)), nil)
			sum.Add(sum, new(big.Rat).SetFrac(n, den))
			count++
		}
	}
	if count == 0 {
		return "", 0, errors.New("no trusted samples")
	}
	return sum.Quo(sum, new(big.Rat).SetInt64(int64(count))).RatString(), count, nil
}
func (p *Projection) Validation(hash common.Hash) (Validation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.validations[hash]
	if !ok || !v.Responded {
		return Validation{}, ErrDenied
	}
	return v, nil
}
