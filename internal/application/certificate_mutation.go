package application

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ychiu1211/dsmctl/internal/domain/certificate"
	"github.com/ychiu1211/dsmctl/internal/synology"
)

const certificateAPIVersion = "dsmctl.io/v1alpha1"

// CertificatePlan is a validated, hash-bound certificate mutation plan. Every
// certificate write is high risk. The plan is safe to persist: it carries only
// public certificate metadata and the NAME of the private-key credential
// reference — never the key value, which is resolved to bytes solely at apply.
type CertificatePlan struct {
	APIVersion             string                          `json:"api_version" jsonschema:"Plan schema version"`
	NAS                    string                          `json:"nas" jsonschema:"NAS profile selected during planning"`
	ProfileRevision        uint64                          `json:"profile_revision,omitempty" jsonschema:"Persistent gateway profile revision selected during planning"`
	Request                certificate.ChangeRequest       `json:"request" jsonschema:"Validated certificate mutation intent"`
	Desired                *certificate.DesiredCertificate `json:"desired,omitempty" jsonschema:"Public fingerprint of the certificate an import will install (import only)"`
	Precondition           certificate.Precondition        `json:"precondition" jsonschema:"Observed certificate + binding state that must still match during apply"`
	Destructive            bool                            `json:"destructive" jsonschema:"Whether the plan removes or replaces an existing certificate"`
	Risk                   string                          `json:"risk" jsonschema:"Plan risk level; every certificate write is high"`
	CurrentSessionAffected bool                            `json:"current_session_affected" jsonschema:"Whether the plan changes the certificate serving the current dsmctl session"`
	Warnings               []string                        `json:"warnings" jsonschema:"Data-loss, TLS-lockout, and exposure warnings"`
	Summary                []string                        `json:"summary" jsonschema:"Human-readable actions the plan will perform"`
	Hash                   string                          `json:"hash" jsonschema:"SHA-256 approval hash covering intent, desired cert, and observed state"`
}

// CertificateApplyResult is the outcome of applying a certificate plan.
type CertificateApplyResult struct {
	NAS       string                             `json:"nas" jsonschema:"NAS profile used for apply"`
	PlanHash  string                             `json:"plan_hash" jsonschema:"Approved plan hash"`
	Applied   bool                               `json:"applied" jsonschema:"Whether DSM accepted the change and postcondition verification passed"`
	Repinned  bool                               `json:"repinned,omitempty" jsonschema:"Whether the client re-pinned to the new leaf fingerprint for the post-apply re-read"`
	Operation synology.CertificateMutationResult `json:"operation" jsonschema:"Normalized DSM mutation result (no key material)"`
}

// planContext carries the per-profile facts (connection host + pinned
// fingerprint) the plan builder needs to reproduce the current-session decision
// and the SAN-coverage validation deterministically at both plan and apply time.
type planContext struct {
	host              string
	pinnedFingerprint string // lowercase hex, no colons; empty when not pinned
}

func (s *Service) PlanCertificateChange(ctx context.Context, requestedNAS string, request certificate.ChangeRequest) (CertificatePlan, error) {
	if err := validateCertificateChange(request); err != nil {
		return CertificatePlan{}, err
	}
	name, client, err := s.certificateClient(ctx, requestedNAS)
	if err != nil {
		return CertificatePlan{}, err
	}
	pctx, err := s.certificatePlanContext(ctx, name)
	if err != nil {
		return CertificatePlan{}, err
	}
	plan, err := planCertificateChangeWithClient(ctx, name, client, request, pctx)
	if err != nil {
		return CertificatePlan{}, err
	}
	plan.ProfileRevision, err = s.profileRevision(ctx, name)
	if err != nil {
		return CertificatePlan{}, err
	}
	plan.Hash, err = certificatePlanHash(plan)
	return plan, err
}

func (s *Service) ApplyCertificatePlan(ctx context.Context, plan CertificatePlan, approvalHash string) (CertificateApplyResult, error) {
	if err := validateCertificatePlan(plan, approvalHash); err != nil {
		return CertificateApplyResult{}, err
	}
	if err := s.authorizeRemoteApply(ctx, plan.NAS, plan.ProfileRevision, plan.Hash, plan.Risk); err != nil {
		return CertificateApplyResult{}, err
	}
	if err := s.verifyProfileRevision(ctx, plan.NAS, plan.ProfileRevision); err != nil {
		return CertificateApplyResult{}, err
	}
	name, client, err := s.certificateClient(ctx, plan.NAS)
	if err != nil {
		return CertificateApplyResult{}, err
	}
	if name != plan.NAS {
		return CertificateApplyResult{}, fmt.Errorf("certificate plan NAS %q resolved to different profile %q", plan.NAS, name)
	}
	pctx, err := s.certificatePlanContext(ctx, plan.NAS)
	if err != nil {
		return CertificateApplyResult{}, err
	}
	// Merge into fresh state and reject a plan whose observed precondition or hash
	// no longer matches — DSM may have changed the certificate store since planning.
	current, err := planCertificateChangeWithClient(ctx, plan.NAS, client, plan.Request, pctx)
	if err != nil {
		return CertificateApplyResult{}, fmt.Errorf("certificate plan precondition no longer holds: %w", err)
	}
	current.ProfileRevision = plan.ProfileRevision
	current.Hash, err = certificatePlanHash(current)
	if err != nil {
		return CertificateApplyResult{}, err
	}
	if current.Precondition.Fingerprint != plan.Precondition.Fingerprint || current.Hash != plan.Hash {
		return CertificateApplyResult{}, fmt.Errorf("certificate plan is stale; create a new plan")
	}

	switch plan.Request.Action {
	case certificate.ActionImport:
		return s.applyCertificateImport(ctx, client, plan)
	case certificate.ActionSetDefault:
		return applyCertificateSetDefault(ctx, client, plan)
	case certificate.ActionBindService:
		return applyCertificateBind(ctx, client, plan)
	case certificate.ActionDelete:
		return applyCertificateDelete(ctx, client, plan)
	default:
		return CertificateApplyResult{}, fmt.Errorf("unsupported certificate action %q", plan.Request.Action)
	}
}

// applyCertificateImport resolves the key ONLY here, runs the full local
// validation (including the key/cert match), streams the bundle, zeroizes the
// key, re-pins to the new leaf when the current session is affected, and
// postcondition-re-reads.
func (s *Service) applyCertificateImport(ctx context.Context, client certificateClient, plan CertificatePlan) (CertificateApplyResult, error) {
	change := plan.Request.Import
	leafPEM, err := os.ReadFile(change.LeafCertPath)
	if err != nil {
		return CertificateApplyResult{}, fmt.Errorf("read leaf certificate %q: %w", change.LeafCertPath, err)
	}
	var interPEM []byte
	if change.IntermediatePath != "" {
		interPEM, err = os.ReadFile(change.IntermediatePath)
		if err != nil {
			return CertificateApplyResult{}, fmt.Errorf("read intermediate chain %q: %w", change.IntermediatePath, err)
		}
	}
	// Resolve the private key to bytes ONLY now, at apply time. It never touched
	// the plan, the hash, or any log line; it lives in keyPEM until the import
	// returns and is zeroized immediately after.
	secret, err := s.secretReferences.ResolveSecret(ctx, change.KeyCredentialRef)
	if err != nil {
		return CertificateApplyResult{}, fmt.Errorf("resolve certificate key reference: %w", err)
	}
	keyPEM := []byte(secret)
	defer zeroize(keyPEM)

	// Full local validation before the NAS is touched: the key must match the
	// leaf (the only check that needs the key bytes), the leaf must be unexpired,
	// and the chain must link. SAN coverage was validated at plan time.
	if err := certificate.ValidateKeyMatchesLeaf(keyPEM, leafPEM); err != nil {
		return CertificateApplyResult{}, err
	}

	operation, err := client.ImportCertificate(ctx, synology.CertificateImportRequest{
		Key:          keyPEM,
		Leaf:         leafPEM,
		Intermediate: interPEM,
		ReplaceID:    change.ReplaceID,
		Description:  change.Description,
		AsDefault:    change.AsDefault,
	})
	if err != nil {
		return CertificateApplyResult{}, authenticationError(plan.NAS, err)
	}

	result := CertificateApplyResult{NAS: plan.NAS, PlanHash: plan.Hash, Operation: operation}

	// Current-session protection: replacing the DSM-serving certificate changes
	// the leaf dsmctl is pinned to, so the post-apply re-read cannot ride the old
	// pinned connection. Re-pin to the new leaf's fingerprint (known locally from
	// the imported PEM) before re-reading rather than treating the pinning break
	// as failure.
	if plan.CurrentSessionAffected && plan.Desired != nil {
		if err := client.RepinLeafFingerprint(plan.Desired.SHA256); err != nil {
			return CertificateApplyResult{}, fmt.Errorf("re-pin to new leaf after import: %w", err)
		}
		result.Repinned = true
	}

	if err := verifyCertificatePostcondition(ctx, client, plan); err != nil {
		return CertificateApplyResult{}, lockoutOrError(plan, err)
	}
	result.Applied = true
	return result, nil
}

func applyCertificateSetDefault(ctx context.Context, client certificateClient, plan CertificatePlan) (CertificateApplyResult, error) {
	change := plan.Request.SetDefault
	operation, err := client.SetDefaultCertificate(ctx, change.ID, "")
	if err != nil {
		return CertificateApplyResult{}, authenticationError(plan.NAS, err)
	}
	if err := verifyCertificatePostcondition(ctx, client, plan); err != nil {
		return CertificateApplyResult{}, lockoutOrError(plan, err)
	}
	return CertificateApplyResult{NAS: plan.NAS, PlanHash: plan.Hash, Applied: true, Operation: operation}, nil
}

func applyCertificateBind(ctx context.Context, client certificateClient, plan CertificatePlan) (CertificateApplyResult, error) {
	change := plan.Request.BindService
	operation, err := client.BindCertificateService(ctx, change.Service, change.CertID)
	if err != nil {
		return CertificateApplyResult{}, authenticationError(plan.NAS, err)
	}
	if err := verifyCertificatePostcondition(ctx, client, plan); err != nil {
		return CertificateApplyResult{}, lockoutOrError(plan, err)
	}
	return CertificateApplyResult{NAS: plan.NAS, PlanHash: plan.Hash, Applied: true, Operation: operation}, nil
}

func applyCertificateDelete(ctx context.Context, client certificateClient, plan CertificatePlan) (CertificateApplyResult, error) {
	change := plan.Request.Delete
	operation, err := client.DeleteCertificate(ctx, change.ID)
	if err != nil {
		return CertificateApplyResult{}, authenticationError(plan.NAS, err)
	}
	if err := verifyCertificatePostcondition(ctx, client, plan); err != nil {
		return CertificateApplyResult{}, lockoutOrError(plan, err)
	}
	return CertificateApplyResult{NAS: plan.NAS, PlanHash: plan.Hash, Applied: true, Operation: operation}, nil
}

// lockoutOrError reports a broken-and-unrecoverable handshake after a
// current-session-affecting change as a lockout rather than a plain failure.
func lockoutOrError(plan CertificatePlan, err error) error {
	if plan.CurrentSessionAffected && synology.IsSessionExpired(err) {
		return fmt.Errorf("certificate applied but the current dsmctl session can no longer reach %q over TLS (possible lockout); re-establish trust and verify the certificate manually: %w", plan.NAS, err)
	}
	return err
}

func planCertificateChangeWithClient(ctx context.Context, nas string, client certificateClient, request certificate.ChangeRequest, pctx planContext) (CertificatePlan, error) {
	capabilities, _, err := client.CertificateCapabilities(ctx)
	if err != nil {
		return CertificatePlan{}, authenticationError(nas, err)
	}
	if !certificateActionSupported(capabilities, request.Action) {
		return CertificatePlan{}, fmt.Errorf("NAS %q does not expose a verified certificate backend for %q", nas, request.Action)
	}
	current, err := client.Certificates(ctx)
	if err != nil {
		return CertificatePlan{}, authenticationError(nas, err)
	}
	precondition := buildCertificatePrecondition(current)

	plan := CertificatePlan{
		APIVersion:   certificateAPIVersion,
		NAS:          nas,
		Request:      request,
		Precondition: precondition,
		Risk:         "high", // every certificate write is high risk
		Warnings:     []string{},
	}

	switch request.Action {
	case certificate.ActionImport:
		if err := buildImportPlan(&plan, current, request.Import, pctx); err != nil {
			return CertificatePlan{}, err
		}
	case certificate.ActionSetDefault:
		if err := buildSetDefaultPlan(&plan, current, request.SetDefault); err != nil {
			return CertificatePlan{}, err
		}
	case certificate.ActionBindService:
		if err := buildBindPlan(&plan, current, request.BindService, pctx); err != nil {
			return CertificatePlan{}, err
		}
	case certificate.ActionDelete:
		if err := buildDeletePlan(&plan, current, request.Delete); err != nil {
			return CertificatePlan{}, err
		}
	default:
		return CertificatePlan{}, fmt.Errorf("unsupported certificate action %q", request.Action)
	}

	plan.Hash, err = certificatePlanHash(plan)
	if err != nil {
		return CertificatePlan{}, err
	}
	return plan, nil
}

func buildImportPlan(plan *CertificatePlan, current synology.Certificates, change *certificate.ImportChange, pctx planContext) error {
	leafPEM, err := os.ReadFile(change.LeafCertPath)
	if err != nil {
		return fmt.Errorf("read leaf certificate %q: %w", change.LeafCertPath, err)
	}
	leaf, err := certificate.ParseLeaf(leafPEM)
	if err != nil {
		return err
	}
	hasIntermediate := change.IntermediatePath != ""
	if hasIntermediate {
		interPEM, err := os.ReadFile(change.IntermediatePath)
		if err != nil {
			return fmt.Errorf("read intermediate chain %q: %w", change.IntermediatePath, err)
		}
		chain, err := certificate.ParseIntermediates(interPEM)
		if err != nil {
			return err
		}
		if err := certificate.ValidateChain(leaf, chain); err != nil {
			return err
		}
	}
	// The leaf must be valid now — an expired or not-yet-valid cert is a
	// plan-time error, not a silent apply that bricks TLS.
	if err := certificate.ValidateNotExpired(leaf, time.Now()); err != nil {
		return err
	}

	refName := strings.TrimPrefix(strings.TrimSpace(change.KeyCredentialRef), "env:")
	desired := certificate.DesiredFromLeaf(leaf, refName, hasIntermediate)
	plan.Desired = &desired

	affected, replacing := findObservedCertByID(current, change.ReplaceID)
	dsmServing := (replacing && certificateServesCurrentSession(affected, pctx))
	plan.CurrentSessionAffected = change.AsDefault || dsmServing
	plan.Destructive = replacing

	// SAN coverage is required only when the imported cert will serve the DSM
	// desktop (as-default or replacing the DSM-serving cert).
	if plan.CurrentSessionAffected {
		if err := certificate.ValidateSANCoversHost(leaf, pctx.host); err != nil {
			return err
		}
	}
	if plan.CurrentSessionAffected && !change.AcknowledgeCurrentSession {
		return errCurrentSessionAck("import")
	}
	if replacing {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("replaces the certificate with id %q; services bound to it will present the new leaf", change.ReplaceID))
	}
	if plan.CurrentSessionAffected {
		plan.Warnings = append(plan.Warnings, "changes the certificate serving the DSM desktop; dsmctl re-pins to the new leaf, but a mismatched or unreachable leaf can lock this session out")
	}
	verb := "install a new certificate"
	if replacing {
		verb = fmt.Sprintf("replace certificate %q", change.ReplaceID)
	}
	def := ""
	if change.AsDefault {
		def = " and make it the default"
	}
	plan.Summary = append(plan.Summary, fmt.Sprintf("%s%s (leaf %s, SHA-256 %s)", verb, def, leaf.Subject.CommonName, desired.SHA256))
	return nil
}

func buildSetDefaultPlan(plan *CertificatePlan, current synology.Certificates, change *certificate.SetDefaultChange) error {
	target, found := findObservedCertByID(current, change.ID)
	if !found {
		return fmt.Errorf("certificate %q does not exist", change.ID)
	}
	if target.IsDefault {
		return fmt.Errorf("certificate %q is already the default", change.ID)
	}
	// Making a different certificate the default changes what the DSM desktop
	// presents — it always affects the current session.
	plan.CurrentSessionAffected = true
	if !change.AcknowledgeCurrentSession {
		return errCurrentSessionAck("set_default")
	}
	plan.Warnings = append(plan.Warnings, "changes the default certificate the DSM desktop presents; this can interrupt the current session")
	plan.Summary = append(plan.Summary, fmt.Sprintf("set certificate %q (%s) as the default", change.ID, target.SubjectCN))
	return nil
}

func buildBindPlan(plan *CertificatePlan, current synology.Certificates, change *certificate.BindServiceChange, pctx planContext) error {
	cert, found := findCertByID(current, change.CertID)
	if !found {
		return fmt.Errorf("certificate %q does not exist", change.CertID)
	}
	isDSM := isDSMService(change.Service)
	plan.CurrentSessionAffected = isDSM
	if isDSM {
		if err := certificate.ValidateNamesCoverHost(cert.SubjectAltNames, cert.Subject.CommonName, pctx.host); err != nil {
			return err
		}
		if !change.AcknowledgeCurrentSession {
			return errCurrentSessionAck("bind_service")
		}
		plan.Warnings = append(plan.Warnings, "binds the DSM desktop service; this can interrupt the current session")
	}
	plan.Summary = append(plan.Summary, fmt.Sprintf("bind service %q to certificate %q (%s)", change.Service, change.CertID, cert.Subject.CommonName))
	return nil
}

func buildDeletePlan(plan *CertificatePlan, current synology.Certificates, change *certificate.DeleteChange) error {
	target, found := findObservedCertByID(current, change.ID)
	if !found {
		return fmt.Errorf("certificate %q does not exist", change.ID)
	}
	plan.Destructive = true
	plan.CurrentSessionAffected = target.IsDefault || observedServesDSM(target)
	if plan.CurrentSessionAffected && !change.AcknowledgeCurrentSession {
		return errCurrentSessionAck("delete")
	}
	plan.Warnings = append(plan.Warnings, "certificate deletion is permanent; services bound to it lose their certificate")
	if plan.CurrentSessionAffected {
		plan.Warnings = append(plan.Warnings, "deletes the certificate serving the DSM desktop; this can lock the current session out")
	}
	plan.Summary = append(plan.Summary, fmt.Sprintf("delete certificate %q (%s)", change.ID, target.SubjectCN))
	return nil
}

func verifyCertificatePostcondition(ctx context.Context, client certificateClient, plan CertificatePlan) error {
	state, err := client.Certificates(ctx)
	if err != nil {
		return fmt.Errorf("re-read certificates after apply: %w", err)
	}
	switch plan.Request.Action {
	case certificate.ActionImport:
		// DSM's CRT list does not return a DER fingerprint, so verify the desired
		// certificate's PUBLIC identity (subject CN + issuer CN + the SAN set)
		// appears in the store. Full DER re-verification would require export
		// (which extracts the key) and is a live-verification follow-up.
		if plan.Desired == nil {
			return nil
		}
		for _, cert := range state.Certificates {
			if certMatchesDesired(cert, *plan.Desired) {
				return nil
			}
		}
		return fmt.Errorf("imported certificate (subject %q) was not found in the store after apply", plan.Desired.Subject.CommonName)
	case certificate.ActionSetDefault:
		for _, cert := range state.Certificates {
			if cert.ID == plan.Request.SetDefault.ID {
				if !cert.IsDefault {
					return fmt.Errorf("certificate %q is not the default after apply", cert.ID)
				}
				return nil
			}
		}
		return fmt.Errorf("certificate %q was not found after apply", plan.Request.SetDefault.ID)
	case certificate.ActionBindService:
		change := plan.Request.BindService
		for _, cert := range state.Certificates {
			if cert.ID != change.CertID {
				continue
			}
			for _, svc := range cert.Services {
				if svc.Service == change.Service {
					return nil
				}
			}
		}
		return fmt.Errorf("service %q is not bound to certificate %q after apply", change.Service, change.CertID)
	case certificate.ActionDelete:
		for _, cert := range state.Certificates {
			if cert.ID == plan.Request.Delete.ID {
				return fmt.Errorf("certificate %q still exists after delete", cert.ID)
			}
		}
		return nil
	}
	return nil
}

// ExportCertificateResult reports a completed certificate export.
type ExportCertificateResult struct {
	NAS       string `json:"nas" jsonschema:"NAS profile used for the request"`
	CertID    string `json:"cert_id" jsonschema:"Exported certificate id"`
	LocalPath string `json:"local_path" jsonschema:"Local file the archive was written to"`
	Bytes     int64  `json:"bytes" jsonschema:"Bytes written to the local file"`
}

// ExportCertificate writes a certificate archive to a caller-named LOCAL FILE.
// The archive contains the private key, so no key bytes are ever returned to the
// caller — only the local path and size. It is intentionally NOT a plan/apply
// operation (it does not mutate the NAS) but IS stripped from the read-only
// gateway because it exfiltrates secret material.
func (s *Service) ExportCertificate(ctx context.Context, requestedNAS, certID, localPath string) (ExportCertificateResult, error) {
	if strings.TrimSpace(certID) == "" {
		return ExportCertificateResult{}, fmt.Errorf("certificate id is required")
	}
	if strings.TrimSpace(localPath) == "" {
		return ExportCertificateResult{}, fmt.Errorf("a local output path is required")
	}
	name, client, err := s.certificateClient(ctx, requestedNAS)
	if err != nil {
		return ExportCertificateResult{}, err
	}
	content, err := client.ExportCertificate(ctx, certID)
	if err != nil {
		return ExportCertificateResult{}, authenticationError(name, err)
	}
	defer content.Body.Close()
	file, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return ExportCertificateResult{}, fmt.Errorf("create local export file %q: %w", localPath, err)
	}
	written, copyErr := io.Copy(file, content.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return ExportCertificateResult{}, fmt.Errorf("write certificate archive to %q: %w", localPath, copyErr)
	}
	if closeErr != nil {
		return ExportCertificateResult{}, fmt.Errorf("finalize certificate archive %q: %w", localPath, closeErr)
	}
	return ExportCertificateResult{NAS: name, CertID: certID, LocalPath: localPath, Bytes: written}, nil
}

// --- helpers ---

func (s *Service) certificatePlanContext(ctx context.Context, name string) (planContext, error) {
	cfg, err := s.configSnapshot(ctx)
	if err != nil {
		return planContext{}, err
	}
	profile, ok := cfg.NAS[name]
	if !ok {
		return planContext{}, fmt.Errorf("NAS profile %q is no longer configured", name)
	}
	host := ""
	if parsed, perr := url.Parse(profile.URL); perr == nil {
		host = parsed.Hostname()
	}
	pinned := ""
	if profile.TLSMode == "pinned_fingerprint" {
		pinned = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(profile.CertificateFingerprint), ":", ""))
	}
	return planContext{host: host, pinnedFingerprint: pinned}, nil
}

func buildCertificatePrecondition(state synology.Certificates) certificate.Precondition {
	observed := make([]certificate.ObservedCertificate, 0, len(state.Certificates))
	for _, cert := range state.Certificates {
		services := make([]string, 0, len(cert.Services))
		for _, svc := range cert.Services {
			services = append(services, svc.Service)
		}
		sort.Strings(services)
		observed = append(observed, certificate.ObservedCertificate{
			ID:            cert.ID,
			IsDefault:     cert.IsDefault,
			Services:      services,
			ValidTillUnix: cert.ValidTillUnix,
			SubjectCN:     cert.Subject.CommonName,
			IssuerCN:      cert.Issuer.CommonName,
		})
	}
	sort.Slice(observed, func(i, j int) bool { return observed[i].ID < observed[j].ID })
	precondition := certificate.Precondition{Certificates: observed}
	precondition.Fingerprint = fingerprint(observed)
	return precondition
}

func findObservedCertByID(state synology.Certificates, id string) (certificate.ObservedCertificate, bool) {
	if id == "" {
		return certificate.ObservedCertificate{}, false
	}
	for _, cert := range state.Certificates {
		if cert.ID == id {
			services := make([]string, 0, len(cert.Services))
			for _, svc := range cert.Services {
				services = append(services, svc.Service)
			}
			return certificate.ObservedCertificate{
				ID:            cert.ID,
				IsDefault:     cert.IsDefault,
				Services:      services,
				ValidTillUnix: cert.ValidTillUnix,
				SubjectCN:     cert.Subject.CommonName,
				IssuerCN:      cert.Issuer.CommonName,
			}, true
		}
	}
	return certificate.ObservedCertificate{}, false
}

func findCertByID(state synology.Certificates, id string) (certificate.Certificate, bool) {
	for _, cert := range state.Certificates {
		if cert.ID == id {
			return cert, true
		}
	}
	return certificate.Certificate{}, false
}

// certificateServesCurrentSession reports whether an observed certificate serves
// the current dsmctl session: it is the default, it is bound to the DSM desktop
// service, or (when the profile is pinned) it is the pinned leaf. DSM's list has
// no DER fingerprint, so the pin comparison is best-effort via the identity
// signals; the pinned-fingerprint case is a live-verification refinement.
func certificateServesCurrentSession(cert certificate.ObservedCertificate, _ planContext) bool {
	return cert.IsDefault || observedServesDSM(cert)
}

func observedServesDSM(cert certificate.ObservedCertificate) bool {
	for _, svc := range cert.Services {
		if isDSMService(svc) {
			return true
		}
	}
	return false
}

func isDSMService(service string) bool {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "default", "dsm", "webui":
		return true
	default:
		return false
	}
}

func certMatchesDesired(cert certificate.Certificate, desired certificate.DesiredCertificate) bool {
	if cert.Subject.CommonName != desired.Subject.CommonName {
		return false
	}
	if cert.Issuer.CommonName != desired.Issuer.CommonName {
		return false
	}
	return sameStringSet(cert.SubjectAltNames, desired.SubjectAltNames)
}

func certificateActionSupported(capabilities synology.CertificateCapabilities, action string) bool {
	switch action {
	case certificate.ActionImport:
		return capabilities.Import
	case certificate.ActionSetDefault:
		return capabilities.SetDefault
	case certificate.ActionBindService:
		return capabilities.BindService
	case certificate.ActionDelete:
		return capabilities.Delete
	default:
		return false
	}
}

func errCurrentSessionAck(action string) error {
	return fmt.Errorf("%s changes the certificate serving the current dsmctl session; set acknowledge_current_session to proceed", action)
}

func certificatePlanHash(plan CertificatePlan) (string, error) {
	plan.Hash = ""
	return hashJSON(plan)
}

func validateCertificatePlan(plan CertificatePlan, approvalHash string) error {
	if strings.TrimSpace(approvalHash) == "" || approvalHash != plan.Hash {
		return fmt.Errorf("approval hash does not match the certificate plan")
	}
	if plan.APIVersion != certificateAPIVersion || strings.TrimSpace(plan.NAS) == "" {
		return fmt.Errorf("invalid certificate plan metadata")
	}
	if err := validateCertificateChange(plan.Request); err != nil {
		return err
	}
	expected, err := certificatePlanHash(plan)
	if err != nil {
		return err
	}
	if expected != plan.Hash {
		return fmt.Errorf("certificate plan contents were modified after planning")
	}
	return nil
}

func validateCertificateChange(request certificate.ChangeRequest) error {
	switch request.Action {
	case certificate.ActionImport:
		change := request.Import
		if change == nil {
			return fmt.Errorf("import payload is required")
		}
		if strings.TrimSpace(change.LeafCertPath) == "" {
			return fmt.Errorf("import leaf_cert_path is required")
		}
		if !strings.HasPrefix(strings.TrimSpace(change.KeyCredentialRef), "env:") {
			return fmt.Errorf("import key_credential_ref must be an env:NAME reference, not a literal key")
		}
		return nil
	case certificate.ActionSetDefault:
		if request.SetDefault == nil || strings.TrimSpace(request.SetDefault.ID) == "" {
			return fmt.Errorf("set_default id is required")
		}
		return nil
	case certificate.ActionBindService:
		change := request.BindService
		if change == nil || strings.TrimSpace(change.Service) == "" || strings.TrimSpace(change.CertID) == "" {
			return fmt.Errorf("bind_service requires service and cert_id")
		}
		return nil
	case certificate.ActionDelete:
		if request.Delete == nil || strings.TrimSpace(request.Delete.ID) == "" {
			return fmt.Errorf("delete id is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported certificate action %q", request.Action)
	}
}

// zeroize overwrites a secret byte slice in place so the key material does not
// linger in the heap after the import completes.
func zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
