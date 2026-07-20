package certificate

// This file models the guarded, high-risk certificate WRITE surface: import a
// bring-your-own certificate bundle, set the default certificate, bind a service
// to a certificate, and delete a certificate. Every write rides the hash-bound
// plan/apply contract. Private-key material is NEVER modeled here: the imported
// key is referenced by a credential_ref (env:NAME) and resolved to bytes only at
// apply time in the synology layer — it never enters any type in this package,
// the plan, the approval hash, the result, or any log line. The leaf and
// intermediate certificates are public and their parsed metadata is recorded.

// Write actions. Export is intentionally not an action here — it is a
// key-extracting local transfer modeled outside the plan/apply flow.
const (
	ActionImport      = "import"
	ActionSetDefault  = "set_default"
	ActionBindService = "bind_service"
	ActionDelete      = "delete"
)

// ChangeRequest is a single typed certificate mutation intent. Exactly one
// payload matching Action is populated. It is deliberately a small set of
// intents rather than a generic SYNO.Core.Certificate proxy.
type ChangeRequest struct {
	Action      string             `json:"action" jsonschema:"Certificate write action: import, set_default, bind_service, or delete"`
	Import      *ImportChange      `json:"import,omitempty" jsonschema:"Import a certificate bundle (leaf + private key + optional intermediate)"`
	SetDefault  *SetDefaultChange  `json:"set_default,omitempty" jsonschema:"Make an installed certificate the default DSM presents"`
	BindService *BindServiceChange `json:"bind_service,omitempty" jsonschema:"Bind a DSM service to an installed certificate"`
	Delete      *DeleteChange      `json:"delete,omitempty" jsonschema:"Delete an installed certificate"`
}

// ImportChange imports a bring-your-own certificate. The leaf and optional
// intermediate chain are LOCAL FILE PATHS to PEM files (public, read at plan and
// apply time). The private key is a credential reference, never a path or a
// literal, and is resolved to bytes only at apply time.
type ImportChange struct {
	// LeafCertPath is a local path to the PEM-encoded leaf certificate (public).
	LeafCertPath string `json:"leaf_cert_path" jsonschema:"Local path to the PEM-encoded leaf certificate (public material)"`
	// IntermediatePath is an optional local path to the PEM-encoded intermediate
	// chain (public). One or more concatenated PEM blocks.
	IntermediatePath string `json:"intermediate_path,omitempty" jsonschema:"Optional local path to the PEM-encoded intermediate chain (public material)"`
	// KeyCredentialRef names the environment variable holding the private-key PEM
	// as env:NAME. Only the NAME is ever recorded; the value is resolved to bytes
	// exclusively at apply time and zeroized immediately after the upload.
	KeyCredentialRef string `json:"key_credential_ref" jsonschema:"Private-key reference as env:NAME; the value is resolved only at apply time and never stored, hashed, or logged"`
	// ReplaceID, when set, replaces the existing certificate with this id;
	// empty installs a new certificate.
	ReplaceID string `json:"replace_id,omitempty" jsonschema:"Existing certificate id to replace; empty installs a new certificate"`
	// Description is the user-facing label DSM stores for the certificate.
	Description string `json:"description,omitempty" jsonschema:"Human-readable certificate description"`
	// AsDefault makes the imported certificate the default DSM presents.
	AsDefault bool `json:"as_default,omitempty" jsonschema:"Make the imported certificate the default DSM presents"`
	// AcknowledgeCurrentSession is the required explicit acknowledgement when the
	// operation replaces or rebinds the certificate that serves the current
	// dsmctl session (DSM desktop / default), which can break admin TLS.
	AcknowledgeCurrentSession bool `json:"acknowledge_current_session,omitempty" jsonschema:"Explicit acknowledgement required when the operation affects the certificate serving the current dsmctl session"`
}

// SetDefaultChange makes an installed certificate the default.
type SetDefaultChange struct {
	ID                        string `json:"id" jsonschema:"Certificate id to make default"`
	AcknowledgeCurrentSession bool   `json:"acknowledge_current_session,omitempty" jsonschema:"Explicit acknowledgement required when this changes the certificate serving the current dsmctl session"`
}

// BindServiceChange binds a DSM service key to an installed certificate.
type BindServiceChange struct {
	Service                   string `json:"service" jsonschema:"DSM service key to bind, for example default (DSM desktop) or ftpd"`
	CertID                    string `json:"cert_id" jsonschema:"Certificate id the service should present"`
	AcknowledgeCurrentSession bool   `json:"acknowledge_current_session,omitempty" jsonschema:"Explicit acknowledgement required when binding the DSM desktop service"`
}

// DeleteChange removes an installed certificate.
type DeleteChange struct {
	ID                        string `json:"id" jsonschema:"Certificate id to delete"`
	AcknowledgeCurrentSession bool   `json:"acknowledge_current_session,omitempty" jsonschema:"Explicit acknowledgement required when deleting the certificate serving the current dsmctl session"`
}

// DesiredCertificate is the plan-time public fingerprint of the certificate an
// import will install. It carries only public certificate fields plus the NAME
// of the key credential reference — never the key value. The plan hash covers
// this struct, so the approval binds the exact desired leaf without ever
// touching secret material.
type DesiredCertificate struct {
	Subject              Name     `json:"subject" jsonschema:"Parsed leaf subject"`
	SubjectAltNames      []string `json:"subject_alt_names,omitempty" jsonschema:"Parsed leaf subject alternative names"`
	Issuer               Name     `json:"issuer" jsonschema:"Parsed leaf issuer"`
	Serial               string   `json:"serial,omitempty" jsonschema:"Leaf serial number (hex)"`
	NotBeforeUnix        int64    `json:"not_before_unix,omitempty" jsonschema:"Leaf not-before as Unix seconds"`
	NotAfterUnix         int64    `json:"not_after_unix,omitempty" jsonschema:"Leaf not-after as Unix seconds"`
	SHA256               string   `json:"sha256" jsonschema:"SHA-256 fingerprint of the leaf DER (hex, lowercase, no colons)"`
	KeyCredentialRefName string   `json:"key_credential_ref_name" jsonschema:"NAME portion of the key credential reference; never the value"`
	HasIntermediate      bool     `json:"has_intermediate,omitempty" jsonschema:"Whether an intermediate chain accompanies the import"`
}

// ObservedCertificate is the plan-time observation of one installed certificate
// used for the staleness precondition. DSM's CRT list does not return a DER
// fingerprint, so the observation fingerprints the identity fields it does
// return (id, default flag, bound service keys, expiry, subject/issuer CN).
type ObservedCertificate struct {
	ID            string   `json:"id" jsonschema:"Certificate id"`
	IsDefault     bool     `json:"is_default" jsonschema:"Whether DSM presents this certificate by default"`
	Services      []string `json:"services,omitempty" jsonschema:"Service keys bound to this certificate"`
	ValidTillUnix int64    `json:"valid_till_unix,omitempty" jsonschema:"Not-after as Unix seconds"`
	SubjectCN     string   `json:"subject_cn,omitempty" jsonschema:"Leaf subject common name"`
	IssuerCN      string   `json:"issuer_cn,omitempty" jsonschema:"Issuer common name"`
}

// Precondition is the fingerprinted observed certificate + binding state the
// apply requires to still hold.
type Precondition struct {
	Certificates []ObservedCertificate `json:"certificates" jsonschema:"Observed installed certificates and their bindings at plan time"`
	Fingerprint  string                `json:"fingerprint" jsonschema:"SHA-256 fingerprint of the observed state"`
}

// MutationResult is the normalized outcome of a certificate write, returned to
// the caller. It carries no key material.
type MutationResult struct {
	Action      string `json:"action" jsonschema:"Applied action"`
	CertID      string `json:"cert_id,omitempty" jsonschema:"Affected certificate id, when known"`
	Description string `json:"description,omitempty" jsonschema:"Certificate description echoed by DSM, when returned"`
	AsDefault   bool   `json:"as_default,omitempty" jsonschema:"Whether the certificate is now the default"`
}
