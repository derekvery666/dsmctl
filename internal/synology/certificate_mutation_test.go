package synology

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newCertTransferPrep(t *testing.T, server *httptest.Server) transferPrep {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	return transferPrep{
		endpoint:  *parsed,
		apiPath:   "entry.cgi",
		version:   1,
		sid:       "sess-sid-CANARY",
		synoToken: "synotoken-CANARY",
		client:    server.Client(),
	}
}

// TestDoCertificateImportKeyRidesOnlyMultipartBody is the request-capture proof
// that the private key travels solely as a multipart file part — never in the
// URL, the query string, or any header.
func TestDoCertificateImportKeyRidesOnlyMultipartBody(t *testing.T) {
	const keyCanary = "PRIVATE-KEY-CANARY-abc123"
	var (
		capturedURL   string
		capturedQuery string
		headerBlob    string
		keyPartBody   string
		keyPartField  string
		certPartBody  string
		interSeen     bool
		fields        = map[string]string{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		capturedQuery = r.URL.RawQuery
		var headers strings.Builder
		for name, values := range r.Header {
			headers.WriteString(name + ": " + strings.Join(values, ",") + "\n")
		}
		headerBlob = headers.String()

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			http.Error(w, "bad content type", http.StatusBadRequest)
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("next part: %v", err)
				break
			}
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "key":
				keyPartField = part.FormName()
				keyPartBody = string(data)
			case "cert":
				certPartBody = string(data)
			case "inter_cert":
				interSeen = true
			default:
				fields[part.FormName()] = string(data)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"data":{"id":"NewCert1"}}`)
	}))
	defer server.Close()

	prep := newCertTransferPrep(t, server)
	result, err := doCertificateImport(context.Background(), prep, CertificateImportRequest{
		Key:          []byte("-----BEGIN PRIVATE KEY-----\n" + keyCanary + "\n-----END PRIVATE KEY-----\n"),
		Leaf:         []byte("-----BEGIN CERTIFICATE-----\nLEAF\n-----END CERTIFICATE-----\n"),
		Intermediate: []byte("-----BEGIN CERTIFICATE-----\nCHAIN\n-----END CERTIFICATE-----\n"),
		ReplaceID:    "",
		Description:  "prod cert",
		AsDefault:    true,
	})
	if err != nil {
		t.Fatalf("doCertificateImport() error = %v", err)
	}
	if result.CertID != "NewCert1" || !result.AsDefault {
		t.Fatalf("result = %#v", result)
	}

	// The key must be in the key file part...
	if keyPartField != "key" || !strings.Contains(keyPartBody, keyCanary) {
		t.Fatalf("key part missing the private key: field=%q body=%q", keyPartField, keyPartBody)
	}
	if !strings.Contains(certPartBody, "LEAF") || !interSeen {
		t.Fatalf("leaf/intermediate parts missing: cert=%q inter=%v", certPartBody, interSeen)
	}
	// ...and NOWHERE else: not the URL, not the query, not any header.
	if strings.Contains(capturedURL, keyCanary) || strings.Contains(capturedQuery, keyCanary) {
		t.Fatalf("private key leaked into the request URL/query: %q", capturedURL)
	}
	if strings.Contains(headerBlob, keyCanary) {
		t.Fatalf("private key leaked into a request header:\n%s", headerBlob)
	}
	// Metadata fields ride as regular form fields.
	if fields["method"] != "import" || fields["as_default"] != "true" || fields["desc"] != "prod cert" {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestDoCertificateImportAPIErrorSurfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":false,"error":{"code":401}}`)
	}))
	defer server.Close()
	prep := newCertTransferPrep(t, server)
	_, err := doCertificateImport(context.Background(), prep, CertificateImportRequest{Key: []byte("k"), Leaf: []byte("c")})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected API error code 401, got %v", err)
	}
}

// TestDoCertificateExportRedactsSessionTokens proves the export transport masks
// _sid and SynoToken in a transfer error (the redactTransferURL lesson).
func TestDoCertificateExportRedactsSessionTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	prep := newCertTransferPrep(t, server)
	_, err := doCertificateExport(context.Background(), prep, prep.sid, prep.synoToken, "CertId1")
	if err == nil {
		t.Fatal("expected an export error")
	}
	msg := err.Error()
	if strings.Contains(msg, prep.sid) || strings.Contains(msg, prep.synoToken) {
		t.Fatalf("export error leaked session credentials: %s", msg)
	}
	if !strings.Contains(msg, "REDACTED") {
		t.Fatalf("export error did not redact credentials: %s", msg)
	}
}

// TestRepinTLSConfig verifies the re-pin swaps the pinned fingerprint: after
// re-pinning to a new leaf, its handshake state passes and the old leaf fails.
func TestRepinTLSConfig(t *testing.T) {
	newLeaf := selfSignedForPin(t, "new.example.com")
	oldLeaf := selfSignedForPin(t, "old.example.com")
	newFP := sha256.Sum256(newLeaf.Raw)

	base := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test
		VerifyConnection:   func(tls.ConnectionState) error { return nil },
	}
	repinned, err := repinTLSConfig(base, hex.EncodeToString(newFP[:]))
	if err != nil {
		t.Fatalf("repinTLSConfig() error = %v", err)
	}
	if err := repinned.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{newLeaf}}); err != nil {
		t.Fatalf("re-pinned config rejected the new leaf: %v", err)
	}
	if err := repinned.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{oldLeaf}}); err == nil {
		t.Fatal("re-pinned config accepted the old leaf")
	}
	// The base config must not be mutated in place.
	if err := base.VerifyConnection(tls.ConnectionState{}); err != nil {
		t.Fatalf("base config was mutated: %v", err)
	}
	// A malformed fingerprint is rejected.
	if _, err := repinTLSConfig(base, "not-hex"); err == nil {
		t.Fatal("malformed fingerprint accepted")
	}
}

func selfSignedForPin(t *testing.T, cn string) *x509.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
