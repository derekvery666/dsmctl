# Gateway development secrets

Create `dev-token` with a random value of at least 32 bytes. Optionally create
`dsm-passwords.env` with one `DSMCTL_PASSWORD_<PROFILE>=...` entry per NAS.
Files in this directory are mounted read-only and ignored by Git.

The environment-password mechanism is only the WI-014 read-only development
bootstrap. WI-015 replaces it with the encrypted gateway vault.
