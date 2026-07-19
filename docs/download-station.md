# Download Station

A read-only module for the Synology Download Station package, package-version
gated on the installed `DownloadStation` package like the Photos and Surveillance
modules. The service/task/statistic reads use the stable, publicly-documented
legacy `SYNO.DownloadStation.*` API (each served from its own CGI path, resolved
from the discovered API registry); the full detailed settings read uses the
newer `SYNO.DownloadStation2.Settings.*` API generation (all on `entry.cgi`).

```console
dsmctl download capabilities --nas office
dsmctl download service --nas office --json
dsmctl download tasks --nas office
dsmctl download statistics --nas office
dsmctl download settings --nas office
```

- **`capabilities`** reports the installed package evidence (installed, version,
  running) and which reads are available, each selected independently. A NAS
  without Download Station — or below the verified 3.0 baseline — fails closed.
- **`service`** reads `SYNO.DownloadStation.Info` (`getinfo` + `getconfig`) and
  `SYNO.DownloadStation.Schedule` (`getconfig`): version, manager flag, default
  destination, eMule and auto-unzip switches, per-protocol (BT/eMule/FTP/HTTP/NZB)
  rate limits in KB/s (0 = unlimited), and the bandwidth schedule.
- **`tasks`** reads `SYNO.DownloadStation.Task` (`list`): each task's id, type,
  title, size, status, destination, and live transfer speed. Task entries are
  decoded tolerantly (a size or speed returned as a quoted string is handled)
  because the verification NAS had no task to populate the list.
- **`statistics`** reads `SYNO.DownloadStation.Statistic` (`getinfo`): the
  aggregate download and upload speed in bytes/s.
- **`settings`** composes the `SYNO.DownloadStation2.Settings.*` reads into the
  full detailed configuration: BitTorrent (TCP/DHT ports, DHT, port forwarding,
  preview, encryption, rate limits, max peers, seeding), eMule, FTP/HTTP, NZB,
  automatic extraction, destination/watch-folder, RSS refresh interval, and the
  bandwidth scheduler (with DSM's raw 168-slot weekly bitmap). The NZB password
  and archive-extraction passwords are never surfaced — only a
  `password_configured` flag is.

MCP exposes the same reads through `get_download_station_capabilities`,
`get_download_station_service`, `get_download_station_tasks`,
`get_download_station_statistics`, and `get_download_station_settings`. All are
read-only.

Field shapes are live-verified on Download Station 4.1.2. Everything writable —
task mutations (create/pause/resume/delete/edit), settings writes — plus BT/eMule
search, RSS management, and eMule server management are still out of scope for
this read module; see
[WI-043](../spec/work-items/WI-043-download-station.md).
