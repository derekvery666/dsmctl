dsmctl CLI release bundle

This archive contains:

- dsmctl: the command-line client for administrators, scripts, and CI.
- dsmctl-mcp: the local stdio MCP server for MCP clients (e.g. Claude Code /
  Claude Desktop). No certificate or DNS is needed: it runs on your machine and
  reaches the NAS through dsmctl's own authenticated session.
- LICENSE: Apache License 2.0.

Local MCP quick start (no certificate, no DNS):
  dsmctl nas add <name> --url https://<nas-ip>:5001 --username <account> --default
  dsmctl auth login --nas <name>
  claude mcp add <name> <path-to-dsmctl-mcp>

The remote gateway (browser Admin console + HTTP MCP endpoint over HTTPS) is
delivered inside the separate x86_64 Synology .spk release asset; that path
requires the NAS to be reached over HTTPS with a certificate the client trusts
(free via Synology DDNS + Let's Encrypt).

Documentation and release downloads:
https://github.com/derekvery666/dsmctl
https://github.com/derekvery666/dsmctl/releases

Verify the downloaded archive against SHA256SUMS before extracting it.
