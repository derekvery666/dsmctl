# dsmctl

`dsmctl` 是以 Go 開發的 Synology DSM 管理工具。Repository 同時產生兩個產品：

- `dsmctl`：給人員與自動化腳本使用的 CLI。
- `dsmctl-mcp`：讓 MCP client 使用相同操作能力的 stdio MCP server。

兩個入口共用 profile、credential resolver、DSM WebAPI client、session manager 與 application service。第一版的範圍刻意保持很小：管理多台 NAS 連線設定、登入 DSM，以及讀取基本 system info。

## 架構

```text
cmd/dsmctl          ─┐
                      ├─ internal/application ─ internal/runtime ─ internal/synology ─ DSM WebAPI
cmd/dsmctl-mcp      ─┘                         │
                                                ├─ internal/config
                                                └─ internal/credentials
```

- `internal/synology`：HTTP、API discovery、登入、session refresh、登出及 typed DSM API。
- `internal/runtime`：每個 NAS profile 各自維護可重用的 client/session，多台 NAS 可以同時登入。
- `internal/application`：CLI 與 MCP 共用的 use cases。後續功能應優先加在這裡。
- `internal/cli`、`internal/mcpserver`：只處理各自的輸入輸出。

## 需求與建置

需要 Go 1.25 或更新版本。

```console
go test ./...
go build -o bin/dsmctl ./cmd/dsmctl
go build -o bin/dsmctl-mcp ./cmd/dsmctl-mcp
```

## CLI 快速開始

加入第一台 NAS：

```console
dsmctl nas add office --url https://nas-office.example.com:5001 --username automation --default
```

PowerShell 設定本次 terminal session 使用的密碼：

```powershell
$env:DSMCTL_PASSWORD_OFFICE = "your-password"
```

讀取 system info：

```console
dsmctl system info
dsmctl system info --nas office --json
```

加入第二台 NAS：

```console
dsmctl nas add lab --url https://192.168.10.20:5001 --username automation
dsmctl nas list
dsmctl system info --nas lab
dsmctl nas use lab
```

預設設定檔位於作業系統的 user config directory，例如 Windows 的 `%AppData%\dsmctl\config.json`。可以使用 `--config` 或 `DSMCTL_CONFIG` 指定其他位置。

設定檔不儲存密碼，只儲存環境變數名稱：

```json
{
  "default_nas": "office",
  "nas": {
    "office": {
      "url": "https://nas-office.example.com:5001",
      "username": "automation",
      "password_env": "DSMCTL_PASSWORD_OFFICE",
      "timeout_seconds": 30
    }
  }
}
```

DSM 常使用自簽憑證。正式環境建議配置可信任憑證；測試環境可在 `nas add` 加上 `--insecure-skip-tls-verify`，但這會停用伺服器憑證驗證。

## MCP Server

啟動 stdio server：

```console
dsmctl-mcp --config C:\path\to\config.json
```

目前提供兩個 tools：

- `list_nas`：列出已設定的 NAS，不會回傳密碼。
- `get_system_info`：接受可選的 `nas` profile name，登入並讀取 system info。

MCP host 啟動 `dsmctl-mcp` 時，也必須能讀取各 profile 指定的 password environment variable。不要把密碼提交到 Git。

## 第一版限制

- Credential resolver 目前使用環境變數；OS keychain backend 可在後續加入，不需要修改 application、CLI 或 MCP 層。
- 尚未支援 OTP、Approve sign-in 或其他 MFA challenge。
- System info 使用 NAS 經 `SYNO.API.Info` 公告的 `SYNO.Core.System` API 與最高可用版本；不同 DSM 版本回傳欄位可能不同。
- 第一版只有 stdio MCP transport；未開放網路監聽。

## 擴充功能的方式

新增功能時依序加入：

1. 在 `internal/synology` 建立 typed API method。
2. 在 `internal/application` 建立有 validation 與安全策略的 use case。
3. 分別在 CLI command 與 MCP tool 做薄薄的輸入輸出 mapping。

修改底層 DSM 行為後，CLI 與 MCP 會從同一個 application service 同步取得修正。
