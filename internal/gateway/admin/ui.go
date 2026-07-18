package admin

const indexHTML = `<!doctype html>
<html lang="zh-Hant">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>dsmctl Gateway</title>
<style>
:root{
  color-scheme:light;
  --blue:#2384e8;
  --blue-strong:#0873dc;
  --blue-soft:#eaf4ff;
  --navy:#203247;
  --navy-soft:#34495f;
  --canvas:#f2f5f9;
  --surface:#ffffff;
  --surface-soft:#f8fafc;
  --text:#182434;
  --muted:#65758a;
  --line:#dce4ed;
  --line-strong:#c9d4df;
  --success:#1f9d68;
  --warning:#d88918;
  --danger:#cf3f3f;
  --shadow:0 12px 30px rgba(27,52,78,.08);
  --shadow-large:0 28px 80px rgba(25,54,86,.18);
  font-family:Inter,"Segoe UI","Noto Sans TC",system-ui,-apple-system,sans-serif;
}
*{box-sizing:border-box}
html,body{margin:0;min-height:100%;background:var(--canvas);color:var(--text)}
body{font-size:14px;line-height:1.55}
button,input,select{font:inherit}
button{cursor:pointer}
[hidden]{display:none!important}
:focus-visible{outline:3px solid rgba(35,132,232,.28);outline-offset:2px}

.brand{display:flex;align-items:center;gap:11px;color:inherit;text-decoration:none}
.brand-mark{display:grid;grid-template-columns:repeat(2,9px);grid-template-rows:repeat(2,9px);gap:3px;padding:8px;border-radius:10px;background:linear-gradient(145deg,#43a3ff,#0873dc);box-shadow:0 7px 16px rgba(8,115,220,.25)}
.brand-mark i{display:block;border-radius:2px;background:rgba(255,255,255,.94)}
.brand-copy strong{display:block;font-size:15px;line-height:1.15;letter-spacing:.01em}
.brand-copy span{display:block;margin-top:2px;color:var(--muted);font-size:11px}

.auth-shell{min-height:100vh;display:grid;grid-template-columns:minmax(360px,44%) 1fr;padding:24px;background:radial-gradient(circle at 86% 12%,rgba(75,164,255,.18),transparent 28%),linear-gradient(145deg,#eaf2fa 0,#f7f9fc 52%,#edf4fb 100%)}
.auth-visual{position:relative;display:flex;flex-direction:column;justify-content:space-between;min-height:calc(100vh - 48px);padding:48px;overflow:hidden;border-radius:24px;background:linear-gradient(150deg,#1c6fbf 0,#238eea 54%,#42a6f6 100%);color:white;box-shadow:var(--shadow-large)}
.auth-visual:before,.auth-visual:after{content:"";position:absolute;border:1px solid rgba(255,255,255,.18);border-radius:50%}
.auth-visual:before{width:430px;height:430px;right:-190px;top:-110px}
.auth-visual:after{width:310px;height:310px;right:-30px;bottom:-190px;background:rgba(255,255,255,.05)}
.auth-visual .brand{position:relative;z-index:1}
.auth-visual .brand-mark{background:rgba(255,255,255,.2);box-shadow:none;backdrop-filter:blur(8px)}
.auth-visual .brand-copy strong{font-size:18px}
.auth-visual .brand-copy span{color:rgba(255,255,255,.7)}
.auth-message{position:relative;z-index:1;max-width:520px;margin:auto 0}
.auth-kicker{display:inline-flex;align-items:center;gap:8px;margin-bottom:18px;padding:6px 10px;border:1px solid rgba(255,255,255,.24);border-radius:999px;background:rgba(255,255,255,.1);font-size:12px}
.auth-kicker:before{content:"";width:7px;height:7px;border-radius:50%;background:#8cf2c4;box-shadow:0 0 0 4px rgba(140,242,196,.14)}
.auth-message h1{max-width:500px;margin:0 0 18px;font-size:clamp(34px,4vw,58px);line-height:1.08;letter-spacing:-.035em}
.auth-message p{max-width:480px;margin:0;color:rgba(255,255,255,.76);font-size:16px}
.auth-foot{position:relative;z-index:1;display:flex;gap:18px;color:rgba(255,255,255,.68);font-size:12px}
.auth-foot span{display:flex;align-items:center;gap:6px}
.auth-foot span:before{content:"✓";color:#9af2ca}
.auth-panel{display:flex;align-items:center;justify-content:center;padding:42px}
.auth-panel-inner{width:min(460px,100%)}
.auth-panel-brand{display:none;margin-bottom:28px}
.auth-card{padding:34px;border:1px solid rgba(211,222,233,.92);border-radius:18px;background:rgba(255,255,255,.92);box-shadow:var(--shadow);backdrop-filter:blur(14px)}
.auth-card h2{margin:0 0 8px;font-size:25px;letter-spacing:-.02em}
.auth-card>.lead{margin:0 0 26px;color:var(--muted)}
.auth-card .helper{margin:18px 0 0;color:var(--muted);font-size:12px}
.auth-card .warning-copy{padding:12px 14px;border-left:3px solid var(--warning);border-radius:4px;background:#fff8eb;color:#7b581c;font-size:12px}
.auth-deadline{display:flex;align-items:center;gap:9px;margin:0 0 22px;padding:10px 12px;border-radius:8px;background:var(--blue-soft);color:#2366a7;font-size:12px}
.auth-deadline:before{content:"";width:8px;height:8px;border-radius:50%;background:var(--blue)}

.field{display:flex;flex-direction:column;gap:7px;min-width:0}
.field+.field{margin-top:16px}
.field label,.field-label{color:#44546a;font-size:12px;font-weight:600}
.field-hint{margin-top:5px;color:var(--muted);font-size:11px}
.control{width:100%;min-height:40px;padding:9px 11px;border:1px solid var(--line-strong);border-radius:7px;background:#fff;color:var(--text);transition:border-color .15s,box-shadow .15s}
.control:hover{border-color:#aebdca}
.control:focus{border-color:var(--blue);box-shadow:0 0 0 3px rgba(35,132,232,.12);outline:none}
.control::placeholder{color:#9aa7b6}
.button-row{display:flex;align-items:center;flex-wrap:wrap;gap:9px;margin-top:22px}
.button{display:inline-flex;align-items:center;justify-content:center;gap:7px;min-height:38px;padding:8px 15px;border:1px solid transparent;border-radius:7px;background:var(--blue);color:white;font-weight:600;box-shadow:0 2px 4px rgba(8,115,220,.16);transition:background .15s,transform .15s,box-shadow .15s}
.button:hover{background:var(--blue-strong);box-shadow:0 5px 12px rgba(8,115,220,.2)}
.button:active{transform:translateY(1px)}
.button.secondary{border-color:var(--line-strong);background:#fff;color:#34455a;box-shadow:none}
.button.secondary:hover{border-color:#aebdca;background:#f8fafc}
.button.danger{background:var(--danger);box-shadow:none}
.button.danger:hover{background:#b93636}
.button.compact{min-height:32px;padding:5px 10px;font-size:12px;font-weight:500}
.button.full{width:100%}

.app-shell{min-height:100vh;display:grid;grid-template-columns:224px minmax(0,1fr);grid-template-rows:58px minmax(0,1fr)}
.topbar{position:sticky;z-index:20;top:0;grid-column:1/-1;display:flex;align-items:center;justify-content:space-between;padding:0 20px;border-bottom:1px solid var(--line);background:rgba(255,255,255,.95);box-shadow:0 2px 10px rgba(33,55,79,.04);backdrop-filter:blur(12px)}
.topbar-right{display:flex;align-items:center;gap:12px}
.online-pill{display:flex;align-items:center;gap:7px;padding:6px 10px;border-radius:999px;background:#edf9f4;color:#27825c;font-size:11px;font-weight:600}
.online-pill:before{content:"";width:7px;height:7px;border-radius:50%;background:#35b47b;box-shadow:0 0 0 3px rgba(53,180,123,.12)}
.user-button{display:flex;align-items:center;gap:9px;padding:4px 9px 4px 5px;border:0;border-radius:999px;background:transparent;color:#33465a}
.user-button:hover{background:#eef3f8}
.user-avatar{display:grid;width:30px;height:30px;place-items:center;border-radius:50%;background:linear-gradient(145deg,#e1effe,#b9daf9);color:#176cbf;font-size:12px;font-weight:700}

.sidebar{position:sticky;top:58px;align-self:start;height:calc(100vh - 58px);display:flex;flex-direction:column;padding:18px 12px 14px;border-right:1px solid #2c4056;background:var(--navy);color:#dbe6f1}
.nav-label{margin:4px 11px 8px;color:#8095aa;font-size:10px;font-weight:700;letter-spacing:.13em;text-transform:uppercase}
.nav-list{display:flex;flex-direction:column;gap:4px}
.nav-item{display:flex;align-items:center;gap:11px;width:100%;padding:10px 11px;border:0;border-radius:7px;background:transparent;color:#becddd;text-align:left;transition:background .15s,color .15s}
.nav-item svg{width:18px;height:18px;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.nav-item:hover{background:rgba(255,255,255,.07);color:#fff}
.nav-item.active{background:linear-gradient(90deg,rgba(55,150,238,.34),rgba(55,150,238,.15));color:#fff;box-shadow:inset 3px 0 #55adff}
.sidebar-foot{margin-top:auto;padding:12px 10px 4px;border-top:1px solid rgba(255,255,255,.08);color:#8fa2b6;font-size:11px}
.sidebar-foot strong{display:block;margin-bottom:4px;color:#c8d6e4;font-size:11px}

.workspace{min-width:0;padding:28px clamp(20px,3vw,42px) 52px;background:var(--canvas)}
.view{max-width:1280px;margin:0 auto}
.page-head{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;margin-bottom:22px}
.page-head h1{margin:0 0 5px;font-size:25px;line-height:1.25;letter-spacing:-.025em}
.page-head p{margin:0;color:var(--muted)}
.page-actions{display:flex;flex-wrap:wrap;gap:8px}

.hero{position:relative;display:flex;align-items:center;justify-content:space-between;gap:28px;min-height:190px;margin-bottom:20px;padding:30px 34px;overflow:hidden;border:1px solid #d0e4f8;border-radius:14px;background:linear-gradient(120deg,#fff 0,#f5faff 56%,#e5f3ff 100%);box-shadow:var(--shadow)}
.hero:after{content:"";position:absolute;width:240px;height:240px;right:-60px;top:-115px;border:45px solid rgba(35,132,232,.08);border-radius:50%}
.hero-copy{position:relative;z-index:1;max-width:680px}
.hero-copy .eyebrow{margin-bottom:9px;color:var(--blue-strong);font-size:11px;font-weight:700;letter-spacing:.1em;text-transform:uppercase}
.hero-copy h2{margin:0 0 9px;font-size:27px;letter-spacing:-.025em}
.hero-copy p{margin:0;color:var(--muted)}
.hero-icon{position:relative;z-index:1;display:grid;flex:0 0 96px;width:96px;height:96px;place-items:center;border:1px solid rgba(255,255,255,.9);border-radius:24px;background:rgba(255,255,255,.62);box-shadow:0 14px 30px rgba(39,110,176,.12);backdrop-filter:blur(8px)}
.hero-icon .brand-mark{grid-template-columns:repeat(2,16px);grid-template-rows:repeat(2,16px);gap:5px;padding:13px;border-radius:17px}

.stats{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-bottom:20px}
.stat{display:flex;align-items:center;gap:14px;padding:18px 20px;border:1px solid var(--line);border-radius:11px;background:var(--surface);box-shadow:0 4px 16px rgba(34,58,82,.035)}
.stat-icon{display:grid;flex:0 0 42px;width:42px;height:42px;place-items:center;border-radius:10px;background:var(--blue-soft);color:var(--blue-strong)}
.stat-icon svg{width:21px;height:21px;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.stat strong{display:block;font-size:23px;line-height:1.1}
.stat span{display:block;margin-top:4px;color:var(--muted);font-size:11px}

.content-grid{display:grid;grid-template-columns:minmax(0,1.3fr) minmax(280px,.7fr);gap:18px}
.panel{min-width:0;border:1px solid var(--line);border-radius:11px;background:var(--surface);box-shadow:0 4px 16px rgba(34,58,82,.035)}
.panel+.panel{margin-top:18px}
.panel-head{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;padding:18px 20px;border-bottom:1px solid #e8edf3}
.panel-head h2{margin:0 0 3px;font-size:16px}
.panel-head p{margin:0;color:var(--muted);font-size:12px}
.panel-body{padding:20px}
.quick-list{display:grid;gap:8px}
.quick-action{display:flex;align-items:center;justify-content:space-between;width:100%;padding:12px 13px;border:1px solid var(--line);border-radius:8px;background:#fff;color:var(--text);text-align:left}
.quick-action:hover{border-color:#b9d7f3;background:#f6fbff}
.quick-action span{color:var(--blue);font-size:18px}
.notice{padding:13px 14px;border:1px solid #d7e9f9;border-radius:8px;background:#f3f9ff;color:#3d6386;font-size:12px}
.notice strong{display:block;margin-bottom:3px;color:#28577f}
.notice.warning{border-color:#f1dfbd;background:#fffaf0;color:#765c2e}
.notice.warning strong{color:#664913}

.form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 18px}
.form-grid .field+.field{margin-top:0}
.field-span-2{grid-column:span 2}
.inline-form{display:grid;grid-template-columns:repeat(4,minmax(140px,1fr));gap:14px;align-items:end}
.inline-form .field+.field{margin-top:0}
.scope-row{display:flex;flex-wrap:wrap;gap:8px;margin-top:16px}
.check-chip{position:relative;display:inline-flex;align-items:center;gap:7px;padding:7px 10px;border:1px solid var(--line-strong);border-radius:7px;background:#fff;color:#44546a;font-size:12px}
.check-chip:has(input:checked){border-color:#9bc9f2;background:var(--blue-soft);color:#176bb8}
.check-chip input{accent-color:var(--blue)}

.table-wrap{width:100%;overflow-x:auto}
table{width:100%;border-collapse:collapse}
th,td{padding:11px 12px;border-bottom:1px solid #e8edf3;text-align:left;vertical-align:top}
th{color:#65758a;font-size:10px;font-weight:700;letter-spacing:.055em;text-transform:uppercase;white-space:nowrap}
td{color:#33455a;font-size:12px}
tbody tr:last-child td{border-bottom:0}
tbody tr:not(.empty-row):hover{background:#fbfcfe}
.table-actions{display:flex;flex-wrap:wrap;gap:5px}
.table-action{padding:4px 7px;border:1px solid var(--line);border-radius:5px;background:#fff;color:#38658d;font-size:11px}
.table-action:hover{border-color:#a8cbed;background:#f4f9fe}
.table-action.danger{color:#bc3e3e}
.empty-row td{padding:34px 20px;text-align:center}
.empty-state{display:flex;flex-direction:column;align-items:center;color:var(--muted)}
.empty-state .empty-icon{display:grid;width:42px;height:42px;margin-bottom:10px;place-items:center;border-radius:50%;background:#eef3f8;color:#7890a7;font-size:18px}
.empty-state strong{margin-bottom:3px;color:#41546a;font-size:13px}
.empty-state span{font-size:11px}
.badge{display:inline-flex;align-items:center;padding:3px 7px;border-radius:999px;background:#eef3f8;color:#52677c;font-size:10px;font-weight:600;white-space:nowrap}
.badge.success{background:#eaf8f2;color:#267b59}
.badge.warning{background:#fff5df;color:#8a641b}
.secret{margin-top:16px;padding:13px;border:1px solid #efd28d;border-radius:8px;background:#fff9e8;color:#6f5522;white-space:pre-wrap;overflow-wrap:anywhere}
.audit-log{max-height:520px;margin:0;padding:16px;overflow:auto;border:1px solid #dce4ed;border-radius:8px;background:#182433;color:#d8e4ef;font:11px/1.6 ui-monospace,SFMono-Regular,Consolas,monospace;white-space:pre-wrap}

.toast{position:fixed;z-index:100;right:22px;bottom:22px;max-width:min(430px,calc(100vw - 44px));padding:12px 15px;border:1px solid #b9d9f5;border-radius:9px;background:#fff;color:#315b7e;box-shadow:0 14px 36px rgba(27,52,78,.18);white-space:pre-wrap}
.toast.error{border-color:#efbbbb;color:#a43131}

@media(max-width:1000px){
  .auth-shell{grid-template-columns:minmax(320px,40%) 1fr}.auth-visual{padding:34px}.inline-form{grid-template-columns:repeat(2,minmax(160px,1fr))}.content-grid{grid-template-columns:1fr}
}
@media(max-width:760px){
  .auth-shell{display:block;padding:0;background:#f3f7fb}.auth-visual{display:none}.auth-panel{min-height:100vh;padding:24px 18px}.auth-panel-brand{display:flex}.auth-card{padding:25px 20px}
  .app-shell{display:block;padding-top:58px}.topbar{position:fixed;left:0;right:0;height:58px}.online-pill{display:none}
  .sidebar{position:sticky;z-index:15;top:58px;width:100%;height:auto;display:block;padding:7px 10px;border-right:0;border-bottom:1px solid #2c4056;overflow-x:auto;scrollbar-width:none}.sidebar::-webkit-scrollbar{display:none}.nav-label,.sidebar-foot{display:none}.nav-list{display:flex;flex-direction:row;min-width:max-content}.nav-item{width:auto;padding:8px 11px}.nav-item.active{box-shadow:inset 0 -3px #55adff}.nav-item svg{width:16px;height:16px}
  .workspace{padding:22px 14px 40px}.page-head{display:block}.page-actions{margin-top:14px}.hero{min-height:0;padding:24px}.hero-icon{display:none}.hero-copy h2{font-size:22px}.stats{grid-template-columns:1fr}.form-grid,.inline-form{grid-template-columns:1fr}.field-span-2{grid-column:auto}.panel-head{padding:16px}.panel-body{padding:16px}.topbar{padding:0 12px}.topbar .brand-copy span{display:none}
}
</style>
</head>
<body>
<div id="message" class="toast" role="status" aria-live="polite" hidden></div>

<section id="entryShell" class="auth-shell">
  <div class="auth-visual" aria-hidden="true">
    <div class="brand"><span class="brand-mark"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl Gateway</strong><span>Fleet administration</span></span></div>
    <div class="auth-message">
      <div class="auth-kicker">Private NAS control plane</div>
      <h1>一個入口，管理每一台 NAS。</h1>
      <p>Gateway 與 DSM 身分完全分離。每台 NAS 都有獨立 Profile、登入狀態與存取邊界。</p>
    </div>
    <div class="auth-foot"><span>HttpOnly/SameSite session</span><span>Encrypted state</span><span>Scoped MCP access</span></div>
  </div>
  <div class="auth-panel">
    <div class="auth-panel-inner">
      <div class="brand auth-panel-brand"><span class="brand-mark"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl Gateway</strong><span>Fleet administration</span></span></div>
      <section id="setupCard" class="auth-card" hidden>
        <h2>建立 Gateway 管理員</h2>
        <p class="lead">這組帳密只管理 Gateway，不會自動取得任何 DSM 或 Host NAS 權限。</p>
        <p id="setupExpiry" class="auth-deadline"></p>
        <div class="field"><label for="setupUsername">管理員帳號</label><input class="control" id="setupUsername" autocomplete="username" placeholder="3–64 個字元"></div>
        <div class="field"><label for="setupPassword">密碼</label><input class="control" id="setupPassword" type="password" autocomplete="new-password" placeholder="至少 12 bytes"></div>
        <div class="field"><label for="setupConfirm">確認密碼</label><input class="control" id="setupConfirm" type="password" autocomplete="new-password" placeholder="再次輸入密碼"></div>
        <div class="button-row"><button class="button full" type="button" onclick="setupAdministrator()">建立並進入管理介面</button></div>
        <p class="helper">第一次啟動後一小時內可建立管理員；逾時且仍未設定時，重新啟動 Gateway 即可重新開放。</p>
      </section>
      <section id="expiredCard" class="auth-card" hidden>
        <h2>設定時間已過</h2>
        <p class="lead">Gateway 尚未初始化，為避免長時間暴露首次設定入口，本次視窗已關閉。</p>
        <div class="notice warning"><strong>如何繼續</strong>重新啟動未初始化的 container 或套件，設定入口會再開放一小時。</div>
      </section>
      <section id="loginCard" class="auth-card" hidden>
        <h2>歡迎回來</h2>
        <p id="initializedAt" class="lead"></p>
        <div class="field"><label for="loginUsername">管理員帳號</label><input class="control" id="loginUsername" autocomplete="username" placeholder="Gateway 管理員"></div>
        <div class="field"><label for="loginPassword">密碼</label><input class="control" id="loginPassword" type="password" autocomplete="current-password" placeholder="輸入密碼"></div>
        <div class="button-row"><button class="button full" type="button" onclick="loginAdministrator()">登入 Gateway</button></div>
        <p class="warning-copy">若你從未建立管理員卻看到此頁，請不要新增 NAS。請由部署主機清除 Gateway 資料後重新初始化；清除資料會刪除所有 NAS session。</p>
      </section>
    </div>
  </div>
</section>

<main id="application" hidden>
  <div class="app-shell">
    <header class="topbar">
      <div class="brand"><span class="brand-mark"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl Gateway</strong><span>Fleet administration</span></span></div>
      <div class="topbar-right"><span class="online-pill">Gateway online</span><button id="topUser" class="user-button" type="button" onclick="setView('admin')"><span class="user-avatar">A</span><span>Administrator</span></button></div>
    </header>
    <aside class="sidebar" aria-label="Gateway 管理導覽">
      <div class="nav-label">管理</div>
      <nav class="nav-list">
        <button class="nav-item active" data-nav="overview" type="button" onclick="setView('overview')"><svg viewBox="0 0 24 24"><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z"/></svg>總覽</button>
        <button class="nav-item" data-nav="nas" type="button" onclick="setView('nas')"><svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01M11 7.5h6M11 16.5h6"/></svg>NAS</button>
        <button class="nav-item" data-nav="access" type="button" onclick="setView('access')"><svg viewBox="0 0 24 24"><circle cx="8" cy="12" r="4"/><path d="M12 12h9M17 12v3M20 12v2"/></svg>MCP 存取</button>
        <button class="nav-item" data-nav="approvals" type="button" onclick="setView('approvals')"><svg viewBox="0 0 24 24"><path d="M12 3l8 4v5c0 5-3.4 8-8 9-4.6-1-8-4-8-9V7z"/><path d="M8.5 12l2.2 2.2 4.8-5"/></svg>核准</button>
        <button class="nav-item" data-nav="audit" type="button" onclick="setView('audit')"><svg viewBox="0 0 24 24"><path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5M9 12h7M9 16h7"/></svg>Audit</button>
        <button class="nav-item" data-nav="admin" type="button" onclick="setView('admin')"><svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="4"/><path d="M4 21c.8-4 3.4-6 8-6s7.2 2 8 6"/></svg>管理員</button>
      </nav>
      <div class="sidebar-foot"><strong>獨立信任邊界</strong>Host NAS 與遠端 NAS 都必須個別新增及登入。</div>
    </aside>

    <section class="workspace">
      <section id="view-overview" class="view" data-view="overview">
        <div class="page-head"><div><h1>系統總覽</h1><p>Gateway 與 Fleet 的目前狀態。</p></div><div class="page-actions"><button class="button" type="button" onclick="setView('nas')">新增 NAS</button></div></div>
        <div class="hero"><div class="hero-copy"><div class="eyebrow">Portable management gateway</div><h2>你的 NAS Fleet，由一個乾淨的控制面管理。</h2><p>每個 Profile 各自保存登入狀態；MCP Token、NAS allowlist 與高風險核准彼此獨立，不會因為 Gateway 安裝在哪台 NAS 就自動取得權限。</p></div><div class="hero-icon"><span class="brand-mark"><i></i><i></i><i></i><i></i></span></div></div>
        <div class="stats">
          <div class="stat"><span class="stat-icon"><svg viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="6" rx="2"/><rect x="3" y="13" width="18" height="6" rx="2"/></svg></span><div><strong id="metricNAS">0</strong><span>NAS Profiles</span></div></div>
          <div class="stat"><span class="stat-icon"><svg viewBox="0 0 24 24"><circle cx="8" cy="12" r="4"/><path d="M12 12h9M17 12v3"/></svg></span><div><strong id="metricTokens">0</strong><span>Active MCP Tokens</span></div></div>
          <div class="stat"><span class="stat-icon"><svg viewBox="0 0 24 24"><path d="M12 3l8 4v5c0 5-3.4 8-8 9-4.6-1-8-4-8-9V7z"/></svg></span><div><strong id="metricApprovals">0</strong><span>Ready Approvals</span></div></div>
        </div>
        <div class="content-grid">
          <div class="panel"><div class="panel-head"><div><h2>開始管理 Fleet</h2><p>所有 NAS 都從明確的 Profile 開始。</p></div></div><div class="panel-body"><div class="notice"><strong>沒有隱含的 Host NAS</strong>請使用 LAN IP 或 DNS 名稱新增安裝 Gateway 的 NAS；container 裡的 localhost 不是 Host NAS。</div><div class="button-row"><button class="button" type="button" onclick="setView('nas')">前往 NAS 管理</button><button class="button secondary" type="button" onclick="setView('access')">設定 MCP 存取</button></div></div></div>
          <div class="panel"><div class="panel-head"><div><h2>快速前往</h2><p>常用的管理區域。</p></div></div><div class="panel-body quick-list"><button class="quick-action" type="button" onclick="setView('nas')">NAS Profiles <span>›</span></button><button class="quick-action" type="button" onclick="setView('approvals')">高風險核准 <span>›</span></button><button class="quick-action" type="button" onclick="setView('audit')">Audit 紀錄 <span>›</span></button></div></div>
        </div>
      </section>

      <section id="view-nas" class="view" data-view="nas" hidden>
        <div class="page-head"><div><h1>NAS 管理</h1><p>建立獨立 Profile，並為每台 NAS 個別完成 DSM 登入。</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadProfiles()">重新整理</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2>新增 NAS</h2><p>請使用 Gateway container 可連線的 LAN IP 或 DNS 名稱。</p></div></div><div class="panel-body"><div class="form-grid">
          <div class="field"><label for="name">Profile 名稱</label><input class="control" id="name" placeholder="例如 office"></div>
          <div class="field"><label for="url">DSM URL</label><input class="control" id="url" placeholder="https://nas.example:5001"></div>
          <div class="field"><label for="username">DSM 帳號</label><input class="control" id="username" placeholder="operator"></div>
          <div class="field"><label for="tls">TLS 驗證</label><select class="control" id="tls" onchange="toggleFingerprint()"><option value="system_ca">System CA</option><option value="pinned_fingerprint">Pinned fingerprint</option></select></div>
          <div id="fingerprintField" class="field field-span-2" hidden><label for="fingerprint">SHA-256 certificate fingerprint</label><input class="control" id="fingerprint" placeholder="64 位十六進位 fingerprint"><span class="field-hint">選擇 pinned fingerprint 時必須明確確認信任。</span></div>
        </div><div class="button-row"><button class="button" type="button" onclick="addProfile()">新增 Profile</button></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2>NAS Profiles</h2><p>登入狀態與操作只影響所選 NAS。</p></div><span id="profileCount" class="badge">0 profiles</span></div><div class="table-wrap"><table><thead><tr><th>名稱</th><th>URL</th><th>Revision</th><th>登入狀態</th><th>操作</th></tr></thead><tbody id="profiles"></tbody></table></div></div>
      </section>

      <section id="view-access" class="view" data-view="access" hidden>
        <div class="page-head"><div><h1>MCP 存取</h1><p>建立可撤銷、具 Scope 與 NAS allowlist 的遠端憑證。</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadTokens()">重新整理</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2>建立 MCP Token</h2><p>Allowlist 留空代表不允許存取任何 NAS。</p></div></div><div class="panel-body"><div class="form-grid"><div class="field"><label for="tokenName">Token 名稱</label><input class="control" id="tokenName" placeholder="例如 monitoring-agent"></div><div class="field"><label for="tokenNAS">NAS allowlist</label><input class="control" id="tokenNAS" placeholder="office, lab"></div></div><div class="field-label" style="margin-top:16px">Scopes</div><div class="scope-row"><label class="check-chip"><input type="checkbox" class="scope" value="nas.read" checked>nas.read</label><label class="check-chip"><input type="checkbox" class="scope" value="nas.plan">nas.plan</label><label class="check-chip"><input type="checkbox" class="scope" value="nas.apply">nas.apply</label><label class="check-chip"><input type="checkbox" class="scope" value="nas.admin">nas.admin</label></div><div class="button-row"><button class="button" type="button" onclick="createMCPToken()">建立 Token</button></div><div id="issued" class="secret" hidden></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2>已發行 Tokens</h2><p>Bearer token 只在建立或輪替後顯示一次。</p></div><span id="tokenCount" class="badge">0 tokens</span></div><div class="table-wrap"><table><thead><tr><th>名稱 / ID</th><th>Scope</th><th>NAS allowlist</th><th>狀態</th><th>操作</th></tr></thead><tbody id="tokens"></tbody></table></div></div>
      </section>

      <section id="view-approvals" class="view" data-view="approvals" hidden>
        <div class="page-head"><div><h1>高風險 Apply 核准</h1><p>核准綁定 Plan、NAS Profile revision 與 requesting token，且只能使用一次。</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadApprovals()">重新整理</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2>建立核准</h2><p>請從可信任的 out-of-band 流程取得完整欄位。</p></div></div><div class="panel-body"><div class="form-grid"><div class="field field-span-2"><label for="approvalHash">Plan SHA-256 hash</label><input class="control" id="approvalHash" placeholder="64 位 plan hash"></div><div class="field"><label for="approvalNAS">NAS Profile</label><input class="control" id="approvalNAS" placeholder="office"></div><div class="field"><label for="approvalRevision">Profile revision</label><input class="control" id="approvalRevision" type="number" min="1" placeholder="1"></div><div class="field field-span-2"><label for="approvalToken">Requesting token ID</label><input class="control" id="approvalToken" placeholder="Token ID"></div></div><div class="button-row"><button class="button" type="button" onclick="createApproval()">建立核准</button></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2>核准紀錄</h2><p>包含 ready、expired 與 consumed 狀態。</p></div><span id="approvalCount" class="badge">0 approvals</span></div><div class="table-wrap"><table><thead><tr><th>Plan / NAS</th><th>Requesting token</th><th>期限</th><th>狀態</th></tr></thead><tbody id="approvals"></tbody></table></div></div>
      </section>

      <section id="view-audit" class="view" data-view="audit" hidden>
        <div class="page-head"><div><h1>Audit</h1><p>查閱 Gateway 管理、Token、核准與遠端執行事件。</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadAudit()">重新整理</button><button class="button" type="button" onclick="exportAudit()">匯出 JSONL</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2>最近事件</h2><p>最多顯示最近 100 筆；秘密與 bearer token 不會進入 Audit。</p></div></div><div class="panel-body"><pre id="audit" class="audit-log">[]</pre></div></div>
      </section>

      <section id="view-admin" class="view" data-view="admin" hidden>
        <div class="page-head"><div><h1>Gateway 管理員</h1><p>管理本機密碼與目前的瀏覽器 Session。</p></div></div>
        <div class="content-grid"><div class="panel"><div class="panel-head"><div><h2>變更密碼</h2><p>更新後會撤銷其他管理員 Session。</p></div></div><div class="panel-body"><div class="form-grid"><div class="field"><label for="currentPassword">目前密碼</label><input class="control" id="currentPassword" type="password" autocomplete="current-password" placeholder="目前密碼"></div><div class="field"><label for="newPassword">新密碼</label><input class="control" id="newPassword" type="password" autocomplete="new-password" placeholder="至少 12 bytes"></div></div><div class="button-row"><button class="button" type="button" onclick="changePassword()">變更密碼</button></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2>目前 Session</h2><p>Gateway 管理員與 DSM 帳號互不相干。</p></div></div><div class="panel-body"><p id="sessionInfo" style="margin:0 0 16px;color:var(--muted)"></p><div class="button-row"><button class="button secondary" type="button" onclick="revokeOthers()">登出其他裝置</button><button class="button danger" type="button" onclick="logoutAdministrator()">登出</button></div></div></div></div>
      </section>
    </section>
  </div>
</main>

<script>
const $=id=>document.getElementById(id),adminBase=location.pathname.replace(/\/?$/,'/'),apiBase=adminBase+'api';
function show(value){let node=$('message'),text=value instanceof Error?value.message:String(value||'');node.textContent=text;node.hidden=!text;node.className='toast'+(value instanceof Error?' error':'')}
async function api(path,options={}){let method=(options.method||'GET').toUpperCase(),headers=Object.assign({},options.headers||{});if(method!=='GET'&&method!=='HEAD'){headers['Content-Type']='application/json';headers['X-DSMCTL-Request']='1'}let response=await fetch(apiBase+path,Object.assign({credentials:'same-origin'},options,{headers})),text=await response.text(),value={};if(text){try{value=JSON.parse(text)}catch{value={error:text}}}if(!response.ok){let error=new Error(value.error||response.statusText);error.status=response.status;throw error}return value}
function hideEntry(){for(const id of ['setupCard','expiredCard','loginCard','entryShell','application'])$(id).hidden=true}
function showEntry(id){hideEntry();$('entryShell').hidden=false;$(id).hidden=false}
function setView(name){for(const view of document.querySelectorAll('.view'))view.hidden=view.dataset.view!==name;for(const item of document.querySelectorAll('[data-nav]'))item.classList.toggle('active',item.dataset.nav===name);document.documentElement.scrollTop=0}
function formatDate(value){return value?new Date(value).toLocaleString():'—'}
function setMetric(id,value){$(id).textContent=String(value)}
function emptyRow(body,columns,title,detail){let row=body.insertRow();row.className='empty-row';let td=row.insertCell();td.colSpan=columns;let box=document.createElement('div');box.className='empty-state';let icon=document.createElement('span');icon.className='empty-icon';icon.textContent='◇';let strong=document.createElement('strong');strong.textContent=title;let span=document.createElement('span');span.textContent=detail;box.append(icon,strong,span);td.appendChild(box)}
function badge(text,kind=''){let item=document.createElement('span');item.className='badge'+(kind?' '+kind:'');item.textContent=text;return item}
function cell(row,text){let td=row.insertCell();td.textContent=text;return td}
function button(parent,label,fn,danger=false){let item=document.createElement('button');item.type='button';item.textContent=label;item.onclick=fn;item.className='table-action'+(danger?' danger':'');parent.classList.add('table-actions');parent.appendChild(item)}
function toggleFingerprint(){$('fingerprintField').hidden=$('tls').value!=='pinned_fingerprint'}

async function initialize(){hideEntry();try{let status=await api('/setup/status');if(status.state==='setup_available'){showEntry('setupCard');$('setupExpiry').textContent='設定期限 '+formatDate(status.setup_expires_at);return}if(status.state==='setup_expired'){showEntry('expiredCard');return}$('initializedAt').textContent=status.initialized_at?'此 Gateway 已於 '+formatDate(status.initialized_at)+' 初始化。':'請使用本機管理員帳號登入。';try{await showApplication()}catch(error){if(error.status===401)showEntry('loginCard');else throw error}}catch(error){show(error)}}
async function setupAdministrator(){if($('setupPassword').value!==$('setupConfirm').value){show(new Error('兩次輸入的密碼不一致'));return}try{await api('/setup',{method:'POST',body:JSON.stringify({username:$('setupUsername').value,password:$('setupPassword').value})});$('setupPassword').value=$('setupConfirm').value='';await showApplication()}catch(error){show(error)}}
async function loginAdministrator(){try{await api('/login',{method:'POST',body:JSON.stringify({username:$('loginUsername').value,password:$('loginPassword').value})});$('loginPassword').value='';await showApplication()}catch(error){$('loginPassword').value='';show(error)}}
async function showApplication(){let value=await api('/session'),session=value.session;hideEntry();$('application').hidden=false;$('sessionInfo').textContent=session.username+' · Session 到期 '+formatDate(session.expires_at);$('topUser').lastElementChild.textContent=session.username;$('topUser').querySelector('.user-avatar').textContent=session.username.slice(0,1).toUpperCase();setView('overview');show('');await Promise.allSettled([loadProfiles(),loadTokens(),loadApprovals(),loadAudit()])}
async function logoutAdministrator(){try{await api('/logout',{method:'POST',body:'{}'});await initialize()}catch(error){show(error)}}
async function changePassword(){try{await api('/password',{method:'PUT',body:JSON.stringify({current_password:$('currentPassword').value,new_password:$('newPassword').value})});$('currentPassword').value=$('newPassword').value='';show('密碼已更新，其他管理員 Session 已撤銷。')}catch(error){$('currentPassword').value=$('newPassword').value='';show(error)}}
async function revokeOthers(){try{await api('/sessions/revoke-others',{method:'POST',body:'{}'});show('其他管理員 Session 已撤銷。')}catch(error){show(error)}}

async function addProfile(){try{let pinned=$('tls').value==='pinned_fingerprint',fingerprint=$('fingerprint').value.trim(),confirmed=!pinned||confirm('確認信任此 SHA-256 certificate fingerprint？\n'+fingerprint);if(!confirmed)return;await api('/profiles',{method:'POST',body:JSON.stringify({name:$('name').value,url:$('url').value,username:$('username').value,tls_mode:$('tls').value,certificate_fingerprint:fingerprint,confirm_certificate_fingerprint:confirmed})});$('name').value=$('url').value=$('username').value=$('fingerprint').value='';$('tls').value='system_ca';toggleFingerprint();show('NAS Profile 已新增。請使用 Web Login 或密碼/OTP 完成該 NAS 的 DSM 登入。');await loadProfiles()}catch(error){show(error)}}
async function loadProfiles(){try{let value=await api('/profiles'),profiles=value.profiles||[],body=$('profiles');body.textContent='';$('profileCount').textContent=profiles.length+' profiles';setMetric('metricNAS',profiles.length);if(!profiles.length){emptyRow(body,5,'尚未新增 NAS','每台 NAS（包含 Host NAS）都必須建立獨立 Profile。');return profiles}for(const profile of profiles){let row=body.insertRow();cell(row,(profile.default?'★ ':'')+profile.name);cell(row,profile.url);cell(row,String(profile.revision));let status=cell(row,'');status.appendChild(profile.session_stored?badge('Web session','success'):profile.password_stored?badge('Password','success'):badge('尚未登入','warning'));let actions=cell(row,'');button(actions,'設為預設',()=>setDefault(profile.name));button(actions,'測試',()=>testNAS(profile.name));button(actions,'Web Login',()=>webLogin(profile.name));button(actions,'密碼/OTP',()=>passwordLogin(profile.name));button(actions,'刪除',()=>removeNAS(profile),true)}return profiles}catch(error){show(error);return[]}}
async function setDefault(name){try{await api('/profiles/'+encodeURIComponent(name)+'/default',{method:'POST',body:'{}'});await loadProfiles()}catch(error){show(error)}}
async function testNAS(name){try{show(JSON.stringify(await api('/profiles/'+encodeURIComponent(name)+'/test',{method:'POST',body:'{}'}),null,2))}catch(error){show(error)}}
async function passwordLogin(name){let password=prompt('DSM password（只用於這次 enrollment）');if(password===null)return;let otp=prompt('OTP（沒有就留空）')||'';try{await api('/profiles/'+encodeURIComponent(name)+'/credentials/password',{method:'POST',body:JSON.stringify({password,otp})});password=otp='';show('DSM 登入成功。');await loadProfiles()}catch(error){password=otp='';show(error)}}
async function webLogin(name){try{let start=await api('/profiles/'+encodeURIComponent(name)+'/weblogin/start',{method:'POST',body:'{}'}),popup=window.open(start.login_url,'dsmctl_signin','width=560,height=720'),listener=async event=>{if(event.origin!==start.nas_origin)return;let data=event.data||{};if(!data.code)return;window.removeEventListener('message',listener);try{await api('/profiles/'+encodeURIComponent(name)+'/weblogin/complete',{method:'POST',body:JSON.stringify({enrollment_id:start.enrollment_id,code:data.code,rs:data.rs,state:data.state||start.state})});if(popup)popup.close();show('DSM Web Login 已完成。');await loadProfiles()}catch(error){show(error)}};window.addEventListener('message',listener)}catch(error){show(error)}}
async function removeNAS(profile){if(!confirm('刪除 '+profile.name+' 及其 credentials？'))return;try{await api('/profiles/'+encodeURIComponent(profile.name)+'?revision='+profile.revision,{method:'DELETE',body:'{}'});show('NAS Profile 已刪除。');await loadProfiles()}catch(error){show(error)}}

async function createMCPToken(){try{let scopes=[...document.querySelectorAll('.scope:checked')].map(item=>item.value),nas=$('tokenNAS').value.split(',').map(item=>item.trim()).filter(Boolean),value=await api('/mcp-tokens',{method:'POST',body:JSON.stringify({name:$('tokenName').value,scopes,nas_allowlist:nas})});$('issued').hidden=false;$('issued').textContent='請立即保存，之後不會再次顯示：\n'+value.bearer_token;await loadTokens()}catch(error){show(error)}}
async function loadTokens(){try{let value=await api('/mcp-tokens'),tokens=value.tokens||[],body=$('tokens');body.textContent='';$('tokenCount').textContent=tokens.length+' tokens';setMetric('metricTokens',tokens.filter(item=>!item.revoked_at&&(!item.expires_at||new Date(item.expires_at)>new Date())).length);if(!tokens.length){emptyRow(body,5,'尚未建立 MCP Token','建立具 Scope 與 NAS allowlist 的遠端存取憑證。');return tokens}for(const token of tokens){let row=body.insertRow();cell(row,token.name+'\n'+token.id);cell(row,token.scopes.join(', '));cell(row,token.nas_allowlist.join(', ')||'—');let status=cell(row,'');status.appendChild(token.revoked_at?badge('revoked'):token.expires_at?badge('expires '+formatDate(token.expires_at),'warning'):badge('active','success'));let actions=cell(row,'');button(actions,'輪替',()=>rotateToken(token.id));button(actions,'撤銷',()=>revokeToken(token.id),true)}return tokens}catch(error){show(error);return[]}}
async function rotateToken(id){if(!confirm('舊 Token 會立即失效，確定輪替？'))return;try{let value=await api('/mcp-tokens/'+id+'/rotate',{method:'POST',body:'{}'});$('issued').hidden=false;$('issued').textContent='請立即保存新 Token：\n'+value.bearer_token;await loadTokens()}catch(error){show(error)}}
async function revokeToken(id){if(!confirm('確定撤銷？'))return;try{await api('/mcp-tokens/'+id,{method:'DELETE',body:'{}'});show('MCP Token 已撤銷。');await loadTokens()}catch(error){show(error)}}

async function createApproval(){try{await api('/approvals',{method:'POST',body:JSON.stringify({plan_hash:$('approvalHash').value,nas:$('approvalNAS').value,profile_revision:Number($('approvalRevision').value),requesting_token_id:$('approvalToken').value})});show('高風險 Apply 核准已建立。');await loadApprovals()}catch(error){show(error)}}
async function loadApprovals(){try{let value=await api('/approvals?include_consumed=true'),approvals=value.approvals||[],body=$('approvals');body.textContent='';$('approvalCount').textContent=approvals.length+' approvals';setMetric('metricApprovals',approvals.filter(item=>!item.consumed_at&&new Date(item.expires_at)>new Date()).length);if(!approvals.length){emptyRow(body,4,'目前沒有核准','高風險 remote apply 必須先在這裡建立一次性核准。');return approvals}for(const approval of approvals){let row=body.insertRow();cell(row,approval.plan_hash+'\n'+approval.nas+' @ '+approval.profile_revision);cell(row,approval.requesting_token_id);cell(row,formatDate(approval.expires_at));let state=approval.consumed_at?'consumed':new Date(approval.expires_at)<new Date()?'expired':'ready',status=cell(row,'');status.appendChild(badge(state,state==='ready'?'success':state==='expired'?'warning':''))}return approvals}catch(error){show(error);return[]}}
async function loadAudit(){try{let events=(await api('/audit?limit=100')).events||[];$('audit').textContent=JSON.stringify(events,null,2);return events}catch(error){show(error);return[]}}
async function exportAudit(){try{let response=await fetch(apiBase+'/audit/export?limit=1000',{credentials:'same-origin'});if(!response.ok)throw new Error('匯出失敗');let target=URL.createObjectURL(await response.blob()),link=document.createElement('a');link.href=target;link.download='dsmctl-audit.jsonl';link.click();URL.revokeObjectURL(target)}catch(error){show(error)}}
initialize();
</script>
</body>
</html>`
