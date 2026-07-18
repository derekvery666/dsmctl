# Gateway data

The managed gateway creates `gateway.db` here. It contains transactional NAS
profiles and AES-256-GCM encrypted credentials. Keep this directory writable
by container UID/GID `10001:10001`, back it up as a unit, and never place the
vault master key here. Runtime data in this directory is ignored by Git.

`../config.example.json` remains only as an example for the explicit legacy
development mode that starts the binary without `--state`.
