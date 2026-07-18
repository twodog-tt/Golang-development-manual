package frostcluster

import (
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

type AuthMethod string

const (
	AuthMethodMTLS          AuthMethod = "mtls"
	AuthMethodLoopbackToken AuthMethod = "loopback-token"
)

type Identity struct {
	Name   string
	Method AuthMethod
}

// Authenticator prefers a verified mTLS client identity. Loopback bearer
// tokens are an explicit test/local-development escape hatch: they are rejected
// when the TCP peer is not a loopback address and are not a production
// replacement for mTLS or a workload-identity system.
type Authenticator struct {
	// MTLSIdentities optionally maps a certificate URI SAN or Common Name to a
	// FROST application identity. With an empty map, the certificate identity
	// is used unchanged.
	MTLSIdentities map[string]string

	// LoopbackTokens maps application identity to bearer token.
	LoopbackTokens map[string]string
}

func (a Authenticator) Authenticate(r *http.Request) (Identity, error) {
	if identity, ok := certificateIdentity(r.TLS); ok {
		if len(a.MTLSIdentities) > 0 {
			mapped, exists := a.MTLSIdentities[identity]
			if !exists {
				return Identity{}, errors.New("mTLS identity is not authorized")
			}
			identity = mapped
		}
		if identity == "" {
			return Identity{}, errors.New("mTLS identity is empty")
		}
		return Identity{Name: identity, Method: AuthMethodMTLS}, nil
	}

	if len(a.LoopbackTokens) == 0 {
		return Identity{}, errors.New("verified client certificate is required")
	}
	if !isLoopbackRemote(r.RemoteAddr) {
		return Identity{}, errors.New("loopback token authentication is restricted to loopback peers")
	}
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return Identity{}, errors.New("bearer token is required")
	}
	presented := strings.TrimPrefix(header, prefix)
	for identity, expected := range a.LoopbackTokens {
		if len(presented) == len(expected) &&
			subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1 {
			return Identity{Name: identity, Method: AuthMethodLoopbackToken}, nil
		}
	}
	return Identity{}, errors.New("invalid bearer token")
}

func certificateIdentity(state *tls.ConnectionState) (string, bool) {
	if state == nil || len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
		return "", false
	}
	cert := state.PeerCertificates[0]
	if len(cert.URIs) > 0 {
		return cert.URIs[0].String(), true
	}
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName, true
	}
	return "", false
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// LoadIdentityTokens reads a JSON object of identity -> token. The file must
// not be accessible by group or other users.
func LoadIdentityTokens(path string) (map[string]string, error) {
	data, err := readPrivateFile(path, 64<<10)
	if err != nil {
		return nil, err
	}
	var tokens map[string]string
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("decode token file: %w", err)
	}
	if len(tokens) == 0 {
		return nil, errors.New("token file is empty")
	}
	for identity, token := range tokens {
		if identity == "" || len(token) < 32 {
			return nil, fmt.Errorf("identity %q has an invalid token", identity)
		}
	}
	return tokens, nil
}

// LoadIdentityMap reads certificate identity -> application identity mappings.
// Certificate identities are URI SAN strings when present, otherwise Common
// Names. Mapping SPIFFE URI SANs to short FROST party IDs avoids treating
// certificate naming conventions as polynomial participant identifiers.
func LoadIdentityMap(path string) (map[string]string, error) {
	data, err := readPrivateFile(path, 64<<10)
	if err != nil {
		return nil, err
	}
	var identities map[string]string
	if err := json.Unmarshal(data, &identities); err != nil {
		return nil, fmt.Errorf("decode identity map: %w", err)
	}
	if len(identities) == 0 {
		return nil, errors.New("identity map is empty")
	}
	for certificateIdentity, applicationIdentity := range identities {
		if certificateIdentity == "" || applicationIdentity == "" {
			return nil, errors.New("identity map contains an empty identity")
		}
	}
	return identities, nil
}

// LoadSecret reads one opaque token from a mode-0600 file.
func LoadSecret(path string) (string, error) {
	data, err := readPrivateFile(path, 64<<10)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	if len(value) < 32 {
		return "", errors.New("secret must contain at least 32 characters")
	}
	return value, nil
}

// LoadShareEncryptionKey accepts either exactly 32 raw bytes or 64
// hexadecimal characters (with an optional trailing newline) from a private
// file.
func LoadShareEncryptionKey(path string) ([]byte, error) {
	data, err := readPrivateFile(path, 128)
	if err != nil {
		return nil, err
	}
	if len(data) == 32 {
		return append([]byte(nil), data...), nil
	}
	text := strings.TrimSpace(string(data))
	if len(text) != 64 || text != strings.ToLower(text) {
		return nil, errors.New("share encryption key must be 32 raw bytes or 64 lowercase hexadecimal characters")
	}
	key, err := hex.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("decode share encryption key: %w", err)
	}
	return key, nil
}

func readPrivateFile(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%q permissions must not grant group/other access", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%q exceeds %d bytes", path, maxBytes)
	}
	return data, nil
}

func loadCertPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("CA file contains no usable certificates")
	}
	return pool, nil
}

// ServerTLSConfig builds a server configuration that requires and verifies a
// client certificate.
func ServerTLSConfig(clientCAPath string) (*tls.Config, error) {
	pool, err := loadCertPool(clientCAPath)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,
	}, nil
}

// ClientTLSConfig builds a TLS 1.3 client configuration for coordinator or
// participant control-plane calls.
func ClientTLSConfig(certPath, keyPath, serverCAPath, serverName string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client key pair: %w", err)
	}
	pool, err := loadCertPool(serverCAPath)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   serverName,
	}, nil
}
