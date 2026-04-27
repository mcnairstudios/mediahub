# pkg/mtls Interfaces

## Functions

### Enroll(serverURL, token string) (*EnrollResult, error)
POST enrollment token to server's `/enroll` endpoint. Returns cert, key, CA, and fingerprint. Uses `InsecureSkipVerify` during enrollment since the CA is not yet known.

### SaveCerts(dataDir, sourceID string, result *EnrollResult) error
Write cert, key, and CA files to `{dataDir}/certs/{sourceID}/`. Directory 0700, files 0600.

### HasCerts(dataDir, sourceID string) bool
Check if client.crt exists for the given source.

### LoadTLSConfig(dataDir, sourceID string) (*tls.Config, error)
Load cert+key pair and CA pool into a `*tls.Config` for use with HTTP clients. Uses proper CA verification (no InsecureSkipVerify).

### HTTPClient(dataDir, sourceID string) (*http.Client, error)
Create an `*http.Client` configured with mTLS from saved certs. 60-second timeout.

### DeleteCerts(dataDir, sourceID string) error
Remove the entire cert directory for a source.

### Fingerprint(dataDir, sourceID string) string
Extract the serial number from the client certificate as a hex string. Returns empty string if certs don't exist or can't be parsed.

## Types

### EnrollResult
```go
type EnrollResult struct {
    Cert        string `json:"cert"`
    Key         string `json:"key"`
    CA          string `json:"ca"`
    Email       string `json:"email"`
    Fingerprint string `json:"fingerprint"`
}
```
