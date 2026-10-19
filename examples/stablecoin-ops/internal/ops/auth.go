package ops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type Auth struct {
	Store            *Store
	Domain, URI, RPC string
}
type Challenge struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (a Auth) Challenge(ctx context.Context, address string) (Challenge, error) {
	if !common.IsHexAddress(address) {
		return Challenge{}, ErrDenied
	}
	address = common.HexToAddress(address).Hex()
	var exists bool
	if err := a.Store.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM ops.wallets WHERE address=$1)", address).Scan(&exists); err != nil {
		return Challenge{}, err
	}
	if !exists {
		return Challenge{}, ErrDenied
	}
	id := ID()
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(5 * time.Minute)
	msg := fmt.Sprintf("%s wants you to sign in with your Ethereum account:\n%s\n\nSign in to the local stablecoin operations lab.\n\nURI: %s\nVersion: 1\nChain ID: 31337\nNonce: %s\nIssued At: %s\nExpiration Time: %s", a.Domain, address, a.URI, id, now.Format("2006-01-02T15:04:05.000Z"), expires.Format("2006-01-02T15:04:05.000Z"))
	_, err := a.Store.DB.Exec(ctx, "INSERT INTO ops.challenges VALUES($1,$2,$3,$4)", id, address, msg, expires)
	return Challenge{id, msg}, err
}
func verifyEOA(address, message, signature string) error {
	sig, err := hexutil.Decode(signature)
	if err != nil || len(sig) != 65 {
		return ErrDenied
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if !crypto.ValidateSignatureValues(sig[64], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:64]), true) {
		return ErrDenied
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(message)), sig)
	if err != nil || crypto.PubkeyToAddress(*pub) != common.HexToAddress(address) {
		return ErrDenied
	}
	return nil
}
func verify1271(ctx context.Context, url, address, message, signature string) error {
	if url == "" {
		return ErrDenied
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	client, err := rpc.DialContext(ctx, url)
	if err != nil {
		return ErrDenied
	}
	defer client.Close()
	var chain hexutil.Big
	if err = client.CallContext(ctx, &chain, "eth_chainId"); err != nil || (*big.Int)(&chain).Cmp(big.NewInt(31337)) != 0 {
		return ErrDenied
	}
	var block hexutil.Uint64
	if err = client.CallContext(ctx, &block, "eth_blockNumber"); err != nil {
		return ErrDenied
	}
	sig, err := hexutil.Decode(signature)
	if err != nil || len(sig) > 4096 {
		return ErrDenied
	}
	b32, _ := abi.NewType("bytes32", "", nil)
	dynamic, _ := abi.NewType("bytes", "", nil)
	var digest [32]byte
	copy(digest[:], accounts.TextHash([]byte(message)))
	data, err := (abi.Arguments{{Type: b32}, {Type: dynamic}}).Pack(digest, sig)
	if err != nil {
		return ErrDenied
	}
	data = append(common.FromHex("0x1626ba7e"), data...)
	var result hexutil.Bytes
	if err = client.CallContext(ctx, &result, "eth_call", map[string]string{"to": address, "data": hexutil.Encode(data)}, hexutil.EncodeUint64(uint64(block))); err != nil {
		return ErrDenied
	}
	if len(result) != 32 || hex.EncodeToString(result[:4]) != "1626ba7e" {
		return ErrDenied
	}
	return nil
}
func digest(token string) string { b := sha256.Sum256([]byte(token)); return hex.EncodeToString(b[:]) }
func (a Auth) Login(ctx context.Context, id, signature string) (string, error) {
	var address, message, merchant, kind string
	err := a.Store.DB.QueryRow(ctx, "SELECT w.address,c.message,w.merchant,w.kind FROM ops.challenges c JOIN ops.wallets w ON c.address=w.address WHERE c.id=$1 AND c.expires>now()", id).Scan(&address, &message, &merchant, &kind)
	if err != nil {
		return "", ErrDenied
	}
	if kind == "eoa" {
		err = verifyEOA(address, message, signature)
	} else if kind == "erc1271" {
		err = verify1271(ctx, a.RPC, address, message, signature)
	} else {
		err = ErrDenied
	}
	if err != nil {
		return "", ErrDenied
	}
	tx, err := a.Store.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	// Bind the session to the wallet that signed, and serialize with unbinding.
	// A binding changed while RPC verification was in flight cannot issue a session.
	var currentMerchant, currentKind string
	if err = tx.QueryRow(ctx, "SELECT merchant,kind FROM ops.wallets WHERE address=$1 FOR SHARE", address).Scan(&currentMerchant, &currentKind); err != nil || currentMerchant != merchant || currentKind != kind {
		return "", ErrDenied
	}
	tag, err := tx.Exec(ctx, "DELETE FROM ops.challenges WHERE id=$1 AND expires>now()", id)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", ErrDenied
	}
	token := ID()
	if _, err = tx.Exec(ctx, "INSERT INTO ops.sessions(digest,merchant,address,chain_id,expires) VALUES($1,$2,$3,31337,now()+interval '15 minutes')", digest(token), merchant, address); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return token, nil
}
func (a Auth) Merchant(ctx context.Context, header string) (string, error) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", ErrDenied
	}
	var merchant string
	err := a.Store.DB.QueryRow(ctx, "SELECT s.merchant FROM ops.sessions s JOIN ops.wallets w ON w.address=s.address AND w.merchant=s.merchant WHERE s.digest=$1 AND s.chain_id=31337 AND s.expires>now()", digest(strings.TrimPrefix(header, "Bearer "))).Scan(&merchant)
	if err != nil {
		return "", ErrDenied
	}
	return merchant, nil
}
func (a Auth) Logout(ctx context.Context, header string) error {
	if !strings.HasPrefix(header, "Bearer ") {
		return errors.New("missing bearer token")
	}
	_, err := a.Store.DB.Exec(ctx, "DELETE FROM ops.sessions WHERE digest=$1", digest(strings.TrimPrefix(header, "Bearer ")))
	return err
}
