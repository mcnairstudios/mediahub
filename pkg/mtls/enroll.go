package mtls

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EnrollResult struct {
	Cert        string `json:"cert"`
	Key         string `json:"key"`
	CA          string `json:"ca"`
	Email       string `json:"email"`
	Fingerprint string `json:"fingerprint"`
}

func Enroll(serverURL, token string) (*EnrollResult, error) {
	enrollURL := strings.TrimRight(serverURL, "/")
	if idx := strings.Index(enrollURL, "/playlist.m3u"); idx > 0 {
		enrollURL = enrollURL[:idx]
	}
	enrollURL += "/enroll"

	body, _ := json.Marshal(map[string]string{"token": token})

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Post(enrollURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("enrollment request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid or expired enrollment token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enrollment failed with status %d", resp.StatusCode)
	}

	var result EnrollResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding enrollment response: %w", err)
	}
	return &result, nil
}

func certDir(dataDir, sourceID string) string {
	return filepath.Join(dataDir, "certs", sourceID)
}

func SaveCerts(dataDir, sourceID string, result *EnrollResult) error {
	dir := certDir(dataDir, sourceID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "client.crt"), []byte(result.Cert), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "client.key"), []byte(result.Key), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), []byte(result.CA), 0600); err != nil {
		return err
	}
	return nil
}

func HasCerts(dataDir, sourceID string) bool {
	_, err := os.Stat(filepath.Join(certDir(dataDir, sourceID), "client.crt"))
	return err == nil
}

func LoadTLSConfig(dataDir, sourceID string) (*tls.Config, error) {
	dir := certDir(dataDir, sourceID)

	cert, err := tls.LoadX509KeyPair(
		filepath.Join(dir, "client.crt"),
		filepath.Join(dir, "client.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("loading client cert: %w", err)
	}

	caPEM, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("loading CA cert: %w", err)
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}, nil
}

func HTTPClient(dataDir, sourceID string) (*http.Client, error) {
	tlsCfg, err := LoadTLSConfig(dataDir, sourceID)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}, nil
}

func DeleteCerts(dataDir, sourceID string) error {
	return os.RemoveAll(certDir(dataDir, sourceID))
}

func Fingerprint(dataDir, sourceID string) string {
	dir := certDir(dataDir, sourceID)

	cert, err := tls.LoadX509KeyPair(
		filepath.Join(dir, "client.crt"),
		filepath.Join(dir, "client.key"),
	)
	if err != nil || len(cert.Certificate) == 0 {
		return ""
	}

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%X", parsed.SerialNumber)
}
