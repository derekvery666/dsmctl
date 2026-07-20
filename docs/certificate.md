# Certificate management

The certificate module reads the Control Panel → Security → Certificate
surface: the installed certificates, their public metadata, and which DSM
services and packages each one serves. It is the first module of the
security/networking greenfield program (see [gap-inventory](../spec/gap-inventory.md)).

Reads are complemented by a set of **guarded, hash-bound writes** — import a
cert + private key, set the default, bind a service, delete — plus a
key-extracting **export**. Every write is high risk: replacing or deleting the
certificate the DSM desktop presents can break admin TLS, including the
connection dsmctl itself rides. The write wire names are implemented to the
[WI-065](../spec/work-items/WI-065-certificate.md) best-knowledge shape and are
marked `WIRE-UNVERIFIED` in code; they still require a live `DSMCTL_DUMP` probe
and a throwaway-cert live apply before they can be trusted against a real NAS.

## Reads

```console
dsmctl certificate capabilities --nas office
dsmctl certificate list --nas office
dsmctl certificate list --nas office --json
```

- `capabilities` reports whether the certificate read backend was selected and
  the discovered API/version.
- `list` shows each installed certificate: subject CN, whether it is the
  default, expiry (with a computed days-to-expiry hint), whether DSM can renew
  it, whether it is broken, the services bound to it, and the stable id used by
  a future set/bind/delete. `--json` adds the issuer, SANs, key type,
  signature algorithm, and the parsed not-before/not-after Unix times.

The read returns **public certificate metadata only** — the decoder never
carries private-key bytes into the model, and no MCP tool returns key material.

MCP tools: `get_certificate_capabilities`, `get_certificates`.

## DSM backend (verified live on DSM 7.3)

- `SYNO.Core.Certificate.CRT` `list` v1 returns `certificates[]`, each with
  `id`, `desc`, `is_default`, `is_broken`, `renewable`, `key_types`,
  `signature_algorithm`, `issuer`/`subject` (CN/org/country + `sub_alt_name`),
  `valid_from`/`valid_till` (`Jan _2 15:04:05 2006 MST`), `user_deletable`,
  and an inline `services[]` — so one read covers both the inventory and the
  service→certificate bindings; no separate binding call is needed. A
  self-signed certificate carries a `self_signed_cacrt_info` block.

Certificate management is DSM core (not a package), so the operation selects on
the advertised API/version alone and fails closed when the API is absent.

## Guarded writes (plan/apply, hash-bound)

Every write follows the plan → approve → apply contract and is classified high
risk. `plan_*` reads current state and returns a hash-bound plan; `apply_*`
re-reads fresh state, rejects a stale plan, performs the op, and
postcondition-re-reads the certificate store.

```console
# import a bring-your-own certificate (leaf + key + optional chain)
DSMCTL_TLS_KEY=... # the private-key PEM in an env var
dsmctl certificate plan -f import.json -o plan.json
dsmctl certificate apply -f plan.json --approve <hash>
dsmctl certificate export --id <cert-id> -o bundle.p12   # WARNING: extracts the private key
```

- **Private keys are secrets.** The imported key is supplied by
  `key_credential_ref: env:NAME` and resolved to bytes **only at apply time**,
  streamed as a multipart part, and zeroized. It never enters the plan file, the
  approval hash, the result, or any log line — the plan records only the key
  reference's NAME plus the leaf's parsed public fields (subject, SAN, issuer,
  serial, validity, and the SHA-256 of the DER).
- **Pre-apply local validation** rejects a key/cert mismatch, an expired or
  not-yet-valid leaf, a broken intermediate chain, and (for a DSM-service
  binding) a leaf whose SAN does not cover the connection host — before the NAS
  is touched.
- **Current-session protection.** Replacing/deleting/rebinding the certificate
  that serves the current dsmctl session requires `acknowledge_current_session`.
  For an import, apply re-pins to the new leaf's known fingerprint before the
  post-apply re-read; a broken, unrecoverable handshake is reported as a lockout,
  not a success.
- **Export** writes the archive (which contains the private key) to a
  caller-named local file only — no key bytes are returned over MCP — and is
  stripped from the read-only remote gateway.

CLI: `certificate plan`, `certificate apply`, `certificate export`.
MCP tools: `plan_certificate_change`, `apply_certificate_plan`,
`get_certificate_export` — all excluded from the read-only gateway.

Wire names (`import`/`set`/`delete`/`export` methods, the `key`/`cert`/
`inter_cert` multipart fields, and the `SYNO.Core.Certificate.Service` `set`
binding) are the best-knowledge shape and are marked `WIRE-UNVERIFIED` in code;
confirm with a throwaway `DSMCTL_DUMP` probe before shipping.

Let's Encrypt issuance/renewal (`SYNO.Core.Certificate.LetsEncrypt`) is a
non-goal — it drives an external ACME challenge, not a settings write. Self-
signed generation, CSR export, and KMIP are likewise out of scope. See WI-065.
