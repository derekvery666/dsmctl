package loginportal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestDecodersDropSecretsAndIdentity is the WI-070 no-secret-leak acceptance
// criterion on the read path: a reverse-proxy rule may reference a certificate
// and carry custom header values (which can hold an injected auth token), and any
// DSM response may smuggle a SID/SynoToken. The field-whitelist decoders must
// surface a certificate as presence-only, a header set as a count-only, and drop
// SID/SynoToken entirely. Re-encoding each decoded model shows no trace of them.
func TestDecodersDropSecretsAndIdentity(t *testing.T) {
	const certCanary = "CERTKEYCANARY-must-not-survive-decode"
	const headerCanary = "TOKENCANARY-must-not-survive-decode"
	const sidCanary = "SIDCANARY-must-not-survive-decode"
	sidInject := `"sid":"` + sidCanary + `","_sid":"` + sidCanary + `","SynoToken":"` + sidCanary + `","syno_token":"` + sidCanary + `"`

	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		WebDSMAPIName + ".get":         json.RawMessage(`{"http_port":5000,"https_port":5001,"enable_https":true,` + sidInject + `}`),
		WebDSMExternalAPIName + ".get": json.RawMessage(`{"hostname":"dsm.example.com",` + sidInject + `}`),
		AppPortalAPIName + ".list":     json.RawMessage(`{"portal":[{"id":"SYNO.SDS.App.FileStation3.Instance","display_name":"File Station","enable_redirect":false,` + sidInject + `}]}`),
		ReverseProxyAPIName + ".list": json.RawMessage(`{"entries":[{"uuid":"rp-1","description":"media",` +
			`"frontend":{"protocol":"https","fqdn":"media.example.com","port":443,"certificate":"` + certCanary + `","cert_id":"` + certCanary + `"},` +
			`"backend":{"protocol":"http","fqdn":"127.0.0.1","port":8096},` +
			`"customize_headers":[{"name":"Authorization","value":"Bearer ` + headerCanary + `"}],` + sidInject + `}]}`),
	}}

	models := make([]any, 0, 4)
	dsm, _, err := ExecuteDSMWebService(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatal(err)
	}
	models = append(models, dsm)
	external, _, err := ExecuteExternalDomain(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatal(err)
	}
	models = append(models, external)
	portals, _, err := ExecuteApplicationPortals(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatal(err)
	}
	models = append(models, portals)
	rules, _, err := ExecuteReverseProxyRules(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatal(err)
	}
	models = append(models, rules)

	for _, model := range models {
		encoded, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{certCanary, headerCanary, sidCanary, "syno_token", "SynoToken", "_sid", "customize_headers"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("decoded model leaked %q: %s", forbidden, encoded)
			}
		}
	}

	// Sanity: the certificate and headers were still detected as presence/count,
	// and the legitimate fields decoded through the injection.
	rule := rules.Rules[0]
	if !rule.CertificatePresent || rule.CustomHeaderCount != 1 {
		t.Fatalf("presence/count lost: %#v", rule)
	}
	if dsm.HTTPPort != 5000 || !dsm.HTTPSEnabled || external.ExternalHostname != "dsm.example.com" || portals.Portals[0].AppID == "" {
		t.Fatalf("legitimate fields lost: %#v %#v %#v", dsm, external, portals)
	}
}
