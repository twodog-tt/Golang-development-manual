// Package fence implements a durable, single-active signer fencing state
// machine. It is not a complete authorization boundary.
//
// Security precondition: before calling Sign, a trusted transport/control plane
// must authenticate the caller and cryptographically bind/authorize KeyID,
// Owner, Epoch, and Payload. This package does not verify a signed grant, mTLS
// identity, or authenticated caller. Mapping public HTTP/JSON fields directly
// into Request, or exposing the demo CLI as a public service, is unsafe.
package fence
