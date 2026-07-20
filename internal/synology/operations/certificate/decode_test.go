package certificate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestDecodeDropsInjectedKeyMaterial upgrades the no-private-key guarantee from
// structural to test-enforced (WI-065 acceptance criterion). A malicious or
// buggy DSM response that smuggles key/private_key fields into a CRT.list entry
// must have that material dropped by the public-field whitelist decoder: it
// never reaches the domain model, so re-encoding the model shows no trace of it.
func TestDecodeDropsInjectedKeyMaterial(t *testing.T) {
	const canary = "KEYCANARY-must-not-survive-decode"
	executor := &capturingExecutor{response: json.RawMessage(`{
		"certificates": [
			{"id":"AbCdEf","desc":"injected","is_default":true,
			 "key":"` + canary + `",
			 "private_key":"-----BEGIN PRIVATE KEY-----` + canary + `-----END PRIVATE KEY-----",
			 "privateKey":"` + canary + `",
			 "key_pem":"` + canary + `",
			 "issuer":{"common_name":"Acme CA","private_key":"` + canary + `"},
			 "subject":{"common_name":"nas.example.com","sub_alt_name":["nas.example.com"],"key":"` + canary + `"},
			 "valid_from":"Mar 15 15:49:37 2026 GMT","valid_till":"Mar 16 15:49:37 2027 GMT",
			 "services":[{"service":"default","display_name":"DSM","key":"` + canary + `"}]}
		]
	}`)}

	certs, _, err := ExecuteCertificates(context.Background(), certTarget(), executor)
	if err != nil {
		t.Fatalf("ExecuteCertificates() error = %v", err)
	}
	if len(certs.Certificates) != 1 || certs.Certificates[0].ID != "AbCdEf" {
		t.Fatalf("decoded certs = %#v", certs.Certificates)
	}

	// Re-encode the whole decoded model and assert not a single byte of the
	// injected key material survived into any field.
	encoded, err := json.Marshal(certs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), canary) {
		t.Fatalf("decoded model carried injected key material: %s", encoded)
	}
	for _, forbidden := range []string{"private_key", "privateKey", "key_pem"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("decoded model exposes a key-bearing field %q: %s", forbidden, encoded)
		}
	}
}
