package lab

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5"
)

var transfer = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
var swap = crypto.Keccak256Hash([]byte("Swap(address,uint256,uint256,uint256,uint256,address)"))

func (a *App) relevant(l types.Log) bool {
	if len(l.Topics) == 0 {
		return false
	}
	return ((l.Address == a.Config.TokenIn || l.Address == a.Config.TokenOut) && l.Topics[0] == transfer) || (l.Address == a.Config.Pair && l.Topics[0] == swap)
}
func Decode(l types.Log) (string, error) {
	if l.Removed || len(l.Topics) != 3 {
		return "", errors.New("removed log or invalid topic count")
	}
	// Indexed addresses must be ABI-padded; do not silently truncate poison data.
	for _, t := range l.Topics[1:] {
		if strings.Trim(hexPrefix(t.Bytes()[:12]), "0") != "" {
			return "", errors.New("noncanonical address padding")
		}
	}
	switch l.Topics[0] {
	case transfer:
		if len(l.Data) != 32 {
			return "", errors.New("Transfer payload must be 32 bytes")
		}
		return "transfer", nil
	case swap:
		if len(l.Data) != 128 {
			return "", errors.New("Swap payload must be 128 bytes")
		}
		return "swap", nil
	default:
		return "", errors.New("unknown event signature")
	}
}
func hexPrefix(b []byte) string { return fmt.Sprintf("%x", b) }
func logKey(l types.Log) string { return fmt.Sprintf("%s/%s/%d", l.BlockHash, l.TxHash, l.Index) }
func (a *App) quality(ctx context.Context, err error) error {
	// This evidence deliberately survives rollback of the indexing batch.
	_, e := a.DB.Exec(ctx, "INSERT INTO quality_errors(detail) VALUES($1)", err.Error())
	if e != nil {
		return fmt.Errorf("%v; failed to persist quality error: %w", err, e)
	}
	return err
}

func (a *App) Sync(ctx context.Context) error {
	if err := a.CheckChain(ctx); err != nil {
		return err
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(31337002)"); err != nil {
		return err
	}
	// Read the target AFTER obtaining the writer lock. A request waiting behind
	// another batch must not mistake its stale pre-lock head for a real reorg.
	head, err := a.RPC.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	var height, epoch int64
	var hash string
	if err = tx.QueryRow(ctx, "SELECT height,hash,epoch FROM index_meta WHERE id=1 FOR UPDATE").Scan(&height, &hash, &epoch); err != nil {
		return err
	}
	ancestor := height
	if ancestor > head.Number.Int64() {
		ancestor = head.Number.Int64()
	}
	if height-ancestor > 64 {
		return a.quality(ctx, errors.New("head regression exceeds replay bound 64; operator review"))
	}
	for ancestor >= 0 {
		var old string
		if err = tx.QueryRow(ctx, "SELECT hash FROM canonical WHERE height=$1", ancestor).Scan(&old); err != nil {
			return err
		}
		h, e := a.RPC.HeaderByNumber(ctx, big.NewInt(ancestor))
		if e != nil {
			return e
		}
		if h.Hash().Hex() == old {
			break
		}
		ancestor--
		if height-ancestor > 64 {
			return a.quality(ctx, errors.New("reorg exceeds local replay bound 64; operator review"))
		}
	}
	if ancestor < height {
		if _, err = tx.Exec(ctx, "DELETE FROM canonical WHERE height>$1", ancestor); err != nil {
			return err
		}
		epoch++
	}
	parent := ""
	if ancestor >= 0 {
		if err = tx.QueryRow(ctx, "SELECT hash FROM canonical WHERE height=$1", ancestor).Scan(&parent); err != nil {
			return err
		}
	}
	for n := ancestor + 1; n <= head.Number.Int64(); n++ {
		block, e := a.RPC.BlockByNumber(ctx, big.NewInt(n))
		if e != nil {
			return e
		}
		if n > 0 && block.ParentHash().Hex() != parent {
			return a.quality(ctx, errors.New("parent gap during batch"))
		}
		blockHash := block.Hash()
		// Hash-scoped logs avoid an accidental mix of two branches at one height.
		logs, e := a.RPC.FilterLogs(ctx, ethereum.FilterQuery{BlockHash: &blockHash, Addresses: []common.Address{a.Config.TokenIn, a.Config.TokenOut, a.Config.Pair}, Topics: [][]common.Hash{{transfer, swap}}})
		if e != nil {
			return e
		}
		scoped := logs[:0]
		for _, l := range logs {
			if a.relevant(l) {
				scoped = append(scoped, l)
			}
		}
		logs = scoped
		expected := map[string]types.Log{}
		for _, transaction := range block.Transactions() {
			r, e := a.RPC.TransactionReceipt(ctx, transaction.Hash())
			if e != nil {
				return e
			}
			if r.BlockHash != blockHash {
				return a.quality(ctx, errors.New("receipt belongs to another branch"))
			}
			for _, l := range r.Logs {
				if a.relevant(*l) {
					expected[logKey(*l)] = *l
				}
			}
		}
		if len(logs) != len(expected) {
			return a.quality(ctx, fmt.Errorf("log completeness mismatch at %d: logs=%d receipts=%d", n, len(logs), len(expected)))
		}
		if _, err = tx.Exec(ctx, "INSERT INTO raw_blocks VALUES($1,$2,$3) ON CONFLICT DO NOTHING", blockHash.Hex(), n, block.ParentHash().Hex()); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, l := range logs {
			kind, e := Decode(l)
			if e != nil {
				return a.quality(ctx, e)
			}
			want, ok := expected[logKey(l)]
			gotJSON, _ := json.Marshal(l)
			wantJSON, _ := json.Marshal(want)
			if !a.relevant(l) || !ok || seen[logKey(l)] || l.BlockNumber != uint64(n) || string(gotJSON) != string(wantJSON) {
				return a.quality(ctx, errors.New("log identity, duplication or receipt content mismatch"))
			}
			seen[logKey(l)] = true
			if _, err = tx.Exec(ctx, `INSERT INTO raw_logs(block_hash,tx_hash,log_index,height,address,kind,payload) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, blockHash.Hex(), l.TxHash.Hex(), int64(l.Index), n, l.Address.Hex(), kind, string(gotJSON)); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, "INSERT INTO canonical VALUES($1,$2)", n, blockHash.Hex()); err != nil {
			return err
		}
		parent = blockHash.Hex()
	}
	check, err := a.RPC.HeaderByNumber(ctx, head.Number)
	if err != nil {
		return err
	}
	if check.Hash() != head.Hash() {
		return a.quality(ctx, errors.New("head changed during batch; retry whole batch"))
	}
	if _, err = tx.Exec(ctx, "UPDATE index_meta SET height=$1,hash=$2,epoch=$3 WHERE id=1", head.Number.Int64(), head.Hash().Hex(), epoch); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Cursor struct {
	Epoch       int64  `json:"epoch"`
	Height      int64  `json:"height"`
	Hash        string `json:"hash"`
	AfterHeight int64  `json:"after_height"`
	AfterIndex  int64  `json:"after_index"`
}
type Event struct {
	Height      int64           `json:"height"`
	Index       int64           `json:"index"`
	Hash        string          `json:"block_hash"`
	Transaction string          `json:"transaction"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
}
type Page struct {
	Events         []Event `json:"events"`
	Next           string  `json:"next"`
	SnapshotHeight int64   `json:"snapshot_height"`
	SnapshotHash   string  `json:"snapshot_hash"`
	Epoch          int64   `json:"epoch"`
}

func (a *App) Events(ctx context.Context, encoded string, limit int) (Page, error) {
	var out Page
	out.Events = []Event{}
	if limit < 1 || limit > 100 {
		return out, errors.New("limit must be 1..100")
	}
	tx, err := a.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var height, epoch int64
	var hash string
	if err = tx.QueryRow(ctx, "SELECT height,hash,epoch FROM index_meta WHERE id=1").Scan(&height, &hash, &epoch); err != nil {
		return out, err
	}
	c := Cursor{Epoch: epoch, Height: height, Hash: hash, AfterHeight: -1, AfterIndex: -1}
	if encoded != "" {
		if len(encoded) > 1024 {
			return out, errors.New("cursor too large")
		}
		b, e := base64.RawURLEncoding.DecodeString(encoded)
		if e != nil {
			return out, e
		}
		if e = json.Unmarshal(b, &c); e != nil {
			return out, e
		}
		if c.Epoch != epoch || c.Height > height || c.Height < 0 || c.AfterHeight < 0 || c.AfterHeight > c.Height || c.AfterIndex < 0 {
			return out, ErrConflict
		}
		var canonical string
		if err = tx.QueryRow(ctx, "SELECT hash FROM canonical WHERE height=$1", c.Height).Scan(&canonical); err != nil {
			return out, err
		}
		if canonical != c.Hash {
			return out, ErrConflict
		}
	}
	rows, err := tx.Query(ctx, `SELECT l.height,l.log_index,l.block_hash,l.tx_hash,l.kind,l.payload
 FROM raw_logs l JOIN canonical c ON c.height=l.height AND c.hash=l.block_hash
 WHERE l.height <= $1 AND (l.height,l.log_index)>($2,$3)
 ORDER BY l.height,l.log_index LIMIT $4`, c.Height, c.AfterHeight, c.AfterIndex, limit+1)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.Height, &e.Index, &e.Hash, &e.Transaction, &e.Kind, &e.Payload); err != nil {
			rows.Close()
			return out, err
		}
		out.Events = append(out.Events, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	out.SnapshotHeight = c.Height
	out.SnapshotHash = c.Hash
	out.Epoch = c.Epoch
	if len(out.Events) > limit {
		out.Events = out.Events[:limit]
		last := out.Events[limit-1]
		c.AfterHeight = last.Height
		c.AfterIndex = last.Index
		b, _ := json.Marshal(c)
		out.Next = base64.RawURLEncoding.EncodeToString(b)
	}
	return out, tx.Commit(ctx)
}
