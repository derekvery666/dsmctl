package certificate

// Guarded certificate WRITE wire surface. Everything here is isolated so a
// live-verification pass can correct a stale wire name in ONE place.
//
// WIRE-UNVERIFIED (WI-065): the import multipart field names (key/cert/
// inter_cert), the CRT import/set/delete/export method names, the Service set
// method + service/cert parameter names, and whether import posts to a dedicated
// cgi rather than entry.cgi are the spec author's BEST KNOWLEDGE from the DSM
// certificate UI and are FREQUENTLY STALE. They MUST be confirmed against a
// throwaway DSMCTL_DUMP probe before any of these writes are trusted against a
// real NAS. Nothing below has been live-verified in this session.

import (
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	// ServiceAPIName is the service→certificate binding family. Its list is code
	// 103 on the lab (bindings are read inline from CRT.list), but its `set` is
	// the binding write.
	ServiceAPIName = "SYNO.Core.Certificate.Service"

	// Capability names for the guarded writes.
	ImportCapabilityName      = "certificate.import"
	SetDefaultCapabilityName  = "certificate.set_default"
	BindServiceCapabilityName = "certificate.bind_service"
	DeleteCapabilityName      = "certificate.delete"
	ExportCapabilityName      = "certificate.export"

	// WIRE-UNVERIFIED (WI-065): CRT write method names.
	CRTImportMethod = "import"
	CRTSetMethod    = "set"
	CRTDeleteMethod = "delete"
	CRTExportMethod = "export"

	// WIRE-UNVERIFIED (WI-065): Service binding write method name.
	ServiceSetMethod = "set"

	// WIRE-UNVERIFIED (WI-065): import multipart file-part field names. The
	// private-key field name is exactly the kind of detail that is frequently
	// wrong; it is referenced only here.
	ImportFieldKey        = "key"
	ImportFieldCert       = "cert"
	ImportFieldInterCert  = "inter_cert"
	ImportFieldID         = "id"
	ImportFieldDesc       = "desc"
	ImportFieldAsDefault  = "as_default"
	ImportFieldSettleTime = "settle_time"

	// WIRE-UNVERIFIED (WI-065): CRT set / delete / export parameter names.
	SetFieldID        = "id"
	SetFieldAsDefault = "as_default"
	SetFieldDesc      = "desc"
	DeleteFieldID     = "id"
	ExportFieldID     = "id"

	// WIRE-UNVERIFIED (WI-065): Service set parameter names. DSM binds a service
	// by posting a JSON `settings` array of {service, id} objects.
	ServiceSetFieldSettings = "settings"
)

// MutationAPINames lists every DSM API the guarded writes may use, for discovery
// in one call. CRT is reused from the read operation.
func MutationAPINames() []string {
	return []string{CRTAPIName, ServiceAPIName}
}

// SupportsCRTWrites reports whether the CRT family (import/set/delete/export) is
// advertised. Writes reuse the same CRT v1 the read selected.
func SupportsCRTWrites(target compatibility.Target) bool {
	info, ok := target.API(CRTAPIName)
	return ok && info.Supports(1)
}

// SupportsServiceBinding reports whether the Service binding family is advertised
// for the service→certificate write. It is an independent boundary: a NAS may
// advertise CRT but not Service.
func SupportsServiceBinding(target compatibility.Target) bool {
	info, ok := target.API(ServiceAPIName)
	return ok && info.Supports(1)
}
