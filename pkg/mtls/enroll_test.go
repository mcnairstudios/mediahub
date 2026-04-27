package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveCertsAndHasCerts(t *testing.T) {
	dir := t.TempDir()
	sourceID := "test-source-123"

	if HasCerts(dir, sourceID) {
		t.Fatal("should not have certs yet")
	}

	result := &EnrollResult{
		Cert: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n",
		Key:  "-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n",
		CA:   "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n",
	}

	if err := SaveCerts(dir, sourceID, result); err != nil {
		t.Fatal(err)
	}

	if !HasCerts(dir, sourceID) {
		t.Fatal("should have certs after save")
	}

	certPath := filepath.Join(dir, "certs", sourceID, "client.crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != result.Cert {
		t.Errorf("cert content mismatch")
	}

	keyPath := filepath.Join(dir, "certs", sourceID, "client.key")
	data, err = os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != result.Key {
		t.Errorf("key content mismatch")
	}

	caPath := filepath.Join(dir, "certs", sourceID, "ca.crt")
	data, err = os.ReadFile(caPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != result.CA {
		t.Errorf("CA content mismatch")
	}
}

func TestDeleteCerts(t *testing.T) {
	dir := t.TempDir()
	sourceID := "delete-test"

	result := &EnrollResult{Cert: "cert", Key: "key", CA: "ca"}
	if err := SaveCerts(dir, sourceID, result); err != nil {
		t.Fatal(err)
	}

	if !HasCerts(dir, sourceID) {
		t.Fatal("should have certs")
	}

	if err := DeleteCerts(dir, sourceID); err != nil {
		t.Fatal(err)
	}

	if HasCerts(dir, sourceID) {
		t.Fatal("should not have certs after delete")
	}
}

func TestHasCertsNonexistent(t *testing.T) {
	dir := t.TempDir()
	if HasCerts(dir, "nonexistent") {
		t.Fatal("should not have certs for nonexistent source")
	}
}

func TestCertDirPermissions(t *testing.T) {
	dir := t.TempDir()
	sourceID := "perms-test"

	result := &EnrollResult{Cert: "cert", Key: "key", CA: "ca"}
	if err := SaveCerts(dir, sourceID, result); err != nil {
		t.Fatal(err)
	}

	certDir := filepath.Join(dir, "certs", sourceID)
	info, err := os.Stat(certDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("expected dir permissions 0700, got %o", info.Mode().Perm())
	}

	keyInfo, err := os.Stat(filepath.Join(certDir, "client.key"))
	if err != nil {
		t.Fatal(err)
	}
	if keyInfo.Mode().Perm() != 0600 {
		t.Fatalf("expected key permissions 0600, got %o", keyInfo.Mode().Perm())
	}
}

func TestLoadTLSConfig(t *testing.T) {
	dir := t.TempDir()
	sourceID := "tls-test"

	caCert, caKey := generateTestCA(t)
	clientCert, clientKey := generateTestClientCert(t, caCert, caKey)

	result := &EnrollResult{
		Cert: string(clientCert),
		Key:  string(clientKey),
		CA:   string(caCert),
	}

	if err := SaveCerts(dir, sourceID, result); err != nil {
		t.Fatal(err)
	}

	tlsCfg, err := LoadTLSConfig(dir, sourceID)
	if err != nil {
		t.Fatalf("LoadTLSConfig failed: %v", err)
	}

	if len(tlsCfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
	if tlsCfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be set")
	}
}

func TestHTTPClient(t *testing.T) {
	dir := t.TempDir()
	sourceID := "client-test"

	caCert, caKey := generateTestCA(t)
	clientCert, clientKey := generateTestClientCert(t, caCert, caKey)

	result := &EnrollResult{
		Cert: string(clientCert),
		Key:  string(clientKey),
		CA:   string(caCert),
	}

	if err := SaveCerts(dir, sourceID, result); err != nil {
		t.Fatal(err)
	}

	client, err := HTTPClient(dir, sourceID)
	if err != nil {
		t.Fatalf("HTTPClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 60*time.Second {
		t.Fatalf("expected 60s timeout, got %v", client.Timeout)
	}
}

func TestHTTPClientMissingCerts(t *testing.T) {
	dir := t.TempDir()
	_, err := HTTPClient(dir, "missing")
	if err == nil {
		t.Fatal("expected error for missing certs")
	}
}

func TestEnroll(t *testing.T) {
	expected := &EnrollResult{
		Cert:        "-----BEGIN CERTIFICATE-----\nclient\n-----END CERTIFICATE-----\n",
		Key:         "-----BEGIN EC PRIVATE KEY-----\nkey\n-----END EC PRIVATE KEY-----\n",
		CA:          "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n",
		Fingerprint: "AB:CD:EF",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enroll" {
			t.Fatalf("expected /enroll path, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["token"] != "test-token-123" {
			t.Fatalf("expected token test-token-123, got %s", body["token"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer ts.Close()

	result, err := Enroll(ts.URL, "test-token-123")
	if err != nil {
		t.Fatalf("Enroll failed: %v", err)
	}
	if result.Cert != expected.Cert {
		t.Fatalf("cert mismatch")
	}
	if result.Key != expected.Key {
		t.Fatalf("key mismatch")
	}
	if result.CA != expected.CA {
		t.Fatalf("CA mismatch")
	}
	if result.Fingerprint != expected.Fingerprint {
		t.Fatalf("fingerprint mismatch")
	}
}

func TestEnrollStripsPlaylistPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(&EnrollResult{Cert: "c", Key: "k", CA: "ca"})
	}))
	defer ts.Close()

	_, err := Enroll(ts.URL+"/playlist.m3u", "token")
	if err != nil {
		t.Fatalf("Enroll failed: %v", err)
	}
	if gotPath != "/enroll" {
		t.Fatalf("expected /enroll, got %s", gotPath)
	}
}

func TestEnrollInvalidToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	_, err := Enroll(ts.URL, "bad-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if err.Error() != "invalid or expired enrollment token" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnrollServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := Enroll(ts.URL, "token")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestFingerprint(t *testing.T) {
	dir := t.TempDir()
	sourceID := "fp-test"

	caCert, caKey := generateTestCA(t)
	clientCert, clientKey := generateTestClientCert(t, caCert, caKey)

	result := &EnrollResult{
		Cert: string(clientCert),
		Key:  string(clientKey),
		CA:   string(caCert),
	}

	if err := SaveCerts(dir, sourceID, result); err != nil {
		t.Fatal(err)
	}

	fp := Fingerprint(dir, sourceID)
	if fp == "" {
		t.Fatal("expected non-empty fingerprint")
	}
}

func TestFingerprintMissingCerts(t *testing.T) {
	dir := t.TempDir()
	fp := Fingerprint(dir, "missing")
	if fp != "" {
		t.Fatalf("expected empty fingerprint, got %s", fp)
	}
}

func generateTestCA(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

func generateTestClientCert(t *testing.T, caCertPEM, caKeyPEM []byte) (certPEM, keyPEM []byte) {
	t.Helper()

	caBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}
