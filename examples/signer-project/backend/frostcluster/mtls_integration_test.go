package frostcluster

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testPKI struct {
	caCert           string
	serverCert       string
	serverKey        string
	operatorCert     string
	operatorKey      string
	unauthorizedCert string
	unauthorizedKey  string
	operatorURI      string
}

func TestMTLSTLS13HandshakeAndIdentityMapping(t *testing.T) {
	pki := createTestPKI(t)
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Authenticator: Authenticator{
			MTLSIdentities: map[string]string{
				pki.operatorURI: testAdminIdentity,
			},
		},
		AdminIdentities: map[string]bool{testAdminIdentity: true},
		SessionTTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	serverTLS, err := ServerTLSConfig(pki.caCert)
	if err != nil {
		t.Fatal(err)
	}
	serverCertificate, err := tls.LoadX509KeyPair(pki.serverCert, pki.serverKey)
	if err != nil {
		t.Fatal(err)
	}
	serverTLS.Certificates = []tls.Certificate{serverCertificate}

	server := httptest.NewUnstartedServer(coordinator.Handler())
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	operatorTLS, err := ClientTLSConfig(
		pki.operatorCert,
		pki.operatorKey,
		pki.caCert,
		"coordinator.test",
	)
	if err != nil {
		t.Fatal(err)
	}
	operatorTransport := &http.Transport{TLSClientConfig: operatorTLS}
	defer operatorTransport.CloseIdleConnections()
	operatorHTTP := &http.Client{
		Transport: operatorTransport,
		Timeout:   5 * time.Second,
	}

	response, err := operatorHTTP.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d, want 200", response.StatusCode)
	}
	var negotiatedVersion uint16
	if response.TLS != nil {
		negotiatedVersion = response.TLS.Version
	}
	if negotiatedVersion != tls.VersionTLS13 {
		t.Fatalf("TLS version=%#x, want TLS 1.3", negotiatedVersion)
	}

	client, err := NewCoordinatorClient(CoordinatorClientConfig{
		BaseURL:    server.URL,
		HTTPClient: operatorHTTP,
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.CreateSession(ctx, SessionSpec{
		ID:        sessionID,
		Kind:      SessionKindDKG,
		KeyID:     "mtls-key",
		Parties:   []string{"alice", "bob", "carol"},
		Threshold: 1,
	}); err != nil {
		t.Fatalf("create session over mTLS: %v", err)
	}

	rootPool := x509.NewCertPool()
	caPEM, err := os.ReadFile(pki.caCert)
	if err != nil {
		t.Fatal(err)
	}
	if !rootPool.AppendCertsFromPEM(caPEM) {
		t.Fatal("append test CA")
	}
	noCertificateTransport := &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13,
		RootCAs:    rootPool,
		ServerName: "coordinator.test",
	}}
	defer noCertificateTransport.CloseIdleConnections()
	noCertificateClient := &http.Client{
		Transport: noCertificateTransport,
		Timeout:   5 * time.Second,
	}
	if _, err := noCertificateClient.Get(server.URL + "/healthz"); err == nil {
		t.Fatal("server accepted a TLS client without a certificate")
	}

	unauthorizedTLS, err := ClientTLSConfig(
		pki.unauthorizedCert,
		pki.unauthorizedKey,
		pki.caCert,
		"coordinator.test",
	)
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedTransport := &http.Transport{TLSClientConfig: unauthorizedTLS}
	defer unauthorizedTransport.CloseIdleConnections()
	unauthorizedClient, err := NewCoordinatorClient(CoordinatorClientConfig{
		BaseURL: server.URL,
		HTTPClient: &http.Client{
			Transport: unauthorizedTransport,
			Timeout:   5 * time.Second,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedSessionID, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	_, err = unauthorizedClient.CreateSession(ctx, SessionSpec{
		ID:        unauthorizedSessionID,
		Kind:      SessionKindDKG,
		KeyID:     "unauthorized-key",
		Parties:   []string{"alice", "bob", "carol"},
		Threshold: 1,
	})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized certificate error=%v, want HTTP 401", err)
	}
}

func createTestPKI(t *testing.T) testPKI {
	t.Helper()
	dir := t.TempDir()
	now := time.Now().UTC()

	caKey := newTestECKey(t)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "FROST test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER := createTestCertificate(t, caTemplate, caTemplate, &caKey.PublicKey, caKey)

	serverKey := newTestECKey(t)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "coordinator.test"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"coordinator.test"},
	}
	serverDER := createTestCertificate(t, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)

	operatorURI, err := url.Parse("spiffe://signer.test/control/operator")
	if err != nil {
		t.Fatal(err)
	}
	operatorKey := newTestECKey(t)
	operatorTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "ignored-operator-cn"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{operatorURI},
	}
	operatorDER := createTestCertificate(t, operatorTemplate, caTemplate, &operatorKey.PublicKey, caKey)

	unauthorizedURI, err := url.Parse("spiffe://signer.test/control/mallory")
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedKey := newTestECKey(t)
	unauthorizedTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject:      pkix.Name{CommonName: "mallory"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{unauthorizedURI},
	}
	unauthorizedDER := createTestCertificate(
		t,
		unauthorizedTemplate,
		caTemplate,
		&unauthorizedKey.PublicKey,
		caKey,
	)

	result := testPKI{
		caCert:           filepath.Join(dir, "ca.pem"),
		serverCert:       filepath.Join(dir, "server.pem"),
		serverKey:        filepath.Join(dir, "server-key.pem"),
		operatorCert:     filepath.Join(dir, "operator.pem"),
		operatorKey:      filepath.Join(dir, "operator-key.pem"),
		unauthorizedCert: filepath.Join(dir, "unauthorized.pem"),
		unauthorizedKey:  filepath.Join(dir, "unauthorized-key.pem"),
		operatorURI:      operatorURI.String(),
	}
	writeTestCertificateFile(t, result.caCert, caDER)
	writeTestCertificateFile(t, result.serverCert, serverDER)
	writeTestECKeyFile(t, result.serverKey, serverKey)
	writeTestCertificateFile(t, result.operatorCert, operatorDER)
	writeTestECKeyFile(t, result.operatorKey, operatorKey)
	writeTestCertificateFile(t, result.unauthorizedCert, unauthorizedDER)
	writeTestECKeyFile(t, result.unauthorizedKey, unauthorizedKey)
	return result
}

func newTestECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func createTestCertificate(
	t *testing.T,
	template, parent *x509.Certificate,
	publicKey any,
	parentKey any,
) []byte {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func writeTestCertificateFile(t *testing.T, path string, der []byte) {
	t.Helper()
	writeTestPEMFile(t, path, "CERTIFICATE", der)
}

func writeTestECKeyFile(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	writeTestPEMFile(t, path, "EC PRIVATE KEY", der)
}

func writeTestPEMFile(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	raw := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
