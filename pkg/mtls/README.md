# pkg/mtls

Client-side mTLS enrollment and certificate management for tvproxy-streams sources.

## Flow

1. tvproxy-streams server creates a one-time enrollment token
2. Client calls `Enroll(serverURL, token)` which POSTs to `/enroll`
3. Server returns client cert, private key, and CA certificate
4. Client saves certs to `{DataDir}/certs/{sourceID}/`
5. All subsequent requests use `HTTPClient()` with mTLS

## Enrollment

During enrollment, `InsecureSkipVerify` is used because the CA is not yet known. After enrollment completes and certs are saved, `LoadTLSConfig` creates a proper TLS config with CA verification.

## Certificate Storage

```
{DataDir}/certs/{sourceID}/
  client.crt   (0600)
  client.key   (0600)
  ca.crt       (0600)
```

Directory created with 0700 permissions.
