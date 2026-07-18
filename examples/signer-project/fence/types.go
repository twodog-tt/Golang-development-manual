package fence

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

const (
	storeVersion   = uint16(1)
	MaxPayloadSize = 1 << 20
)

var (
	ErrInvalidRequest  = errors.New("fence: invalid request")
	ErrStaleEpoch      = errors.New("fence: stale epoch")
	ErrOwnerConflict   = errors.New("fence: owner conflicts at the same epoch")
	ErrRequestConflict = errors.New("fence: request ID is bound to different content or authority")
	ErrCorruptStore    = errors.New("fence: corrupt store")
	ErrBackendResult   = errors.New("fence: invalid backend result")
	ErrBackendIdentity = errors.New("fence: backend key identity changed")
)

// Request is assumed to be authenticated and authorized before it reaches this
// package. KeyID, Owner, Epoch, and Payload must be bound by a trusted control
// plane/transport; Sign does not verify a signed grant or caller identity.
type Request struct {
	KeyID     string
	Owner     string
	Epoch     uint64
	RequestID string
	Payload   []byte
}

// Receipt is the persisted signing result. The backend signature covers only
// PayloadDigest; the other fields are durable fence metadata, not an independent
// HSM/MPC attestation. Sign never returns a Receipt until the transaction
// changing the request to COMPLETED has committed.
type Receipt struct {
	Version       uint16
	KeyID         string
	Owner         string
	Epoch         uint64
	RequestID     string
	PayloadDigest Digest
	Algorithm     string
	PublicKey     []byte
	Signature     []byte
}

// RequestStatus is the durable lifecycle of a request reservation.
type RequestStatus string

const (
	StatusPending   RequestStatus = "PENDING"
	StatusCompleted RequestStatus = "COMPLETED"
)

// RequestRecord is a read-only view of a durable request reservation.
type RequestRecord struct {
	Status        RequestStatus
	Owner         string
	Epoch         uint64
	PayloadDigest Digest
	Receipt       *Receipt
}

// FenceState is the durable owner at the highest accepted epoch for one key.
type FenceState struct {
	KeyID string
	Owner string
	Epoch uint64
}

func validateRequest(request Request) error {
	if err := validateName("key ID", request.KeyID, 512); err != nil {
		return err
	}
	if err := validateName("owner", request.Owner, 512); err != nil {
		return err
	}
	if err := validateName("request ID", request.RequestID, 512); err != nil {
		return err
	}
	if request.Epoch == 0 {
		return fmt.Errorf("%w: epoch must be greater than zero", ErrInvalidRequest)
	}
	if len(request.Payload) == 0 {
		return fmt.Errorf("%w: payload is empty", ErrInvalidRequest)
	}
	if len(request.Payload) > MaxPayloadSize {
		return fmt.Errorf("%w: payload exceeds %d bytes", ErrInvalidRequest, MaxPayloadSize)
	}
	return nil
}

func validateName(field, value string, max int) error {
	switch {
	case value == "":
		return fmt.Errorf("%w: %s is empty", ErrInvalidRequest, field)
	case !utf8.ValidString(value):
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidRequest, field)
	case len(value) > max:
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrInvalidRequest, field, max)
	default:
		return nil
	}
}

func cloneReceipt(receipt Receipt) Receipt {
	receipt.PublicKey = append([]byte(nil), receipt.PublicKey...)
	receipt.Signature = append([]byte(nil), receipt.Signature...)
	return receipt
}

func cloneRequestRecord(record RequestRecord) RequestRecord {
	if record.Receipt != nil {
		receipt := cloneReceipt(*record.Receipt)
		record.Receipt = &receipt
	}
	return record
}
