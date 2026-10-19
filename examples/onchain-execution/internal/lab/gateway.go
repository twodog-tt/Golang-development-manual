// Package lab is an intentionally local-only, single-chain execution lab.
package lab

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const routerABI = `[{"type":"function","name":"swapExactTokensForTokens","inputs":[{"type":"uint256","name":"amountIn"},{"type":"uint256","name":"amountOutMin"},{"type":"address[]","name":"path"},{"type":"address","name":"to"},{"type":"uint256","name":"deadline"}],"outputs":[{"type":"uint256[]"}]}]`

var RouterABI, _ = abi.JSON(strings.NewReader(routerABI))
var amountPattern = regexp.MustCompile(`^[1-9][0-9]{0,29}$`)
var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var ErrConflict = errors.New("intent conflict")

type Config struct {
	Router           common.Address `json:"router"`
	TokenIn          common.Address `json:"token_in"`
	TokenOut         common.Address `json:"token_out"`
	Pair             common.Address `json:"pair"`
	RouterCodeHash   common.Hash    `json:"router_code_hash"`
	Receiver         common.Address `json:"receiver"`
	ReceiverCodeHash common.Hash    `json:"receiver_code_hash"`
}
type Request struct {
	Amount   string `json:"amount"`
	MinOut   string `json:"min_out"`
	Deadline int64  `json:"deadline"`
}
type Intent struct {
	ID          string   `json:"id"`
	Nonce       uint64   `json:"nonce"`
	State       string   `json:"state"`
	Hashes      []string `json:"hashes"`
	BlockHash   *string  `json:"block_hash"`
	Transaction string   `json:"transaction"`
}
type App struct {
	DB     *pgxpool.Pool
	RPC    *ethclient.Client
	Key    *ecdsa.PrivateKey
	Sender common.Address
	Config Config
}

func Amount(s string) (*big.Int, error) {
	if !amountPattern.MatchString(s) {
		return nil, errors.New("amount must be a positive atomic integer string, at most 30 digits")
	}
	n, _ := new(big.Int).SetString(s, 10)
	return n, nil
}
func (a *App) CheckChain(ctx context.Context) error {
	chain, err := a.RPC.ChainID(ctx)
	if err != nil {
		return err
	}
	if chain.Cmp(big.NewInt(31337)) != 0 {
		return errors.New("lab only supports chain 31337")
	}
	code, err := a.RPC.CodeAt(ctx, a.Config.Router, nil)
	if err != nil {
		return err
	}
	if len(code) == 0 || crypto.Keccak256Hash(code) != a.Config.RouterCodeHash {
		return errors.New("router bytecode mismatch")
	}
	if a.Config.Receiver != (common.Address{}) {
		code, err = a.RPC.CodeAt(ctx, a.Config.Receiver, nil)
		if err != nil {
			return err
		}
		if len(code) == 0 || crypto.Keccak256Hash(code) != a.Config.ReceiverCodeHash {
			return errors.New("receiver bytecode mismatch")
		}
	}
	return nil
}
func (a *App) Data(ctx context.Context, r Request) ([]byte, error) {
	amount, err := Amount(r.Amount)
	if err != nil {
		return nil, err
	}
	min, err := Amount(r.MinOut)
	if err != nil {
		return nil, err
	}
	h, err := a.RPC.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	if r.Deadline <= int64(h.Time) || r.Deadline > int64(h.Time)+3600 {
		return nil, errors.New("deadline outside (head time, head time + 3600]")
	}
	return RouterABI.Pack("swapExactTokensForTokens", amount, min, []common.Address{a.Config.TokenIn, a.Config.TokenOut}, a.Sender, big.NewInt(r.Deadline))
}
func (a *App) signed(nonce uint64, data []byte, price *big.Int) (*types.Transaction, []byte, error) {
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: nonce, To: &a.Config.Router, Gas: 400000, GasPrice: price, Data: data}), types.NewEIP155Signer(big.NewInt(31337)), a.Key)
	if err != nil {
		return nil, nil, err
	}
	raw, err := tx.MarshalBinary()
	return tx, raw, err
}
func (a *App) Create(ctx context.Context, id string, r Request) error {
	if !idPattern.MatchString(id) {
		return errors.New("invalid intent id")
	}
	payload, _ := json.Marshal(r)
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// One signer/chain in this lab; all cooperating gateway instances share the lock.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(31337001)"); err != nil {
		return err
	}
	var old string
	err = tx.QueryRow(ctx, "SELECT payload FROM intents WHERE id=$1", id).Scan(&old)
	if err == nil {
		if old != string(payload) {
			return ErrConflict
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err = a.CheckChain(ctx); err != nil {
		return err
	}
	data, err := a.Data(ctx, r)
	if err != nil {
		return err
	}
	// Simulation is a preflight, not a promise that inclusion will succeed.
	if _, err = a.RPC.CallContract(ctx, ethereum.CallMsg{From: a.Sender, To: &a.Config.Router, Gas: 400000, Data: data}, nil); err != nil {
		return fmt.Errorf("simulation: %w", err)
	}
	nonce, err := a.RPC.PendingNonceAt(ctx, a.Sender)
	if err != nil {
		return err
	}
	var next int64
	if err = tx.QueryRow(ctx, "SELECT COALESCE(max(nonce)+1,0) FROM intents").Scan(&next); err != nil {
		return err
	}
	if uint64(next) > nonce {
		nonce = uint64(next)
	}
	price, err := a.RPC.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}
	signed, raw, err := a.signed(nonce, data, price)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO intents(id,payload,nonce) VALUES($1,$2,$3)", id, string(payload), int64(nonce)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO attempts VALUES($1,0,$2,$3,$4)", id, signed.Hash().Hex(), raw, price.String()); err != nil {
		return err
	}
	// Durability MUST precede sendRawTransaction. No network send inside this transaction.
	return tx.Commit(ctx)
}

func (a *App) Broadcast(ctx context.Context, id string) error {
	if err := a.CheckChain(ctx); err != nil {
		return err
	}
	var raw []byte
	var state string
	err := a.DB.QueryRow(ctx, `SELECT raw,state FROM attempts JOIN intents ON id=intent WHERE id=$1 ORDER BY seq DESC LIMIT 1`, id).Scan(&raw, &state)
	if err != nil {
		return err
	}
	if state == "mined" || state == "confirmed" || state == "reverted" || state == "orphaned_review" {
		return ErrConflict
	}
	var tx types.Transaction
	if err = tx.UnmarshalBinary(raw); err != nil {
		return err
	}
	if err = a.RPC.SendTransaction(ctx, &tx); err != nil {
		// A timeout, 503, or "already known" is ambiguous. Keep the same signed bytes.
		return fmt.Errorf("broadcast unknown; reconcile same intent/hash: %w", err)
	}
	_, err = a.DB.Exec(ctx, "UPDATE intents SET state='broadcast' WHERE id=$1 AND state='signed'", id)
	return err
}

func (a *App) Bump(ctx context.Context, id string) error {
	if err := a.CheckChain(ctx); err != nil {
		return err
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(31337001)"); err != nil {
		return err
	}
	var raw []byte
	var seq int
	var price string
	var state string
	if err = tx.QueryRow(ctx, `SELECT raw,seq,gas_price::text,state FROM attempts JOIN intents ON id=intent WHERE id=$1 ORDER BY seq DESC LIMIT 1 FOR UPDATE OF intents`, id).Scan(&raw, &seq, &price, &state); err != nil {
		return err
	}
	if seq >= 2 || (state != "signed" && state != "broadcast") {
		return ErrConflict
	}
	rows, err := tx.Query(ctx, "SELECT hash FROM attempts WHERE intent=$1", id)
	if err != nil {
		return err
	}
	var hashes []common.Hash
	for rows.Next() {
		var h string
		if err = rows.Scan(&h); err != nil {
			rows.Close()
			return err
		}
		hashes = append(hashes, common.HexToHash(h))
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, h := range hashes {
		receipt, e := a.RPC.TransactionReceipt(ctx, h)
		if e == nil && receipt != nil {
			return ErrConflict
		}
		if e != nil && !errors.Is(e, ethereum.NotFound) {
			return e
		}
	}
	var old types.Transaction
	if err = old.UnmarshalBinary(raw); err != nil {
		return err
	}
	values, err := RouterABI.Methods["swapExactTokensForTokens"].Inputs.Unpack(old.Data()[4:])
	if err != nil {
		return err
	}
	h, err := a.RPC.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	if values[4].(*big.Int).Uint64() <= h.Time {
		return errors.New("expired intent requires review")
	}
	p, _ := new(big.Int).SetString(price, 10)
	p.Mul(p, big.NewInt(2))
	signed, raw, err := a.signed(old.Nonce(), old.Data(), p)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO attempts VALUES($1,$2,$3,$4,$5)", id, seq+1, signed.Hash().Hex(), raw, p.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) Reconcile(ctx context.Context, id string) (Intent, error) {
	if err := a.CheckChain(ctx); err != nil {
		return Intent{}, err
	}
	// Serialize reconciliation with replacement and other reconcilers. A stale
	// reader must not overwrite a newer state after observing an older head.
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return Intent{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(31337001)"); err != nil {
		return Intent{}, err
	}
	var out Intent
	out.ID = id
	if err = tx.QueryRow(ctx, "SELECT nonce,state,block_hash FROM intents WHERE id=$1", id).Scan(&out.Nonce, &out.State, &out.BlockHash); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, "SELECT hash FROM attempts WHERE intent=$1 ORDER BY seq", id)
	if err != nil {
		return out, err
	}
	out.Hashes = []string{}
	for rows.Next() {
		var h string
		if err = rows.Scan(&h); err != nil {
			rows.Close()
			return out, err
		}
		out.Hashes = append(out.Hashes, h)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return out, err
	}
	var found *types.Receipt
	for _, h := range out.Hashes {
		r, e := a.RPC.TransactionReceipt(ctx, common.HexToHash(h))
		if errors.Is(e, ethereum.NotFound) {
			continue
		}
		if e != nil {
			return out, e
		}
		header, e := a.RPC.HeaderByNumber(ctx, r.BlockNumber)
		if e != nil {
			return out, e
		}
		if header.Hash() == r.BlockHash {
			found = r
			break
		}
	}
	if found != nil {
		out.Transaction = found.TxHash.Hex()
		head, e := a.RPC.HeaderByNumber(ctx, nil)
		if e != nil {
			return out, e
		}
		if head.Number.Cmp(found.BlockNumber) < 0 {
			return out, errors.New("RPC head behind receipt")
		}
		out.State = "mined"
		if found.Status == 0 {
			out.State = "reverted"
		} else if head.Number.Uint64()-found.BlockNumber.Uint64()+1 >= 2 {
			out.State = "confirmed"
		}
		hash := found.BlockHash.Hex()
		out.BlockHash = &hash
		_, err = tx.Exec(ctx, "UPDATE intents SET state=$2,block_hash=$3,block_height=$4 WHERE id=$1", id, out.State, hash, found.BlockNumber.Int64())
	} else if out.BlockHash != nil {
		out.State = "orphaned_review"
		_, err = tx.Exec(ctx, "UPDATE intents SET state=$2 WHERE id=$1", id, out.State)
	}
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
