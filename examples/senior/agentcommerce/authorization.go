// Package agentcommerce models local payment and job recovery. It is not an x402 SDK or deployed escrow.
package agentcommerce

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

var ErrDenied = errors.New("commerce: denied or conflicting intent")
var ErrUnknown = errors.New("commerce: unresolved external outcome; do not repay")

type Mechanism struct {
	Version                                                   int
	Scheme, Network, Asset, Facilitator, TransferMethod, Flow string
}

func (m Mechanism) Normalized() Mechanism {
	if m.TransferMethod == "" {
		m.TransferMethod = "eip3009"
	}
	if m.Flow == "" {
		m.Flow = "authorization"
	}
	return m
}

// Supported is the deliberately narrow payment adapter implemented below.
func (m Mechanism) Supported() bool {
	m = m.Normalized()
	return m.Version == 2 && m.Scheme == "exact" && m.Network == "eip155:84532" && m.Asset == FixtureAsset.Hex() && m.Facilitator == "local-chain-v1" && m.TransferMethod == "eip3009" && m.Flow == "authorization"
}

var FixtureAsset = common.HexToAddress("0x0000000000000000000000000000000000000100")
var FixtureMerchant = common.HexToAddress("0x0000000000000000000000000000000000000200")

func FixtureMechanism() Mechanism {
	return Mechanism{2, "exact", "eip155:84532", FixtureAsset.Hex(), "local-chain-v1", "eip3009", "authorization"}
}

// FlowStages describes core v2 ordering; it does NOT assert a network binding
// implements every flow. Concrete adapters still need a supported matrix.
func FlowStages(flow string) ([]string, error) {
	switch flow {
	case "authorization":
		return []string{"verify", "resource", "settle"}, nil
	case "upfront":
		return []string{"settle", "resource"}, nil
	case "escrow":
		return []string{"deposit", "resource", "capture"}, nil
	}
	return nil, ErrDenied
}

// RunFlow is a LOCAL orchestration fixture. On any stage failure it stops; an
// ambiguous deposit/capture needs recovery, not a fresh run of this helper.
func RunFlow(ctx context.Context, flow string, stage func(context.Context, string) error) error {
	steps, e := FlowStages(flow)
	if e != nil || stage == nil {
		return ErrDenied
	}
	for _, s := range steps {
		if e = ctx.Err(); e != nil {
			return e
		}
		if e = stage(ctx, s); e != nil {
			return e
		}
	}
	return nil
}

type Payment struct {
	From, To                common.Address
	Value                   string
	ValidAfter, ValidBefore int64
	Nonce                   common.Hash
	Signature               []byte
}

// PaymentDigest uses the actual EIP-3009 TransferWithAuthorization type. The
// fixture token/domain are pinned locally, not inferred from untrusted quotes.
func PaymentDigest(m Mechanism, p Payment) (common.Hash, error) {
	if !m.Supported() || p.From == (common.Address{}) || p.To != FixtureMerchant || p.Value != "10" || p.ValidAfter < 0 || p.ValidBefore <= p.ValidAfter || p.Nonce == (common.Hash{}) {
		return common.Hash{}, ErrDenied
	}
	chain := math.HexOrDecimal256(*big.NewInt(84532))
	typed := apitypes.TypedData{Types: apitypes.Types{
		"EIP712Domain":              {{Name: "name", Type: "string"}, {Name: "version", Type: "string"}, {Name: "chainId", Type: "uint256"}, {Name: "verifyingContract", Type: "address"}},
		"TransferWithAuthorization": {{Name: "from", Type: "address"}, {Name: "to", Type: "address"}, {Name: "value", Type: "uint256"}, {Name: "validAfter", Type: "uint256"}, {Name: "validBefore", Type: "uint256"}, {Name: "nonce", Type: "bytes32"}},
	}, PrimaryType: "TransferWithAuthorization", Domain: apitypes.TypedDataDomain{Name: "Local Fixture USD", Version: "1", ChainId: &chain, VerifyingContract: FixtureAsset.Hex()}, Message: apitypes.TypedDataMessage{"from": p.From.Hex(), "to": p.To.Hex(), "value": p.Value, "validAfter": fmt.Sprint(p.ValidAfter), "validBefore": fmt.Sprint(p.ValidBefore), "nonce": p.Nonce.Hex()}}
	digest, _, err := apitypes.TypedDataAndHash(typed)
	return common.BytesToHash(digest), err
}
func VerifyPayment(m Mechanism, p Payment, now int64) (common.Hash, error) {
	digest, err := PaymentDigest(m, p)
	if err != nil || now <= p.ValidAfter || now >= p.ValidBefore || len(p.Signature) != 65 || p.Signature[64] > 1 {
		return common.Hash{}, ErrDenied
	}
	pub, err := crypto.SigToPub(digest[:], p.Signature)
	if err != nil || crypto.PubkeyToAddress(*pub) != p.From {
		return common.Hash{}, ErrDenied
	}
	return digest, nil
}

type Binding struct {
	Tenant, Subject, Intent, Method, Resource string
	BodyHash                                  [32]byte
}

func NewBinding(tenant, subject, intent, method, resource string, body []byte) (Binding, error) {
	u, err := url.Parse(resource)
	if err != nil || u.Scheme != "https" || u.Host != "reports.fixture" || u.Path != "/report" || u.RawQuery != "" || u.User != nil || u.Fragment != "" || tenant == "" || subject == "" || intent == "" || len(intent) > 128 || method != "POST" || len(body) > 16<<10 {
		return Binding{}, ErrDenied
	}
	return Binding{tenant, subject, intent, method, resource, sha256.Sum256(body)}, nil
}
func bindingHash(b Binding) string {
	raw, _ := json.Marshal(b)
	h := sha256.Sum256(raw)
	return common.Bytes2Hex(h[:])
}
