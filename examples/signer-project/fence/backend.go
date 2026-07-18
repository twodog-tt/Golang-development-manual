package fence

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const digestDomain = "signer-project/payload/v1"

// Digest is the domain-separated SHA-256 digest passed to signing backends.
type Digest [sha256.Size]byte

// BackendResult is the cryptographic material returned by a Backend. The fence
// store persists this material as part of a Receipt before releasing it.
type BackendResult struct {
	Algorithm string
	PublicKey []byte
	Signature []byte
}

// Backend performs the cryptographic operation after the durable fence and
// request reservation have committed. Implementations should document their
// own retry and determinism behavior.
type Backend interface {
	Sign(ctx context.Context, keyID string, digest Digest) (BackendResult, error)
}

// DigestPayload produces the exact digest supplied to a Backend. A length
// prefix prevents concatenation ambiguity if this encoding gains more fields.
func DigestPayload(payload []byte) Digest {
	h := sha256.New()
	_, _ = h.Write([]byte(digestDomain))
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(payload)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(payload)
	var digest Digest
	copy(digest[:], h.Sum(nil))
	return digest
}

func validateBackendResult(result BackendResult) error {
	switch {
	case result.Algorithm == "":
		return errors.New("algorithm is empty")
	case len(result.Algorithm) > 128:
		return errors.New("algorithm is too long")
	case len(result.PublicKey) == 0:
		return errors.New("public key is empty")
	case len(result.PublicKey) > 64*1024:
		return errors.New("public key is too large")
	case len(result.Signature) == 0:
		return errors.New("signature is empty")
	case len(result.Signature) > 64*1024:
		return errors.New("signature is too large")
	default:
		return nil
	}
}
