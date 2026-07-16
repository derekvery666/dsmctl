package synology

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

func TestEncodeJSONParametersUsesTLSForSecretTransport(t *testing.T) {
	client, err := NewClient(Options{BaseURL: "https://nas.example.test", Username: "admin", Password: "login-secret"})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	parameters, err := client.encodeJSONParametersLocked(context.Background(), map[string]any{
		"name": "automation", "password": "user-secret", "notify_by_email": false,
	}, []string{"password"})
	if err != nil {
		t.Fatalf("encodeJSONParametersLocked() error = %v", err)
	}
	if got := parameters.Get("name"); got != `"automation"` {
		t.Errorf("name = %q", got)
	}
	if got := parameters.Get("password"); got != `"user-secret"` {
		t.Errorf("password = %q", got)
	}
	if got := parameters.Get("notify_by_email"); got != "false" {
		t.Errorf("notify_by_email = %q", got)
	}
}

func TestEncodeJSONParametersEncryptsSecretsWithoutTLS(t *testing.T) {
	const publicKey = "MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEArIrNKvuUX0T0FOndhej5lhCwibLOpxtrnUlYUiAkub2/EJRyRZxjC0AJYnsMZJsxSF+Bq83sTrodL2lfOOpQxFfgjaYK0He7MThtRHu6RNE3FS1vSuQIZpSmxJltZZnApxy9j/hJ2ESzwWaPqv9uIvL98C9UY7/OemeeWkhN9jnqR9uFIdjn1xVWRCBAYf5LwdKjNhnHjhgo0ymn15Dz/84gsIq3NQMgJHzD8FycU4PhbAMAplbcCQlw1z2PbKA8hYPA3cRLsLMvICtj1Btl1siABH2IIyEAs8/z8gKqwi5cNinIoT2ijdKmqsRrCd0dyH2MeOUTkKKGsqPv2JdyFy0uzBIkINQDoVConEXceT5B0grFYp9sxu1xKW0BnU8V+NQxFazxnsKjkquDPAwgatuGFKi5KvyQMIqWLFCdUub/9JxxMIfTJI+H0PwNeSvtt/uMEV0urm4G/C/jdqlRtGVqPmtaUUh8E71jqT7wU+dZwpN2AxlVIuRnu9WvwUSOUXqO32QlQEMgVAxgpiJWaIKsO9k8SK0F1a+Pl95t0Gbpfw05qJS1BAb2+zs74NU+sSC/bEGkuJ7Hrj8+hkA8tku/HxtCy27s8aipyOrE5WL4khUmozhOczuwZ0pxwpTfZDyjEZJDRyqfsPuhreOb2hy2i4/01jUbpTJRS4xjGO0CAwEAAQ=="
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
		}
		if r.Form.Get("api") != encryptionAPIName || r.Form.Get("method") != "getinfo" || r.Form.Get("format") != "module" {
			t.Errorf("unexpected encryption request: %#v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"cipherkey":"__cIpHeRtExT","ciphertoken":"__cIpHeRtOkEn","public_key":%q,"server_time":1784190000}}`, publicKey)
	}))
	defer server.Close()

	client, err := NewClient(Options{BaseURL: server.URL, Username: "admin", Password: "login-secret", HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client.target.SetAPI(encryptionAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 2})
	client.apiChecked[encryptionAPIName] = true
	parameters, err := client.encodeJSONParametersLocked(context.Background(), map[string]any{
		"name": "automation", "password": "user-secret",
	}, []string{"password"})
	if err != nil {
		t.Fatalf("encodeJSONParametersLocked() error = %v", err)
	}
	if parameters.Has("password") || strings.Contains(parameters.Encode(), "user-secret") {
		t.Fatalf("plaintext secret leaked into parameters: %s", parameters.Encode())
	}
	if got := parameters.Get("name"); got != `"automation"` {
		t.Errorf("name = %q", got)
	}
	bundle := parameters.Get("__cIpHeRtExT")
	if !strings.Contains(bundle, `"rsa"`) || !strings.Contains(bundle, `"aes"`) {
		t.Fatalf("encrypted bundle = %q", bundle)
	}
}

func TestCryptoJSPassphraseEncryptCompatibilityVector(t *testing.T) {
	plaintext := []byte("__cIpHeRtOkEn=1784190000&password=%22Aa1%21%22")
	got, err := cryptoJSPassphraseEncrypt(bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7}), plaintext, []byte("transport-key"))
	if err != nil {
		t.Fatalf("cryptoJSPassphraseEncrypt() error = %v", err)
	}
	const want = "U2FsdGVkX18AAQIDBAUGB9FdGoQJH0yp3Br9yhbbSa6bleO/7xkeF69hiuk0K3qGA3YR09PR8tSFoPqL1wAnhQ=="
	if got != want {
		t.Fatalf("ciphertext = %q, want %q", got, want)
	}
}
