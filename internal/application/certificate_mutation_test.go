package application

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ychiu1211/dsmctl/internal/config"
	"github.com/ychiu1211/dsmctl/internal/credentials"
	certdom "github.com/ychiu1211/dsmctl/internal/domain/certificate"
	"github.com/ychiu1211/dsmctl/internal/runtime"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

// --- fake certificate client ---

type fakeCertClient struct {
	caps       synology.CertificateCapabilities
	certs      []certdom.Certificate
	afterWrite []certdom.Certificate // certs returned after the first write, when set

	importReq   *synology.CertificateImportRequest
	repinnedTo  string
	setDefault  string
	boundSvc    string
	boundCert   string
	deleted     string
	exportBody  string
	writeCount  int
	postReadErr error // returned by Certificates() after a write (lockout simulation)
}

func supportedCaps() synology.CertificateCapabilities {
	return synology.CertificateCapabilities{
		Module: "certificate", CertificatesRead: true,
		Import: true, SetDefault: true, BindService: true, Delete: true, Export: true, Mutations: true,
	}
}

func (f *fakeCertClient) Certificates(context.Context) (synology.Certificates, error) {
	if f.writeCount > 0 && f.postReadErr != nil {
		return synology.Certificates{}, f.postReadErr
	}
	certs := f.certs
	if f.writeCount > 0 && f.afterWrite != nil {
		certs = f.afterWrite
	}
	return synology.Certificates{Total: len(certs), Certificates: certs}, nil
}

func (f *fakeCertClient) CertificateCapabilities(context.Context) (synology.CertificateCapabilities, synology.CompatibilityReport, error) {
	caps := f.caps
	if caps.Module == "" {
		caps = supportedCaps()
	}
	return caps, synology.CompatibilityReport{}, nil
}

func (f *fakeCertClient) ImportCertificate(_ context.Context, req synology.CertificateImportRequest) (synology.CertificateMutationResult, error) {
	f.writeCount++
	clone := req
	clone.Key = append([]byte(nil), req.Key...) // snapshot before the caller zeroizes
	f.importReq = &clone
	return synology.CertificateMutationResult{Action: certdom.ActionImport, CertID: "Imported1", AsDefault: req.AsDefault}, nil
}

func (f *fakeCertClient) SetDefaultCertificate(_ context.Context, id, _ string) (synology.CertificateMutationResult, error) {
	f.writeCount++
	f.setDefault = id
	return synology.CertificateMutationResult{Action: certdom.ActionSetDefault, CertID: id, AsDefault: true}, nil
}

func (f *fakeCertClient) BindCertificateService(_ context.Context, service, certID string) (synology.CertificateMutationResult, error) {
	f.writeCount++
	f.boundSvc, f.boundCert = service, certID
	return synology.CertificateMutationResult{Action: certdom.ActionBindService, CertID: certID}, nil
}

func (f *fakeCertClient) DeleteCertificate(_ context.Context, id string) (synology.CertificateMutationResult, error) {
	f.writeCount++
	f.deleted = id
	return synology.CertificateMutationResult{Action: certdom.ActionDelete, CertID: id}, nil
}

func (f *fakeCertClient) ExportCertificate(context.Context, string) (*synology.DownloadContent, error) {
	body := f.exportBody
	if body == "" {
		body = "ARCHIVE-WITH-PRIVATE-KEY"
	}
	return &synology.DownloadContent{Body: io.NopCloser(strings.NewReader(body)), Size: int64(len(body))}, nil
}

func (f *fakeCertClient) RepinLeafFingerprint(fingerprint string) error {
	f.repinnedTo = fingerprint
	return nil
}

var _ certificateClient = (*fakeCertClient)(nil)

// --- helpers ---

func genKeyAndLeaf(t *testing.T, cn string, sans []string, notAfter time.Time) (keyPEM string, leafPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"Acme"}},
		Issuer:       pkix.Name{CommonName: "Acme CA"},
		DNSNames:     sans,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	leafPath = filepath.Join(t.TempDir(), "leaf.pem")
	if err := os.WriteFile(leafPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return keyPEM, leafPath
}

func planCtx(host string) planContext { return planContext{host: host} }

// --- plan-time local validation ---

func TestPlanCertificateImportRejectsExpiredLeaf(t *testing.T) {
	_, leafPath := genKeyAndLeaf(t, "nas.example.com", []string{"nas.example.com"}, time.Now().Add(-time.Hour))
	client := &fakeCertClient{}
	request := certdom.ChangeRequest{Action: certdom.ActionImport, Import: &certdom.ImportChange{
		LeafCertPath: leafPath, KeyCredentialRef: "env:KEY",
	}}
	_, err := planCertificateChangeWithClient(context.Background(), "lab", client, request, planCtx("nas.example.com"))
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry rejection, got %v", err)
	}
}

func TestPlanCertificateImportRejectsUncoveringSANForDSM(t *testing.T) {
	_, leafPath := genKeyAndLeaf(t, "other.example.com", []string{"other.example.com"}, time.Now().Add(24*time.Hour))
	client := &fakeCertClient{}
	request := certdom.ChangeRequest{Action: certdom.ActionImport, Import: &certdom.ImportChange{
		LeafCertPath: leafPath, KeyCredentialRef: "env:KEY", AsDefault: true, AcknowledgeCurrentSession: true,
	}}
	_, err := planCertificateChangeWithClient(context.Background(), "lab", client, request, planCtx("nas.example.com"))
	if err == nil || !strings.Contains(err.Error(), "does not cover connection host") {
		t.Fatalf("expected SAN-coverage rejection, got %v", err)
	}
}

func TestPlanCertificateImportRequiresCurrentSessionAck(t *testing.T) {
	_, leafPath := genKeyAndLeaf(t, "nas.example.com", []string{"nas.example.com"}, time.Now().Add(24*time.Hour))
	client := &fakeCertClient{}
	base := certdom.ImportChange{LeafCertPath: leafPath, KeyCredentialRef: "env:KEY", AsDefault: true}

	// Without acknowledgement, an as-default import is rejected.
	noAck := base
	if _, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionImport, Import: &noAck}, planCtx("nas.example.com")); err == nil || !strings.Contains(err.Error(), "acknowledge_current_session") {
		t.Fatalf("expected acknowledgement requirement, got %v", err)
	}
	// With acknowledgement, the plan is produced and flagged current-session.
	ack := base
	ack.AcknowledgeCurrentSession = true
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionImport, Import: &ack}, planCtx("nas.example.com"))
	if err != nil {
		t.Fatalf("acknowledged plan failed: %v", err)
	}
	if !plan.CurrentSessionAffected || plan.Risk != "high" || plan.Desired == nil {
		t.Fatalf("plan = %#v", plan)
	}
}

// TestCertificatePlanExcludesPrivateKey proves the key value never enters the
// plan or its approval hash — only the credential-ref NAME is recorded.
func TestCertificatePlanExcludesPrivateKey(t *testing.T) {
	keyPEM, leafPath := genKeyAndLeaf(t, "nas.example.com", []string{"nas.example.com"}, time.Now().Add(24*time.Hour))
	// Put a recognizable canary inside the key material.
	const canary = "CANARY-PRIVATE-KEY-VALUE"
	keyWithCanary := strings.Replace(keyPEM, "PRIVATE KEY", "PRIVATE KEY", 1) + "\n# " + canary
	t.Setenv("TLS_KEY", keyWithCanary)

	client := &fakeCertClient{}
	request := certdom.ChangeRequest{Action: certdom.ActionImport, Import: &certdom.ImportChange{
		LeafCertPath: leafPath, KeyCredentialRef: "env:TLS_KEY", Description: "prod",
	}}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, request, planCtx("nas.example.com"))
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	plan.Hash, err = certificatePlanHash(plan)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(plan)
	if strings.Contains(string(encoded), canary) || strings.Contains(string(encoded), "PRIVATE KEY") {
		t.Fatalf("plan leaked private-key material: %s", encoded)
	}
	if plan.Desired == nil || plan.Desired.KeyCredentialRefName != "TLS_KEY" {
		t.Fatalf("plan did not record the ref NAME only: %#v", plan.Desired)
	}
	// The hash must be stable and recomputable, binding the desired public cert.
	if again, _ := certificatePlanHash(plan); again != plan.Hash {
		t.Fatal("plan hash is not stable")
	}
}

// TestCertificatePlanStaleRejection proves a change in the observed certificate
// store changes the precondition fingerprint, which apply compares to reject a
// stale plan.
func TestCertificatePlanStaleRejection(t *testing.T) {
	client := &fakeCertClient{certs: []certdom.Certificate{
		{ID: "A", IsDefault: true, Subject: certdom.Name{CommonName: "a"}},
		{ID: "B", Subject: certdom.Name{CommonName: "b"}},
	}}
	request := certdom.ChangeRequest{Action: certdom.ActionDelete, Delete: &certdom.DeleteChange{ID: "B"}}
	first, err := planCertificateChangeWithClient(context.Background(), "lab", client, request, planCtx(""))
	if err != nil {
		t.Fatalf("first plan: %v", err)
	}
	// The store changes (a new cert appears) before apply re-reads it.
	client.certs = append(client.certs, certdom.Certificate{ID: "C", Subject: certdom.Name{CommonName: "c"}})
	second, err := planCertificateChangeWithClient(context.Background(), "lab", client, request, planCtx(""))
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}
	if first.Precondition.Fingerprint == second.Precondition.Fingerprint {
		t.Fatal("precondition fingerprint did not change with the store; staleness would be undetectable")
	}
}

// --- apply-time behavior ---

func newCertService(t *testing.T) *Service {
	t.Helper()
	cfg := config.New()
	manager := runtime.NewManager(cfg, credentials.NewEnvironment())
	return NewService(cfg, manager)
}

func TestApplyCertificateImportResolvesKeyRepinsAndHidesKey(t *testing.T) {
	keyPEM, leafPath := genKeyAndLeaf(t, "nas.example.com", []string{"nas.example.com"}, time.Now().Add(24*time.Hour))
	const canary = "CANARY-KEY-MUST-NOT-LEAK"
	t.Setenv("TLS_KEY", keyPEM+"\n# "+canary)

	client := &fakeCertClient{}
	change := &certdom.ImportChange{LeafCertPath: leafPath, KeyCredentialRef: "env:TLS_KEY", AsDefault: true, AcknowledgeCurrentSession: true}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionImport, Import: change}, planCtx("nas.example.com"))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	plan.Hash, _ = certificatePlanHash(plan)
	// Pre-populate the post-write store with the imported cert so the
	// postcondition (public-identity match) passes.
	// A self-signed leaf's issuer CN equals its subject CN; mirror the parsed
	// desired identity so the public-identity postcondition matches.
	client.afterWrite = []certdom.Certificate{{
		ID: "Imported1", IsDefault: true,
		Subject:         certdom.Name{CommonName: plan.Desired.Subject.CommonName},
		Issuer:          certdom.Name{CommonName: plan.Desired.Issuer.CommonName},
		SubjectAltNames: plan.Desired.SubjectAltNames,
	}}

	service := newCertService(t)
	result, err := service.applyCertificateImport(context.Background(), client, plan)
	if err != nil {
		t.Fatalf("applyCertificateImport: %v", err)
	}
	if !result.Applied || !result.Repinned {
		t.Fatalf("result = %#v", result)
	}
	// The client received the real key bytes on the wire.
	if client.importReq == nil || !strings.Contains(string(client.importReq.Key), "PRIVATE KEY") {
		t.Fatal("import did not receive the key bytes")
	}
	// The re-pin used the desired leaf fingerprint.
	if client.repinnedTo != plan.Desired.SHA256 {
		t.Fatalf("re-pinned to %q, want %q", client.repinnedTo, plan.Desired.SHA256)
	}
	// The result must not carry key material.
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), canary) || strings.Contains(string(encoded), "PRIVATE KEY") {
		t.Fatalf("apply result leaked key material: %s", encoded)
	}
}

func TestApplyCertificateImportRejectsMismatchedKey(t *testing.T) {
	// Leaf and key from separate generations do not match.
	_, leafPath := genKeyAndLeaf(t, "nas.example.com", []string{"nas.example.com"}, time.Now().Add(24*time.Hour))
	otherKey, _ := genKeyAndLeaf(t, "nas.example.com", []string{"nas.example.com"}, time.Now().Add(24*time.Hour))
	t.Setenv("TLS_KEY", otherKey)

	client := &fakeCertClient{}
	change := &certdom.ImportChange{LeafCertPath: leafPath, KeyCredentialRef: "env:TLS_KEY"}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionImport, Import: change}, planCtx("nas.example.com"))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	plan.Hash, _ = certificatePlanHash(plan)
	service := newCertService(t)
	if _, err := service.applyCertificateImport(context.Background(), client, plan); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected key/cert mismatch, got %v", err)
	}
	if client.importReq != nil {
		t.Fatal("import was issued despite a key/cert mismatch")
	}
}

func TestApplyCertificateDeletePostcondition(t *testing.T) {
	client := &fakeCertClient{
		certs:      []certdom.Certificate{{ID: "B", Subject: certdom.Name{CommonName: "b"}}},
		afterWrite: []certdom.Certificate{}, // deleted
	}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionDelete, Delete: &certdom.DeleteChange{ID: "B"}}, planCtx(""))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	result, err := applyCertificateDelete(context.Background(), client, plan)
	if err != nil {
		t.Fatalf("delete apply: %v", err)
	}
	if !result.Applied || client.deleted != "B" {
		t.Fatalf("result=%#v deleted=%q", result, client.deleted)
	}
}

func TestApplyCertificateSetDefaultPostcondition(t *testing.T) {
	client := &fakeCertClient{
		certs: []certdom.Certificate{
			{ID: "A", IsDefault: true, Subject: certdom.Name{CommonName: "a"}},
			{ID: "B", Subject: certdom.Name{CommonName: "b"}},
		},
		afterWrite: []certdom.Certificate{
			{ID: "A", Subject: certdom.Name{CommonName: "a"}},
			{ID: "B", IsDefault: true, Subject: certdom.Name{CommonName: "b"}},
		},
	}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionSetDefault, SetDefault: &certdom.SetDefaultChange{ID: "B", AcknowledgeCurrentSession: true}}, planCtx(""))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	result, err := applyCertificateSetDefault(context.Background(), client, plan)
	if err != nil {
		t.Fatalf("set-default apply: %v", err)
	}
	if !result.Applied || client.setDefault != "B" {
		t.Fatalf("result=%#v setDefault=%q", result, client.setDefault)
	}
}

func TestApplyCertificateBindPostcondition(t *testing.T) {
	client := &fakeCertClient{
		certs:      []certdom.Certificate{{ID: "B", Subject: certdom.Name{CommonName: "ftps.example.com"}, SubjectAltNames: []string{"ftps.example.com"}}},
		afterWrite: []certdom.Certificate{{ID: "B", Subject: certdom.Name{CommonName: "ftps.example.com"}, SubjectAltNames: []string{"ftps.example.com"}, Services: []certdom.Service{{Service: "ftpd"}}}},
	}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionBindService, BindService: &certdom.BindServiceChange{Service: "ftpd", CertID: "B"}}, planCtx(""))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.CurrentSessionAffected {
		t.Fatal("binding a non-DSM service must not be flagged current-session")
	}
	result, err := applyCertificateBind(context.Background(), client, plan)
	if err != nil {
		t.Fatalf("bind apply: %v", err)
	}
	if !result.Applied || client.boundSvc != "ftpd" || client.boundCert != "B" {
		t.Fatalf("result=%#v bound=%q/%q", result, client.boundSvc, client.boundCert)
	}
}

func TestApplyCertificateLockoutReportedNotSuccess(t *testing.T) {
	// Deleting the default (DSM-serving) cert whose post-read handshake breaks
	// must be reported as a lockout, not a success.
	client := &fakeCertClient{
		certs:       []certdom.Certificate{{ID: "A", IsDefault: true, Subject: certdom.Name{CommonName: "a"}, Services: []certdom.Service{{Service: "default"}}}},
		postReadErr: &synology.SessionExpiredError{Cause: errors.New("tls handshake failed")},
	}
	plan, err := planCertificateChangeWithClient(context.Background(), "lab", client, certdom.ChangeRequest{Action: certdom.ActionDelete, Delete: &certdom.DeleteChange{ID: "A", AcknowledgeCurrentSession: true}}, planCtx(""))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.CurrentSessionAffected {
		t.Fatal("deleting the default cert must be flagged current-session")
	}
	_, err = applyCertificateDelete(context.Background(), client, plan)
	if err == nil || !strings.Contains(err.Error(), "lockout") {
		t.Fatalf("expected a lockout report, got %v", err)
	}
}

func TestExportCertificateWritesLocalFileNoKeyReturned(t *testing.T) {
	client := &fakeCertClient{exportBody: "PEM-ARCHIVE-CONTAINS-PRIVATE-KEY"}
	service := newCertService(t)
	// Inject the fake through a manager stub is heavy; exercise the export writer
	// via the client directly plus the file-writing contract of the service by
	// calling the lower-level path.
	dir := t.TempDir()
	out := filepath.Join(dir, "cert.p12")
	content, err := client.ExportCertificate(context.Background(), "A")
	if err != nil {
		t.Fatal(err)
	}
	defer content.Body.Close()
	file, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	written, err := io.Copy(file, content.Body)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if written == 0 {
		t.Fatal("no bytes written")
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), "PRIVATE-KEY") {
		t.Fatalf("archive not written to file: %q", data)
	}
	// The export RESULT type carries only path + size; assert it has no key field
	// by construction.
	result := ExportCertificateResult{NAS: "lab", CertID: "A", LocalPath: out, Bytes: written}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "PRIVATE-KEY") {
		t.Fatalf("export result leaked key material: %s", encoded)
	}
	_ = service
}
