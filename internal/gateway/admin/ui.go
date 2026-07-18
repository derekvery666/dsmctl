package admin

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>dsmctl MCP Server</title>
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
.locale-row{display:flex;justify-content:flex-end;margin-bottom:12px}
.locale-select{min-height:32px;width:122px;padding:5px 28px 5px 9px;border:1px solid var(--line-strong);border-radius:7px;background:#fff;color:#44546a;font-size:12px}
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
.endpoint{display:inline-flex;align-items:center;gap:9px;margin-top:16px;padding:7px 10px;border:1px solid #c8e1f7;border-radius:7px;background:rgba(255,255,255,.72);color:#48657e;font-size:11px}
.endpoint code{color:#0c70c9;font:600 12px/1.2 ui-monospace,SFMono-Regular,Consolas,monospace}
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
  .workspace{padding:22px 14px 40px}.page-head{display:block}.page-actions{margin-top:14px}.hero{min-height:0;padding:24px}.hero-icon{display:none}.hero-copy h2{font-size:22px}.stats{grid-template-columns:1fr}.form-grid,.inline-form{grid-template-columns:1fr}.field-span-2{grid-column:auto}.panel-head{padding:16px}.panel-body{padding:16px}.topbar{padding:0 12px}.topbar .brand-copy span{display:none}.topbar .locale-select{width:84px}.user-button>span:last-child{display:none}
}
</style>
</head>
<body>
<div id="message" class="toast" role="status" aria-live="polite" hidden></div>

<section id="entryShell" class="auth-shell">
  <div class="auth-visual" aria-hidden="true">
    <div class="brand"><span class="brand-mark"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl MCP Server</strong><span data-i18n="brandSubtitle">NAS management gateway</span></span></div>
    <div class="auth-message">
      <div class="auth-kicker" data-i18n="authKicker">MCP Server for Synology NAS</div>
      <h1 data-i18n="authHeadline">Manage multiple NAS systems through MCP.</h1>
      <p data-i18n="authDescription">Manage NAS connections, MCP tokens, approvals, and audit records from one server.</p>
    </div>
    <div class="auth-foot"><span data-i18n="secureSession">Secure admin session</span><span data-i18n="encryptedState">Encrypted state</span><span data-i18n="scopedAccess">Scoped MCP access</span></div>
  </div>
  <div class="auth-panel">
    <div class="auth-panel-inner">
      <div class="locale-row"><select class="locale-select" data-locale-select onchange="changeLocale(this.value)" data-i18n-aria="language"><option value="en">English</option><option value="zh-TW">繁體中文</option><option value="zh-CN">简体中文</option><option value="ja">日本語</option><option value="de">Deutsch</option></select></div>
      <div class="brand auth-panel-brand"><span class="brand-mark"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl MCP Server</strong><span data-i18n="brandSubtitle">NAS management gateway</span></span></div>
      <section id="setupCard" class="auth-card" hidden>
        <h2 data-i18n="setupTitle">Create MCP Server administrator</h2>
        <p class="lead" data-i18n="setupLead">This account manages the MCP Server only. It has no DSM or host NAS permissions.</p>
        <p id="setupExpiry" class="auth-deadline"></p>
        <div class="field"><label for="setupUsername" data-i18n="adminUsername">Administrator username</label><input class="control" id="setupUsername" autocomplete="username" data-i18n-placeholder="usernamePlaceholder" placeholder="3–64 characters"></div>
        <div class="field"><label for="setupPassword" data-i18n="password">Password</label><input class="control" id="setupPassword" type="password" autocomplete="new-password" data-i18n-placeholder="newPasswordPlaceholder" placeholder="At least 12 bytes"></div>
        <div class="field"><label for="setupConfirm" data-i18n="confirmPassword">Confirm password</label><input class="control" id="setupConfirm" type="password" autocomplete="new-password" data-i18n-placeholder="confirmPlaceholder" placeholder="Enter the password again"></div>
        <div class="button-row"><button class="button full" type="button" onclick="setupAdministrator()" data-i18n="createAdministrator">Create administrator</button></div>
        <p class="helper" data-i18n="setupHelper">Complete setup within one hour of startup. Restart the service if the window expires.</p>
      </section>
      <section id="expiredCard" class="auth-card" hidden>
        <h2 data-i18n="expiredTitle">Setup window expired</h2>
        <p class="lead" data-i18n="expiredLead">The uninitialized MCP Server has closed its setup endpoint.</p>
        <div class="notice warning"><strong data-i18n="continueTitle">Next step</strong><span data-i18n="continueDetail">Restart the uninitialized container or package to open a new one-hour window.</span></div>
      </section>
      <section id="loginCard" class="auth-card" hidden>
        <h2 data-i18n="loginTitle">Sign in to MCP Server</h2>
        <p id="initializedAt" class="lead"></p>
        <div class="field"><label for="loginUsername" data-i18n="adminUsername">Administrator username</label><input class="control" id="loginUsername" autocomplete="username" data-i18n-placeholder="adminPlaceholder" placeholder="MCP Server administrator"></div>
        <div class="field"><label for="loginPassword" data-i18n="password">Password</label><input class="control" id="loginPassword" type="password" autocomplete="current-password" data-i18n-placeholder="passwordPlaceholder" placeholder="Enter password"></div>
        <div class="button-row"><button class="button full" type="button" onclick="loginAdministrator()" data-i18n="signIn">Sign in</button></div>
        <p class="warning-copy" data-i18n="unexpectedInit">If you did not initialize this server, stop using it and reset the deployment data. Resetting deletes all NAS sessions.</p>
      </section>
    </div>
  </div>
</section>

<main id="application" hidden>
  <div class="app-shell">
    <header class="topbar">
      <div class="brand"><span class="brand-mark"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl MCP Server</strong><span data-i18n="brandSubtitle">NAS management gateway</span></span></div>
      <div class="topbar-right"><select class="locale-select" data-locale-select onchange="changeLocale(this.value)" data-i18n-aria="language"><option value="en">English</option><option value="zh-TW">繁體中文</option><option value="zh-CN">简体中文</option><option value="ja">日本語</option><option value="de">Deutsch</option></select><span class="online-pill" data-i18n="serverOnline">MCP Server online</span><button id="topUser" class="user-button" type="button" onclick="setView('admin')"><span class="user-avatar">A</span><span data-i18n="administrator">Administrator</span></button></div>
    </header>
    <aside class="sidebar" aria-label="MCP Server navigation" data-i18n-aria="navigationAria">
      <div class="nav-label" data-i18n="management">Management</div>
      <nav class="nav-list">
        <button class="nav-item active" data-nav="overview" type="button" onclick="setView('overview')"><svg viewBox="0 0 24 24"><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z"/></svg><span data-i18n="overview">Overview</span></button>
        <button class="nav-item" data-nav="nas" type="button" onclick="setView('nas')"><svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01M11 7.5h6M11 16.5h6"/></svg><span>NAS</span></button>
        <button class="nav-item" data-nav="access" type="button" onclick="setView('access')"><svg viewBox="0 0 24 24"><circle cx="8" cy="12" r="4"/><path d="M12 12h9M17 12v3M20 12v2"/></svg><span data-i18n="mcpAccess">MCP access</span></button>
        <button class="nav-item" data-nav="approvals" type="button" onclick="setView('approvals')"><svg viewBox="0 0 24 24"><path d="M12 3l8 4v5c0 5-3.4 8-8 9-4.6-1-8-4-8-9V7z"/><path d="M8.5 12l2.2 2.2 4.8-5"/></svg><span data-i18n="approvals">Approvals</span></button>
        <button class="nav-item" data-nav="audit" type="button" onclick="setView('audit')"><svg viewBox="0 0 24 24"><path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5M9 12h7M9 16h7"/></svg><span>Audit</span></button>
        <button class="nav-item" data-nav="admin" type="button" onclick="setView('admin')"><svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="4"/><path d="M4 21c.8-4 3.4-6 8-6s7.2 2 8 6"/></svg><span data-i18n="administrator">Administrator</span></button>
      </nav>
      <div class="sidebar-foot"><strong data-i18n="trustBoundary">Independent trust boundary</strong><span data-i18n="trustBoundaryDetail">Every NAS requires a separate profile and DSM sign-in.</span></div>
    </aside>

    <section class="workspace">
      <section id="view-overview" class="view" data-view="overview">
        <div class="page-head"><div><h1 data-i18n="overviewTitle">MCP Server overview</h1><p data-i18n="overviewSubtitle">Service, NAS connection, and access status.</p></div><div class="page-actions"><button class="button" type="button" onclick="setView('nas')" data-i18n="addNAS">Add NAS</button></div></div>
        <div class="hero"><div class="hero-copy"><div class="eyebrow">MCP SERVER</div><h2 data-i18n="heroTitle">Synology NAS MCP Server</h2><p data-i18n="heroDetail">Manage multiple NAS profiles through one MCP endpoint. DSM sessions, token scopes, and approvals remain independent.</p><div class="endpoint"><span data-i18n="mcpEndpoint">MCP endpoint</span><code>/mcp</code></div></div><div class="hero-icon"><span class="brand-mark"><i></i><i></i><i></i><i></i></span></div></div>
        <div class="stats">
          <div class="stat"><span class="stat-icon"><svg viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="6" rx="2"/><rect x="3" y="13" width="18" height="6" rx="2"/></svg></span><div><strong id="metricNAS">0</strong><span data-i18n="nasProfilesMetric">NAS profiles</span></div></div>
          <div class="stat"><span class="stat-icon"><svg viewBox="0 0 24 24"><circle cx="8" cy="12" r="4"/><path d="M12 12h9M17 12v3"/></svg></span><div><strong id="metricTokens">0</strong><span data-i18n="activeTokens">Active MCP tokens</span></div></div>
          <div class="stat"><span class="stat-icon"><svg viewBox="0 0 24 24"><path d="M12 3l8 4v5c0 5-3.4 8-8 9-4.6-1-8-4-8-9V7z"/></svg></span><div><strong id="metricApprovals">0</strong><span data-i18n="readyApprovals">Ready approvals</span></div></div>
        </div>
        <div class="content-grid">
          <div class="panel"><div class="panel-head"><div><h2 data-i18n="nasConnections">NAS connections</h2><p data-i18n="nasConnectionsDetail">Add a NAS profile, then sign in to DSM.</p></div></div><div class="panel-body"><div class="notice"><strong data-i18n="noImplicitHost">Host NAS is not added automatically</strong><span data-i18n="noImplicitHostDetail">Add the hosting NAS by LAN IP or DNS name. Container localhost is not the host NAS.</span></div><div class="button-row"><button class="button" type="button" onclick="setView('nas')" data-i18n="manageNAS">Manage NAS</button><button class="button secondary" type="button" onclick="setView('access')" data-i18n="configureMCP">Configure MCP access</button></div></div></div>
          <div class="panel"><div class="panel-head"><div><h2 data-i18n="quickLinks">Quick links</h2><p data-i18n="quickLinksDetail">Administration sections.</p></div></div><div class="panel-body quick-list"><button class="quick-action" type="button" onclick="setView('nas')"><span data-i18n="nasProfiles">NAS profiles</span><span>›</span></button><button class="quick-action" type="button" onclick="setView('approvals')"><span data-i18n="highRiskApprovals">High-risk approvals</span><span>›</span></button><button class="quick-action" type="button" onclick="setView('audit')"><span data-i18n="auditRecords">Audit records</span><span>›</span></button></div></div>
        </div>
      </section>

      <section id="view-nas" class="view" data-view="nas" hidden>
        <div class="page-head"><div><h1 data-i18n="nasTitle">NAS management</h1><p data-i18n="nasSubtitle">Create a profile and sign in to each NAS separately.</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadProfiles()" data-i18n="refresh">Refresh</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="addNAS">Add NAS</h2><p data-i18n="addNASDetail">Use a LAN IP or DNS name reachable from the MCP Server container.</p></div></div><div class="panel-body"><div class="form-grid">
          <div class="field"><label for="name" data-i18n="profileName">Profile name</label><input class="control" id="name" data-i18n-placeholder="profileExample" placeholder="Example: office"></div>
          <div class="field"><label for="url">DSM URL</label><input class="control" id="url" placeholder="https://nas.example:5001"></div>
          <div class="field"><label for="username" data-i18n="dsmAccount">DSM account</label><input class="control" id="username" placeholder="operator"></div>
          <div class="field"><label for="tls" data-i18n="tlsVerification">TLS verification</label><select class="control" id="tls" onchange="toggleFingerprint()"><option value="system_ca">System CA</option><option value="pinned_fingerprint">Pinned fingerprint</option></select></div>
          <div id="fingerprintField" class="field field-span-2" hidden><label for="fingerprint">SHA-256 certificate fingerprint</label><input class="control" id="fingerprint" data-i18n-placeholder="fingerprintPlaceholder" placeholder="64-character hexadecimal fingerprint"><span class="field-hint" data-i18n="fingerprintHint">Pinned fingerprints require explicit trust confirmation.</span></div>
        </div><div class="button-row"><button class="button" type="button" onclick="addProfile()" data-i18n="addProfile">Add profile</button></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="nasProfiles">NAS profiles</h2><p data-i18n="nasProfilesDetail">Sign-in state and actions apply only to the selected NAS.</p></div><span id="profileCount" class="badge">0 profiles</span></div><div class="table-wrap"><table><thead><tr><th data-i18n="name">Name</th><th>URL</th><th>Revision</th><th data-i18n="loginStatus">Sign-in status</th><th data-i18n="actions">Actions</th></tr></thead><tbody id="profiles"></tbody></table></div></div>
      </section>

      <section id="view-access" class="view" data-view="access" hidden>
        <div class="page-head"><div><h1 data-i18n="accessTitle">MCP access</h1><p data-i18n="accessSubtitle">Create tokens for the /mcp endpoint and restrict NAS and operation scopes.</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadTokens()" data-i18n="refresh">Refresh</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="createToken">Create MCP token</h2><p data-i18n="createTokenDetail">An empty allowlist grants access to no NAS systems.</p></div></div><div class="panel-body"><div class="form-grid"><div class="field"><label for="tokenName" data-i18n="tokenName">Token name</label><input class="control" id="tokenName" data-i18n-placeholder="tokenExample" placeholder="Example: monitoring-agent"></div><div class="field"><label for="tokenNAS">NAS allowlist</label><input class="control" id="tokenNAS" placeholder="office, lab"></div></div><div class="field-label" style="margin-top:16px">Scopes</div><div class="scope-row"><label class="check-chip"><input type="checkbox" class="scope" value="nas.read" checked>nas.read</label><label class="check-chip"><input type="checkbox" class="scope" value="nas.plan">nas.plan</label><label class="check-chip"><input type="checkbox" class="scope" value="nas.apply">nas.apply</label><label class="check-chip"><input type="checkbox" class="scope" value="nas.admin">nas.admin</label></div><div class="button-row"><button class="button" type="button" onclick="createMCPToken()" data-i18n="createTokenButton">Create token</button></div><div id="issued" class="secret" hidden></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="issuedTokens">Issued tokens</h2><p data-i18n="issuedTokensDetail">The bearer token is shown once after creation or rotation.</p></div><span id="tokenCount" class="badge">0 tokens</span></div><div class="table-wrap"><table><thead><tr><th data-i18n="nameID">Name / ID</th><th>Scope</th><th>NAS allowlist</th><th data-i18n="status">Status</th><th data-i18n="actions">Actions</th></tr></thead><tbody id="tokens"></tbody></table></div></div>
      </section>

      <section id="view-approvals" class="view" data-view="approvals" hidden>
        <div class="page-head"><div><h1 data-i18n="approvalTitle">High-risk apply approvals</h1><p data-i18n="approvalSubtitle">Each approval is bound to a plan, NAS profile revision, and requesting token.</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadApprovals()" data-i18n="refresh">Refresh</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="createApproval">Create approval</h2><p data-i18n="createApprovalDetail">Enter values obtained through a trusted out-of-band process.</p></div></div><div class="panel-body"><div class="form-grid"><div class="field field-span-2"><label for="approvalHash">Plan SHA-256 hash</label><input class="control" id="approvalHash" data-i18n-placeholder="planHashPlaceholder" placeholder="64-character plan hash"></div><div class="field"><label for="approvalNAS">NAS profile</label><input class="control" id="approvalNAS" placeholder="office"></div><div class="field"><label for="approvalRevision" data-i18n="profileRevision">Profile revision</label><input class="control" id="approvalRevision" type="number" min="1" placeholder="1"></div><div class="field field-span-2"><label for="approvalToken" data-i18n="requestingToken">Requesting token ID</label><input class="control" id="approvalToken" placeholder="Token ID"></div></div><div class="button-row"><button class="button" type="button" onclick="createApproval()" data-i18n="createApprovalButton">Create approval</button></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="approvalRecords">Approval records</h2><p data-i18n="approvalRecordsDetail">Includes ready, expired, and consumed approvals.</p></div><span id="approvalCount" class="badge">0 approvals</span></div><div class="table-wrap"><table><thead><tr><th>Plan / NAS</th><th data-i18n="requestingToken">Requesting token ID</th><th data-i18n="expires">Expires</th><th data-i18n="status">Status</th></tr></thead><tbody id="approvals"></tbody></table></div></div>
      </section>

      <section id="view-audit" class="view" data-view="audit" hidden>
        <div class="page-head"><div><h1>Audit</h1><p data-i18n="auditSubtitle">Review MCP Server administration, token, approval, and remote execution events.</p></div><div class="page-actions"><button class="button secondary" type="button" onclick="loadAudit()" data-i18n="refresh">Refresh</button><button class="button" type="button" onclick="exportAudit()" data-i18n="exportJSONL">Export JSONL</button></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="recentEvents">Recent events</h2><p data-i18n="recentEventsDetail">Shows up to 100 events. Secrets and bearer tokens are excluded.</p></div></div><div class="panel-body"><pre id="audit" class="audit-log">[]</pre></div></div>
      </section>

      <section id="view-admin" class="view" data-view="admin" hidden>
        <div class="page-head"><div><h1 data-i18n="adminTitle">MCP Server administrator</h1><p data-i18n="adminSubtitle">Manage the local password and browser sessions.</p></div></div>
        <div class="content-grid"><div class="panel"><div class="panel-head"><div><h2 data-i18n="changePassword">Change password</h2><p data-i18n="changePasswordDetail">Changing the password revokes other administrator sessions.</p></div></div><div class="panel-body"><div class="form-grid"><div class="field"><label for="currentPassword" data-i18n="currentPassword">Current password</label><input class="control" id="currentPassword" type="password" autocomplete="current-password" data-i18n-placeholder="currentPassword" placeholder="Current password"></div><div class="field"><label for="newPassword" data-i18n="newPassword">New password</label><input class="control" id="newPassword" type="password" autocomplete="new-password" data-i18n-placeholder="newPasswordPlaceholder" placeholder="At least 12 bytes"></div></div><div class="button-row"><button class="button" type="button" onclick="changePassword()" data-i18n="changePassword">Change password</button></div></div></div>
        <div class="panel"><div class="panel-head"><div><h2 data-i18n="currentSession">Current session</h2><p data-i18n="currentSessionDetail">MCP Server administrator and DSM accounts are independent.</p></div></div><div class="panel-body"><p id="sessionInfo" style="margin:0 0 16px;color:var(--muted)"></p><div class="button-row"><button class="button secondary" type="button" onclick="revokeOthers()" data-i18n="revokeOthers">Sign out other devices</button><button class="button danger" type="button" onclick="logoutAdministrator()" data-i18n="signOut">Sign out</button></div></div></div></div>
      </section>
    </section>
  </div>
</main>

<script>
const $=id=>document.getElementById(id),adminBase=location.pathname.replace(/\/?$/,'/'),apiBase=adminBase+'api',supportedLocales=['en','zh-TW','zh-CN','ja','de'];
const translations={
en:{
brandSubtitle:'NAS management gateway',authKicker:'MCP Server for Synology NAS',authHeadline:'Manage multiple NAS systems through MCP.',authDescription:'Manage NAS connections, MCP tokens, approvals, and audit records.',secureSession:'HttpOnly/SameSite session',encryptedState:'Encrypted state',scopedAccess:'Scoped MCP access',language:'Language',
setupTitle:'Create MCP Server administrator',setupLead:'This account manages the MCP Server only. It has no DSM or host NAS permissions.',adminUsername:'Administrator username',usernamePlaceholder:'3–64 characters',password:'Password',newPasswordPlaceholder:'At least 12 bytes',confirmPassword:'Confirm password',confirmPlaceholder:'Enter the password again',createAdministrator:'Create administrator',setupHelper:'Complete setup within one hour of startup. Restart the service if the window expires.',
expiredTitle:'Setup window expired',expiredLead:'The uninitialized MCP Server has closed its setup endpoint.',continueTitle:'Next step',continueDetail:'Restart the uninitialized container or package to open a new one-hour window.',loginTitle:'Sign in to MCP Server',adminPlaceholder:'MCP Server administrator',passwordPlaceholder:'Enter password',signIn:'Sign in',unexpectedInit:'If you did not initialize this server, stop using it and reset the deployment data. Resetting deletes all NAS sessions.',
serverOnline:'MCP Server online',administrator:'Administrator',navigationAria:'MCP Server navigation',management:'Management',overview:'Overview',mcpAccess:'MCP access',approvals:'Approvals',trustBoundary:'Independent trust boundary',trustBoundaryDetail:'Every NAS requires a separate profile and DSM sign-in.',
overviewTitle:'MCP Server overview',overviewSubtitle:'Service, NAS connection, and access status.',addNAS:'Add NAS',heroTitle:'Synology NAS MCP Server',heroDetail:'Manage multiple NAS profiles through one MCP endpoint. DSM sessions, token scopes, and approvals remain independent.',mcpEndpoint:'MCP endpoint',nasProfilesMetric:'NAS profiles',activeTokens:'Active MCP tokens',readyApprovals:'Ready approvals',nasConnections:'NAS connections',nasConnectionsDetail:'Add a NAS profile, then sign in to DSM.',noImplicitHost:'Host NAS is not added automatically',noImplicitHostDetail:'Add the hosting NAS by LAN IP or DNS name. Container localhost is not the host NAS.',manageNAS:'Manage NAS',configureMCP:'Configure MCP access',quickLinks:'Quick links',quickLinksDetail:'Administration sections.',nasProfiles:'NAS profiles',highRiskApprovals:'High-risk approvals',auditRecords:'Audit records',
nasTitle:'NAS management',nasSubtitle:'Create a profile and sign in to each NAS separately.',refresh:'Refresh',addNASDetail:'Use a LAN IP or DNS name reachable from the MCP Server container.',profileName:'Profile name',profileExample:'Example: office',dsmAccount:'DSM account',tlsVerification:'TLS verification',fingerprintPlaceholder:'64-character hexadecimal fingerprint',fingerprintHint:'Pinned fingerprints require explicit trust confirmation.',addProfile:'Add profile',nasProfilesDetail:'Sign-in state and actions apply only to the selected NAS.',name:'Name',loginStatus:'Sign-in status',actions:'Actions',
accessTitle:'MCP access',accessSubtitle:'Create tokens for the /mcp endpoint and restrict NAS and operation scopes.',createToken:'Create MCP token',createTokenDetail:'An empty allowlist grants access to no NAS systems.',tokenName:'Token name',tokenExample:'Example: monitoring-agent',createTokenButton:'Create token',issuedTokens:'Issued tokens',issuedTokensDetail:'The bearer token is shown once after creation or rotation.',nameID:'Name / ID',status:'Status',
approvalTitle:'High-risk apply approvals',approvalSubtitle:'Each approval is bound to a plan, NAS profile revision, and requesting token.',createApproval:'Create approval',createApprovalDetail:'Enter values obtained through a trusted out-of-band process.',planHashPlaceholder:'64-character plan hash',profileRevision:'Profile revision',requestingToken:'Requesting token ID',createApprovalButton:'Create approval',approvalRecords:'Approval records',approvalRecordsDetail:'Includes ready, expired, and consumed approvals.',expires:'Expires',
auditSubtitle:'Review MCP Server administration, token, approval, and remote execution events.',exportJSONL:'Export JSONL',recentEvents:'Recent events',recentEventsDetail:'Shows up to 100 events. Secrets and bearer tokens are excluded.',adminTitle:'MCP Server administrator',adminSubtitle:'Manage the local password and browser sessions.',changePassword:'Change password',changePasswordDetail:'Changing the password revokes other administrator sessions.',currentPassword:'Current password',newPassword:'New password',currentSession:'Current session',currentSessionDetail:'MCP Server administrator and DSM accounts are independent.',revokeOthers:'Sign out other devices',signOut:'Sign out',
setupDeadline:'Setup deadline: {date}',initializedAt:'Initialized: {date}',loginFallback:'Sign in with the local administrator account.',sessionExpires:'{username} · Session expires {date}',passwordMismatch:'Passwords do not match.',passwordUpdated:'Password updated. Other administrator sessions were revoked.',otherSessionsRevoked:'Other administrator sessions were revoked.',trustFingerprint:'Trust this SHA-256 certificate fingerprint?\n{fingerprint}',profileAdded:'NAS profile added. Complete DSM sign-in with Web Login or password/OTP.',profilesCount:'{count} profiles',emptyProfilesTitle:'No NAS profiles',emptyProfilesDetail:'Add every NAS, including the host NAS, as a separate profile.',webSession:'Web session',storedPassword:'Password',notSignedIn:'Not signed in',setDefault:'Set default',test:'Test',webLogin:'Web Login',passwordOTP:'Password/OTP',delete:'Delete',dsmPasswordPrompt:'DSM password for this enrollment',otpPrompt:'OTP, or leave blank',dsmLoginSuccess:'DSM sign-in completed.',dsmWebLoginSuccess:'DSM Web Login completed.',deleteProfileConfirm:'Delete {name} and its credentials?',profileDeleted:'NAS profile deleted.',
saveToken:'Save this token now. It will not be shown again:\n{token}',tokensCount:'{count} tokens',emptyTokensTitle:'No MCP tokens',emptyTokensDetail:'Create a scoped token with a NAS allowlist.',revoked:'Revoked',expiresOn:'Expires {date}',active:'Active',rotate:'Rotate',revoke:'Revoke',rotateConfirm:'Rotate this token? The current token becomes invalid immediately.',saveNewToken:'Save the new token now:\n{token}',revokeConfirm:'Revoke this token?',tokenRevoked:'MCP token revoked.',approvalCreated:'High-risk apply approval created.',approvalsCount:'{count} approvals',emptyApprovalsTitle:'No approvals',emptyApprovalsDetail:'High-risk remote apply requires a one-time approval.',consumed:'Consumed',expired:'Expired',ready:'Ready',exportFailed:'Export failed.'
},
'zh-TW':{
brandSubtitle:'NAS 管理閘道',authKicker:'Synology NAS 的 MCP Server',authHeadline:'透過 MCP 管理多台 NAS。',authDescription:'管理 NAS 連線、MCP Token、核准與 Audit 紀錄。',secureSession:'HttpOnly/SameSite 工作階段',encryptedState:'加密狀態',scopedAccess:'MCP 範圍控制',language:'語言',
setupTitle:'建立 MCP Server 管理員',setupLead:'此帳號僅管理 MCP Server，不具備 DSM 或 Host NAS 權限。',adminUsername:'管理員帳號',usernamePlaceholder:'3–64 個字元',password:'密碼',newPasswordPlaceholder:'至少 12 bytes',confirmPassword:'確認密碼',confirmPlaceholder:'再次輸入密碼',createAdministrator:'建立管理員',setupHelper:'請在啟動後一小時內完成設定；逾時請重新啟動服務。',
expiredTitle:'設定期限已過',expiredLead:'尚未初始化的 MCP Server 已關閉設定端點。',continueTitle:'下一步',continueDetail:'重新啟動未初始化的 container 或套件，以開放新的 1 小時設定期限。',loginTitle:'登入 MCP Server',adminPlaceholder:'MCP Server 管理員',passwordPlaceholder:'輸入密碼',signIn:'登入',unexpectedInit:'若不是由你完成初始化，請停止使用並重設部署資料。重設會刪除所有 NAS Session。',
serverOnline:'MCP Server 正常',administrator:'管理員',navigationAria:'MCP Server 管理導覽',management:'管理',overview:'總覽',mcpAccess:'MCP 存取',approvals:'核准',trustBoundary:'獨立信任邊界',trustBoundaryDetail:'每台 NAS 都必須建立個別 Profile 並登入 DSM。',
overviewTitle:'MCP Server 總覽',overviewSubtitle:'服務、NAS 連線與存取權限狀態。',addNAS:'新增 NAS',heroTitle:'Synology NAS MCP Server',heroDetail:'透過單一 MCP 端點管理多個 NAS Profile。DSM Session、Token Scope 與核准彼此獨立。',mcpEndpoint:'MCP 端點',nasProfilesMetric:'NAS Profile',activeTokens:'有效 MCP Token',readyApprovals:'可用核准',nasConnections:'NAS 連線',nasConnectionsDetail:'新增 NAS Profile 後，再登入 DSM。',noImplicitHost:'Host NAS 不會自動加入',noImplicitHostDetail:'安裝本服務的 NAS 也必須使用 LAN IP 或 DNS 名稱新增；container localhost 不是 Host NAS。',manageNAS:'管理 NAS',configureMCP:'設定 MCP 存取',quickLinks:'快速連結',quickLinksDetail:'管理功能。',nasProfiles:'NAS Profile',highRiskApprovals:'高風險核准',auditRecords:'Audit 紀錄',
nasTitle:'NAS 管理',nasSubtitle:'建立 Profile，並為每台 NAS 個別登入 DSM。',refresh:'重新整理',addNASDetail:'使用 MCP Server container 可連線的 LAN IP 或 DNS 名稱。',profileName:'Profile 名稱',profileExample:'例如 office',dsmAccount:'DSM 帳號',tlsVerification:'TLS 驗證',fingerprintPlaceholder:'64 位十六進位 fingerprint',fingerprintHint:'使用 pinned fingerprint 時必須明確確認信任。',addProfile:'新增 Profile',nasProfilesDetail:'登入狀態與操作只影響所選 NAS。',name:'名稱',loginStatus:'登入狀態',actions:'操作',
accessTitle:'MCP 存取',accessSubtitle:'建立用於 /mcp 端點的 Token，並限制 NAS 與操作範圍。',createToken:'建立 MCP Token',createTokenDetail:'Allowlist 留空時不允許存取任何 NAS。',tokenName:'Token 名稱',tokenExample:'例如 monitoring-agent',createTokenButton:'建立 Token',issuedTokens:'已發行 Token',issuedTokensDetail:'Bearer Token 只在建立或輪替後顯示一次。',nameID:'名稱 / ID',status:'狀態',
approvalTitle:'高風險 Apply 核准',approvalSubtitle:'每筆核准綁定 Plan、NAS Profile revision 與 Requesting Token。',createApproval:'建立核准',createApprovalDetail:'輸入由可信任 out-of-band 流程取得的欄位。',planHashPlaceholder:'64 位 Plan hash',profileRevision:'Profile revision',requestingToken:'Requesting Token ID',createApprovalButton:'建立核准',approvalRecords:'核准紀錄',approvalRecordsDetail:'包含 ready、expired 與 consumed 狀態。',expires:'期限',
auditSubtitle:'查閱 MCP Server 管理、Token、核准與遠端執行事件。',exportJSONL:'匯出 JSONL',recentEvents:'最近事件',recentEventsDetail:'最多顯示 100 筆；不包含秘密與 Bearer Token。',adminTitle:'MCP Server 管理員',adminSubtitle:'管理本機密碼與瀏覽器 Session。',changePassword:'變更密碼',changePasswordDetail:'變更密碼會撤銷其他管理員 Session。',currentPassword:'目前密碼',newPassword:'新密碼',currentSession:'目前 Session',currentSessionDetail:'MCP Server 管理員與 DSM 帳號互相獨立。',revokeOthers:'登出其他裝置',signOut:'登出',
setupDeadline:'設定期限：{date}',initializedAt:'初始化時間：{date}',loginFallback:'請使用本機管理員帳號登入。',sessionExpires:'{username} · Session 到期時間 {date}',passwordMismatch:'兩次輸入的密碼不一致。',passwordUpdated:'密碼已更新，其他管理員 Session 已撤銷。',otherSessionsRevoked:'其他管理員 Session 已撤銷。',trustFingerprint:'確認信任此 SHA-256 certificate fingerprint？\n{fingerprint}',profileAdded:'NAS Profile 已新增。請使用 Web Login 或密碼/OTP 完成 DSM 登入。',profilesCount:'{count} 個 Profile',emptyProfilesTitle:'尚未新增 NAS',emptyProfilesDetail:'每台 NAS（包含 Host NAS）都必須建立獨立 Profile。',webSession:'Web Session',storedPassword:'密碼',notSignedIn:'尚未登入',setDefault:'設為預設',test:'測試',webLogin:'Web Login',passwordOTP:'密碼/OTP',delete:'刪除',dsmPasswordPrompt:'此次 Enrollment 使用的 DSM 密碼',otpPrompt:'OTP，沒有則留空',dsmLoginSuccess:'DSM 登入完成。',dsmWebLoginSuccess:'DSM Web Login 完成。',deleteProfileConfirm:'刪除 {name} 及其憑證？',profileDeleted:'NAS Profile 已刪除。',
saveToken:'請立即保存，之後不會再次顯示：\n{token}',tokensCount:'{count} 個 Token',emptyTokensTitle:'尚未建立 MCP Token',emptyTokensDetail:'建立具 Scope 與 NAS Allowlist 的 Token。',revoked:'已撤銷',expiresOn:'到期時間 {date}',active:'有效',rotate:'輪替',revoke:'撤銷',rotateConfirm:'確定輪替？目前的 Token 會立即失效。',saveNewToken:'請立即保存新 Token：\n{token}',revokeConfirm:'確定撤銷此 Token？',tokenRevoked:'MCP Token 已撤銷。',approvalCreated:'高風險 Apply 核准已建立。',approvalsCount:'{count} 筆核准',emptyApprovalsTitle:'目前沒有核准',emptyApprovalsDetail:'高風險 Remote Apply 需要一次性核准。',consumed:'已使用',expired:'已過期',ready:'可使用',exportFailed:'匯出失敗。'
},
'zh-CN':{
brandSubtitle:'NAS 管理网关',authKicker:'Synology NAS MCP Server',authHeadline:'通过 MCP 管理多台 NAS。',authDescription:'管理 NAS 连接、MCP Token、批准和 Audit 记录。',secureSession:'HttpOnly/SameSite 会话',encryptedState:'加密状态',scopedAccess:'MCP 范围控制',language:'语言',
setupTitle:'创建 MCP Server 管理员',setupLead:'此帐号仅管理 MCP Server，不具备 DSM 或 Host NAS 权限。',adminUsername:'管理员帐号',usernamePlaceholder:'3–64 个字符',password:'密码',newPasswordPlaceholder:'至少 12 bytes',confirmPassword:'确认密码',confirmPlaceholder:'再次输入密码',createAdministrator:'创建管理员',setupHelper:'请在启动后一小时内完成设置；超时后请重新启动服务。',
expiredTitle:'设置期限已过',expiredLead:'尚未初始化的 MCP Server 已关闭设置端点。',continueTitle:'下一步',continueDetail:'重新启动未初始化的 container 或套件，以开放新的 1 小时设置期限。',loginTitle:'登录 MCP Server',adminPlaceholder:'MCP Server 管理员',passwordPlaceholder:'输入密码',signIn:'登录',unexpectedInit:'如果不是由你完成初始化，请停止使用并重置部署数据。重置会删除所有 NAS Session。',
serverOnline:'MCP Server 正常',administrator:'管理员',navigationAria:'MCP Server 管理导航',management:'管理',overview:'总览',mcpAccess:'MCP 访问',approvals:'批准',trustBoundary:'独立信任边界',trustBoundaryDetail:'每台 NAS 都必须创建独立 Profile 并登录 DSM。',
overviewTitle:'MCP Server 总览',overviewSubtitle:'服务、NAS 连接和访问权限状态。',addNAS:'添加 NAS',heroTitle:'Synology NAS MCP Server',heroDetail:'通过单一 MCP 端点管理多个 NAS Profile。DSM Session、Token Scope 和批准彼此独立。',mcpEndpoint:'MCP 端点',nasProfilesMetric:'NAS Profile',activeTokens:'有效 MCP Token',readyApprovals:'可用批准',nasConnections:'NAS 连接',nasConnectionsDetail:'添加 NAS Profile 后，再登录 DSM。',noImplicitHost:'Host NAS 不会自动加入',noImplicitHostDetail:'安装本服务的 NAS 也必须使用 LAN IP 或 DNS 名称添加；container localhost 不是 Host NAS。',manageNAS:'管理 NAS',configureMCP:'设置 MCP 访问',quickLinks:'快速链接',quickLinksDetail:'管理功能。',nasProfiles:'NAS Profile',highRiskApprovals:'高风险批准',auditRecords:'Audit 记录',
nasTitle:'NAS 管理',nasSubtitle:'创建 Profile，并为每台 NAS 分别登录 DSM。',refresh:'刷新',addNASDetail:'使用 MCP Server container 可连接的 LAN IP 或 DNS 名称。',profileName:'Profile 名称',profileExample:'例如 office',dsmAccount:'DSM 帐号',tlsVerification:'TLS 验证',fingerprintPlaceholder:'64 位十六进制 fingerprint',fingerprintHint:'使用 pinned fingerprint 时必须明确确认信任。',addProfile:'添加 Profile',nasProfilesDetail:'登录状态和操作只影响所选 NAS。',name:'名称',loginStatus:'登录状态',actions:'操作',
accessTitle:'MCP 访问',accessSubtitle:'创建用于 /mcp 端点的 Token，并限制 NAS 和操作范围。',createToken:'创建 MCP Token',createTokenDetail:'Allowlist 为空时不允许访问任何 NAS。',tokenName:'Token 名称',tokenExample:'例如 monitoring-agent',createTokenButton:'创建 Token',issuedTokens:'已发行 Token',issuedTokensDetail:'Bearer Token 仅在创建或轮替后显示一次。',nameID:'名称 / ID',status:'状态',
approvalTitle:'高风险 Apply 批准',approvalSubtitle:'每项批准绑定 Plan、NAS Profile revision 和 Requesting Token。',createApproval:'创建批准',createApprovalDetail:'输入通过可信 out-of-band 流程获得的字段。',planHashPlaceholder:'64 位 Plan hash',profileRevision:'Profile revision',requestingToken:'Requesting Token ID',createApprovalButton:'创建批准',approvalRecords:'批准记录',approvalRecordsDetail:'包括 ready、expired 和 consumed 状态。',expires:'期限',
auditSubtitle:'查看 MCP Server 管理、Token、批准和远程执行事件。',exportJSONL:'导出 JSONL',recentEvents:'最近事件',recentEventsDetail:'最多显示 100 条；不包含秘密和 Bearer Token。',adminTitle:'MCP Server 管理员',adminSubtitle:'管理本机密码和浏览器 Session。',changePassword:'更改密码',changePasswordDetail:'更改密码会撤销其他管理员 Session。',currentPassword:'当前密码',newPassword:'新密码',currentSession:'当前 Session',currentSessionDetail:'MCP Server 管理员与 DSM 帐号相互独立。',revokeOthers:'登出其他设备',signOut:'登出',
setupDeadline:'设置期限：{date}',initializedAt:'初始化时间：{date}',loginFallback:'请使用本机管理员帐号登录。',sessionExpires:'{username} · Session 到期时间 {date}',passwordMismatch:'两次输入的密码不一致。',passwordUpdated:'密码已更新，其他管理员 Session 已撤销。',otherSessionsRevoked:'其他管理员 Session 已撤销。',trustFingerprint:'确认信任此 SHA-256 certificate fingerprint？\n{fingerprint}',profileAdded:'NAS Profile 已添加。请使用 Web Login 或密码/OTP 完成 DSM 登录。',profilesCount:'{count} 个 Profile',emptyProfilesTitle:'尚未添加 NAS',emptyProfilesDetail:'每台 NAS（包括 Host NAS）都必须创建独立 Profile。',webSession:'Web Session',storedPassword:'密码',notSignedIn:'尚未登录',setDefault:'设为默认',test:'测试',webLogin:'Web Login',passwordOTP:'密码/OTP',delete:'删除',dsmPasswordPrompt:'此次 Enrollment 使用的 DSM 密码',otpPrompt:'OTP，没有则留空',dsmLoginSuccess:'DSM 登录完成。',dsmWebLoginSuccess:'DSM Web Login 完成。',deleteProfileConfirm:'删除 {name} 及其凭据？',profileDeleted:'NAS Profile 已删除。',
saveToken:'请立即保存，之后不会再次显示：\n{token}',tokensCount:'{count} 个 Token',emptyTokensTitle:'尚未创建 MCP Token',emptyTokensDetail:'创建具有 Scope 和 NAS Allowlist 的 Token。',revoked:'已撤销',expiresOn:'到期时间 {date}',active:'有效',rotate:'轮替',revoke:'撤销',rotateConfirm:'确定轮替？当前 Token 会立即失效。',saveNewToken:'请立即保存新 Token：\n{token}',revokeConfirm:'确定撤销此 Token？',tokenRevoked:'MCP Token 已撤销。',approvalCreated:'高风险 Apply 批准已创建。',approvalsCount:'{count} 项批准',emptyApprovalsTitle:'当前没有批准',emptyApprovalsDetail:'高风险 Remote Apply 需要一次性批准。',consumed:'已使用',expired:'已过期',ready:'可使用',exportFailed:'导出失败。'
},
ja:{
brandSubtitle:'NAS 管理ゲートウェイ',authKicker:'Synology NAS 向け MCP Server',authHeadline:'MCP 経由で複数の NAS を管理します。',authDescription:'NAS 接続、MCP Token、承認、Audit 記録を管理します。',secureSession:'HttpOnly/SameSite セッション',encryptedState:'暗号化された状態',scopedAccess:'MCP スコープ制御',language:'言語',
setupTitle:'MCP Server 管理者の作成',setupLead:'このアカウントは MCP Server 専用です。DSM または Host NAS の権限はありません。',adminUsername:'管理者ユーザー名',usernamePlaceholder:'3～64 文字',password:'パスワード',newPasswordPlaceholder:'12 bytes 以上',confirmPassword:'パスワードの確認',confirmPlaceholder:'パスワードを再入力',createAdministrator:'管理者を作成',setupHelper:'起動後 1 時間以内に設定してください。期限切れの場合はサービスを再起動します。',
expiredTitle:'設定期限切れ',expiredLead:'未初期化の MCP Server は設定エンドポイントを閉じました。',continueTitle:'次の操作',continueDetail:'未初期化の container またはパッケージを再起動し、新しい 1 時間の設定期限を開始します。',loginTitle:'MCP Server にサインイン',adminPlaceholder:'MCP Server 管理者',passwordPlaceholder:'パスワードを入力',signIn:'サインイン',unexpectedInit:'自分で初期化していない場合は使用を中止し、デプロイデータをリセットしてください。リセットするとすべての NAS Session が削除されます。',
serverOnline:'MCP Server 正常',administrator:'管理者',navigationAria:'MCP Server 管理ナビゲーション',management:'管理',overview:'概要',mcpAccess:'MCP アクセス',approvals:'承認',trustBoundary:'独立した信頼境界',trustBoundaryDetail:'NAS ごとに個別の Profile と DSM サインインが必要です。',
overviewTitle:'MCP Server 概要',overviewSubtitle:'サービス、NAS 接続、アクセス権限の状態。',addNAS:'NAS を追加',heroTitle:'Synology NAS MCP Server',heroDetail:'1 つの MCP エンドポイントで複数の NAS Profile を管理します。DSM Session、Token Scope、承認は独立しています。',mcpEndpoint:'MCP エンドポイント',nasProfilesMetric:'NAS Profile',activeTokens:'有効な MCP Token',readyApprovals:'利用可能な承認',nasConnections:'NAS 接続',nasConnectionsDetail:'NAS Profile を追加してから DSM にサインインします。',noImplicitHost:'Host NAS は自動追加されません',noImplicitHostDetail:'本サービスを実行する NAS も LAN IP または DNS 名で追加します。container localhost は Host NAS ではありません。',manageNAS:'NAS を管理',configureMCP:'MCP アクセスを設定',quickLinks:'クイックリンク',quickLinksDetail:'管理機能。',nasProfiles:'NAS Profile',highRiskApprovals:'高リスク承認',auditRecords:'Audit 記録',
nasTitle:'NAS 管理',nasSubtitle:'Profile を作成し、NAS ごとに DSM へサインインします。',refresh:'更新',addNASDetail:'MCP Server container から接続可能な LAN IP または DNS 名を使用します。',profileName:'Profile 名',profileExample:'例: office',dsmAccount:'DSM アカウント',tlsVerification:'TLS 検証',fingerprintPlaceholder:'64 文字の 16 進 fingerprint',fingerprintHint:'Pinned fingerprint では明示的な信頼確認が必要です。',addProfile:'Profile を追加',nasProfilesDetail:'サインイン状態と操作は選択した NAS のみに適用されます。',name:'名前',loginStatus:'サインイン状態',actions:'操作',
accessTitle:'MCP アクセス',accessSubtitle:'/mcp エンドポイント用の Token を作成し、NAS と操作スコープを制限します。',createToken:'MCP Token を作成',createTokenDetail:'Allowlist が空の場合、NAS へのアクセスは許可されません。',tokenName:'Token 名',tokenExample:'例: monitoring-agent',createTokenButton:'Token を作成',issuedTokens:'発行済み Token',issuedTokensDetail:'Bearer Token は作成またはローテーション後に 1 回だけ表示されます。',nameID:'名前 / ID',status:'状態',
approvalTitle:'高リスク Apply の承認',approvalSubtitle:'各承認は Plan、NAS Profile revision、Requesting Token に関連付けられます。',createApproval:'承認を作成',createApprovalDetail:'信頼できる out-of-band プロセスで取得した値を入力します。',planHashPlaceholder:'64 文字の Plan hash',profileRevision:'Profile revision',requestingToken:'Requesting Token ID',createApprovalButton:'承認を作成',approvalRecords:'承認記録',approvalRecordsDetail:'ready、expired、consumed の状態を含みます。',expires:'期限',
auditSubtitle:'MCP Server 管理、Token、承認、リモート実行イベントを確認します。',exportJSONL:'JSONL をエクスポート',recentEvents:'最近のイベント',recentEventsDetail:'最大 100 件を表示します。秘密情報と Bearer Token は含まれません。',adminTitle:'MCP Server 管理者',adminSubtitle:'ローカルパスワードとブラウザー Session を管理します。',changePassword:'パスワードを変更',changePasswordDetail:'変更すると他の管理者 Session が取り消されます。',currentPassword:'現在のパスワード',newPassword:'新しいパスワード',currentSession:'現在の Session',currentSessionDetail:'MCP Server 管理者と DSM アカウントは独立しています。',revokeOthers:'他のデバイスをサインアウト',signOut:'サインアウト',
setupDeadline:'設定期限: {date}',initializedAt:'初期化日時: {date}',loginFallback:'ローカル管理者アカウントでサインインしてください。',sessionExpires:'{username} · Session 有効期限 {date}',passwordMismatch:'パスワードが一致しません。',passwordUpdated:'パスワードを更新し、他の管理者 Session を取り消しました。',otherSessionsRevoked:'他の管理者 Session を取り消しました。',trustFingerprint:'この SHA-256 certificate fingerprint を信頼しますか？\n{fingerprint}',profileAdded:'NAS Profile を追加しました。Web Login またはパスワード/OTP で DSM にサインインしてください。',profilesCount:'{count} 件の Profile',emptyProfilesTitle:'NAS が未登録です',emptyProfilesDetail:'Host NAS を含む各 NAS を個別の Profile として追加します。',webSession:'Web Session',storedPassword:'パスワード',notSignedIn:'未サインイン',setDefault:'既定に設定',test:'テスト',webLogin:'Web Login',passwordOTP:'パスワード/OTP',delete:'削除',dsmPasswordPrompt:'今回の Enrollment に使用する DSM パスワード',otpPrompt:'OTP。ない場合は空欄',dsmLoginSuccess:'DSM サインインが完了しました。',dsmWebLoginSuccess:'DSM Web Login が完了しました。',deleteProfileConfirm:'{name} とその資格情報を削除しますか？',profileDeleted:'NAS Profile を削除しました。',
saveToken:'今すぐ保存してください。再表示されません:\n{token}',tokensCount:'{count} 件の Token',emptyTokensTitle:'MCP Token がありません',emptyTokensDetail:'Scope と NAS Allowlist を指定した Token を作成します。',revoked:'取り消し済み',expiresOn:'期限 {date}',active:'有効',rotate:'ローテーション',revoke:'取り消し',rotateConfirm:'ローテーションしますか？現在の Token は直ちに無効になります。',saveNewToken:'新しい Token を今すぐ保存してください:\n{token}',revokeConfirm:'この Token を取り消しますか？',tokenRevoked:'MCP Token を取り消しました。',approvalCreated:'高リスク Apply の承認を作成しました。',approvalsCount:'{count} 件の承認',emptyApprovalsTitle:'承認がありません',emptyApprovalsDetail:'高リスク Remote Apply には 1 回限りの承認が必要です。',consumed:'使用済み',expired:'期限切れ',ready:'利用可能',exportFailed:'エクスポートに失敗しました。'
},
de:{
brandSubtitle:'NAS-Verwaltungs-Gateway',authKicker:'MCP Server für Synology NAS',authHeadline:'Mehrere NAS-Systeme über MCP verwalten.',authDescription:'NAS-Verbindungen, MCP-Token, Freigaben und Audit-Ereignisse verwalten.',secureSession:'HttpOnly/SameSite-Sitzung',encryptedState:'Verschlüsselter Status',scopedAccess:'MCP-Zugriff mit Scopes',language:'Sprache',
setupTitle:'MCP-Server-Administrator erstellen',setupLead:'Dieses Konto verwaltet nur den MCP Server. Es besitzt keine DSM- oder Host-NAS-Rechte.',adminUsername:'Administratorname',usernamePlaceholder:'3–64 Zeichen',password:'Passwort',newPasswordPlaceholder:'Mindestens 12 Bytes',confirmPassword:'Passwort bestätigen',confirmPlaceholder:'Passwort erneut eingeben',createAdministrator:'Administrator erstellen',setupHelper:'Die Einrichtung muss innerhalb einer Stunde nach dem Start erfolgen. Nach Ablauf den Dienst neu starten.',
expiredTitle:'Einrichtungszeitraum abgelaufen',expiredLead:'Der nicht initialisierte MCP Server hat den Einrichtungsendpunkt geschlossen.',continueTitle:'Nächster Schritt',continueDetail:'Den nicht initialisierten Container oder das Paket neu starten. Dadurch beginnt ein neuer Zeitraum von einer Stunde.',loginTitle:'Am MCP Server anmelden',adminPlaceholder:'MCP-Server-Administrator',passwordPlaceholder:'Passwort eingeben',signIn:'Anmelden',unexpectedInit:'Wenn Sie diesen Server nicht initialisiert haben, verwenden Sie ihn nicht und setzen Sie die Bereitstellungsdaten zurück. Dabei werden alle NAS-Sitzungen gelöscht.',
serverOnline:'MCP Server verfügbar',administrator:'Administrator',navigationAria:'MCP-Server-Navigation',management:'Verwaltung',overview:'Übersicht',mcpAccess:'MCP-Zugriff',approvals:'Freigaben',trustBoundary:'Unabhängige Vertrauensgrenze',trustBoundaryDetail:'Jedes NAS benötigt ein eigenes Profil und eine DSM-Anmeldung.',
overviewTitle:'MCP-Server-Übersicht',overviewSubtitle:'Status von Dienst, NAS-Verbindungen und Zugriff.',addNAS:'NAS hinzufügen',heroTitle:'Synology NAS MCP Server',heroDetail:'Mehrere NAS-Profile über einen MCP-Endpunkt verwalten. DSM-Sitzungen, Token-Scopes und Freigaben bleiben getrennt.',mcpEndpoint:'MCP-Endpunkt',nasProfilesMetric:'NAS-Profile',activeTokens:'Aktive MCP-Token',readyApprovals:'Verfügbare Freigaben',nasConnections:'NAS-Verbindungen',nasConnectionsDetail:'NAS-Profil hinzufügen und anschließend bei DSM anmelden.',noImplicitHost:'Host-NAS wird nicht automatisch hinzugefügt',noImplicitHostDetail:'Auch das Host-NAS muss per LAN-IP oder DNS-Name hinzugefügt werden. Container-localhost ist nicht das Host-NAS.',manageNAS:'NAS verwalten',configureMCP:'MCP-Zugriff konfigurieren',quickLinks:'Direktzugriff',quickLinksDetail:'Verwaltungsbereiche.',nasProfiles:'NAS-Profile',highRiskApprovals:'Hochrisiko-Freigaben',auditRecords:'Audit-Ereignisse',
nasTitle:'NAS-Verwaltung',nasSubtitle:'Für jedes NAS ein Profil erstellen und separat bei DSM anmelden.',refresh:'Aktualisieren',addNASDetail:'Eine LAN-IP oder einen DNS-Namen verwenden, der vom MCP-Server-Container erreichbar ist.',profileName:'Profilname',profileExample:'Beispiel: office',dsmAccount:'DSM-Konto',tlsVerification:'TLS-Prüfung',fingerprintPlaceholder:'64-stelliger hexadezimaler Fingerprint',fingerprintHint:'Pinned Fingerprints erfordern eine ausdrückliche Vertrauensbestätigung.',addProfile:'Profil hinzufügen',nasProfilesDetail:'Anmeldestatus und Aktionen gelten nur für das ausgewählte NAS.',name:'Name',loginStatus:'Anmeldestatus',actions:'Aktionen',
accessTitle:'MCP-Zugriff',accessSubtitle:'Token für den Endpunkt /mcp erstellen und NAS- sowie Aktions-Scopes begrenzen.',createToken:'MCP-Token erstellen',createTokenDetail:'Eine leere Allowlist gewährt keinen NAS-Zugriff.',tokenName:'Token-Name',tokenExample:'Beispiel: monitoring-agent',createTokenButton:'Token erstellen',issuedTokens:'Ausgegebene Token',issuedTokensDetail:'Das Bearer-Token wird nach Erstellung oder Rotation einmal angezeigt.',nameID:'Name / ID',status:'Status',
approvalTitle:'Freigaben für risikoreiche Apply-Vorgänge',approvalSubtitle:'Jede Freigabe ist an Plan, NAS-Profilrevision und anforderndes Token gebunden.',createApproval:'Freigabe erstellen',createApprovalDetail:'Werte aus einem vertrauenswürdigen Out-of-Band-Prozess eingeben.',planHashPlaceholder:'64-stelliger Plan-Hash',profileRevision:'Profilrevision',requestingToken:'ID des anfordernden Tokens',createApprovalButton:'Freigabe erstellen',approvalRecords:'Freigaben',approvalRecordsDetail:'Enthält verfügbare, abgelaufene und verwendete Freigaben.',expires:'Gültig bis',
auditSubtitle:'Ereignisse für MCP-Server-Verwaltung, Token, Freigaben und Remote-Ausführung prüfen.',exportJSONL:'JSONL exportieren',recentEvents:'Letzte Ereignisse',recentEventsDetail:'Zeigt bis zu 100 Ereignisse. Geheimnisse und Bearer-Token sind ausgeschlossen.',adminTitle:'MCP-Server-Administrator',adminSubtitle:'Lokales Passwort und Browser-Sitzungen verwalten.',changePassword:'Passwort ändern',changePasswordDetail:'Die Änderung widerruft andere Administrator-Sitzungen.',currentPassword:'Aktuelles Passwort',newPassword:'Neues Passwort',currentSession:'Aktuelle Sitzung',currentSessionDetail:'MCP-Server-Administrator und DSM-Konten sind unabhängig.',revokeOthers:'Andere Geräte abmelden',signOut:'Abmelden',
setupDeadline:'Einrichtung bis: {date}',initializedAt:'Initialisiert: {date}',loginFallback:'Mit dem lokalen Administratorkonto anmelden.',sessionExpires:'{username} · Sitzung gültig bis {date}',passwordMismatch:'Die Passwörter stimmen nicht überein.',passwordUpdated:'Passwort aktualisiert. Andere Administrator-Sitzungen wurden widerrufen.',otherSessionsRevoked:'Andere Administrator-Sitzungen wurden widerrufen.',trustFingerprint:'Diesem SHA-256-Zertifikat-Fingerprint vertrauen?\n{fingerprint}',profileAdded:'NAS-Profil hinzugefügt. DSM-Anmeldung per Web Login oder Passwort/OTP abschließen.',profilesCount:'{count} Profile',emptyProfilesTitle:'Keine NAS-Profile',emptyProfilesDetail:'Jedes NAS einschließlich Host-NAS als separates Profil hinzufügen.',webSession:'Web-Sitzung',storedPassword:'Passwort',notSignedIn:'Nicht angemeldet',setDefault:'Als Standard',test:'Testen',webLogin:'Web Login',passwordOTP:'Passwort/OTP',delete:'Löschen',dsmPasswordPrompt:'DSM-Passwort für diese Einrichtung',otpPrompt:'OTP oder leer lassen',dsmLoginSuccess:'DSM-Anmeldung abgeschlossen.',dsmWebLoginSuccess:'DSM Web Login abgeschlossen.',deleteProfileConfirm:'{name} und die zugehörigen Zugangsdaten löschen?',profileDeleted:'NAS-Profil gelöscht.',
saveToken:'Jetzt speichern. Das Token wird nicht erneut angezeigt:\n{token}',tokensCount:'{count} Token',emptyTokensTitle:'Keine MCP-Token',emptyTokensDetail:'Ein Token mit Scopes und NAS-Allowlist erstellen.',revoked:'Widerrufen',expiresOn:'Gültig bis {date}',active:'Aktiv',rotate:'Rotieren',revoke:'Widerrufen',rotateConfirm:'Token rotieren? Das aktuelle Token wird sofort ungültig.',saveNewToken:'Neues Token jetzt speichern:\n{token}',revokeConfirm:'Dieses Token widerrufen?',tokenRevoked:'MCP-Token widerrufen.',approvalCreated:'Freigabe für risikoreichen Apply-Vorgang erstellt.',approvalsCount:'{count} Freigaben',emptyApprovalsTitle:'Keine Freigaben',emptyApprovalsDetail:'Risikoreiche Remote-Apply-Vorgänge benötigen eine einmalige Freigabe.',consumed:'Verwendet',expired:'Abgelaufen',ready:'Verfügbar',exportFailed:'Export fehlgeschlagen.'
}}
let locale=resolveInitialLocale(),currentSetupStatus=null,currentSession=null,issuedToken='',issuedKind='';
function normalizeLocale(value){let input=String(value||'').toLowerCase();if(input.startsWith('zh-hant')||input.startsWith('zh-tw')||input.startsWith('zh-hk'))return 'zh-TW';if(input.startsWith('zh'))return 'zh-CN';if(input.startsWith('ja'))return 'ja';if(input.startsWith('de'))return 'de';return 'en'}
function resolveInitialLocale(){try{let saved=localStorage.getItem('dsmctl.locale');if(saved&&supportedLocales.includes(saved))return saved}catch{}let preferred=(navigator.languages&&navigator.languages[0])||navigator.language||'en';return normalizeLocale(preferred)}
function t(key,values={}){let value=(translations[locale]&&translations[locale][key])||translations.en[key]||key;return value.replace(/\{([^}]+)\}/g,(_,name)=>String(values[name]??''))}
function applyLocale(){document.documentElement.lang={'zh-TW':'zh-Hant','zh-CN':'zh-Hans',ja:'ja',de:'de',en:'en'}[locale];for(const node of document.querySelectorAll('[data-i18n]'))node.textContent=t(node.dataset.i18n);for(const node of document.querySelectorAll('[data-i18n-placeholder]'))node.placeholder=t(node.dataset.i18nPlaceholder);for(const node of document.querySelectorAll('[data-i18n-aria]'))node.setAttribute('aria-label',t(node.dataset.i18nAria));for(const select of document.querySelectorAll('[data-locale-select]'))select.value=locale;document.title='dsmctl MCP Server';renderRuntimeText()}
function renderRuntimeText(){if(currentSetupStatus){if(currentSetupStatus.state==='setup_available')$('setupExpiry').textContent=t('setupDeadline',{date:formatDate(currentSetupStatus.setup_expires_at)});else if(currentSetupStatus.state!=='setup_expired')$('initializedAt').textContent=currentSetupStatus.initialized_at?t('initializedAt',{date:formatDate(currentSetupStatus.initialized_at)}):t('loginFallback')}if(currentSession){$('sessionInfo').textContent=t('sessionExpires',{username:currentSession.username,date:formatDate(currentSession.expires_at)});$('topUser').lastElementChild.textContent=currentSession.username;$('topUser').querySelector('.user-avatar').textContent=currentSession.username.slice(0,1).toUpperCase()}renderIssuedToken()}
function renderIssuedToken(){if(!issuedToken)return;$('issued').hidden=false;$('issued').textContent=t(issuedKind,{token:issuedToken})}
async function changeLocale(value){locale=supportedLocales.includes(value)?value:'en';try{localStorage.setItem('dsmctl.locale',locale)}catch{}applyLocale();show('');if(!$('application').hidden)await Promise.allSettled([loadProfiles(),loadTokens(),loadApprovals()])}
function translationDiagnostics(){let expected=Object.keys(translations.en),result={};for(const name of supportedLocales)result[name]={missing:expected.filter(key=>!(key in translations[name])),extra:Object.keys(translations[name]).filter(key=>!expected.includes(key))};return result}
window.__dsmctlI18nDiagnostics=translationDiagnostics;
document.documentElement.dataset.i18nDiagnostics=JSON.stringify(translationDiagnostics());
function show(value){let node=$('message'),text=value instanceof Error?value.message:String(value||'');node.textContent=text;node.hidden=!text;node.className='toast'+(value instanceof Error?' error':'')}
async function api(path,options={}){let method=(options.method||'GET').toUpperCase(),headers=Object.assign({},options.headers||{});if(method!=='GET'&&method!=='HEAD'){headers['Content-Type']='application/json';headers['X-DSMCTL-Request']='1'}let response=await fetch(apiBase+path,Object.assign({credentials:'same-origin'},options,{headers})),text=await response.text(),value={};if(text){try{value=JSON.parse(text)}catch{value={error:text}}}if(!response.ok){let error=new Error(value.error||response.statusText);error.status=response.status;throw error}return value}
function hideEntry(){for(const id of ['setupCard','expiredCard','loginCard','entryShell','application'])$(id).hidden=true}
function showEntry(id){hideEntry();$('entryShell').hidden=false;$(id).hidden=false}
function setView(name){for(const view of document.querySelectorAll('.view'))view.hidden=view.dataset.view!==name;for(const item of document.querySelectorAll('[data-nav]'))item.classList.toggle('active',item.dataset.nav===name);document.documentElement.scrollTop=0}
function formatDate(value){return value?new Date(value).toLocaleString(locale):'—'}
function setMetric(id,value){$(id).textContent=String(value)}
function emptyRow(body,columns,title,detail){let row=body.insertRow();row.className='empty-row';let td=row.insertCell();td.colSpan=columns;let box=document.createElement('div');box.className='empty-state';let icon=document.createElement('span');icon.className='empty-icon';icon.textContent='◇';let strong=document.createElement('strong');strong.textContent=title;let span=document.createElement('span');span.textContent=detail;box.append(icon,strong,span);td.appendChild(box)}
function badge(text,kind=''){let item=document.createElement('span');item.className='badge'+(kind?' '+kind:'');item.textContent=text;return item}
function cell(row,text){let td=row.insertCell();td.textContent=text;return td}
function button(parent,label,fn,danger=false){let item=document.createElement('button');item.type='button';item.textContent=label;item.onclick=fn;item.className='table-action'+(danger?' danger':'');parent.classList.add('table-actions');parent.appendChild(item)}
function toggleFingerprint(){$('fingerprintField').hidden=$('tls').value!=='pinned_fingerprint'}

async function initialize(){hideEntry();try{currentSetupStatus=await api('/setup/status');if(currentSetupStatus.state==='setup_available'){showEntry('setupCard');renderRuntimeText();return}if(currentSetupStatus.state==='setup_expired'){showEntry('expiredCard');return}renderRuntimeText();try{await showApplication()}catch(error){if(error.status===401)showEntry('loginCard');else throw error}}catch(error){show(error)}}
async function setupAdministrator(){if($('setupPassword').value!==$('setupConfirm').value){show(new Error(t('passwordMismatch')));return}try{await api('/setup',{method:'POST',body:JSON.stringify({username:$('setupUsername').value,password:$('setupPassword').value})});$('setupPassword').value=$('setupConfirm').value='';await showApplication()}catch(error){show(error)}}
async function loginAdministrator(){try{await api('/login',{method:'POST',body:JSON.stringify({username:$('loginUsername').value,password:$('loginPassword').value})});$('loginPassword').value='';await showApplication()}catch(error){$('loginPassword').value='';show(error)}}
async function showApplication(){let value=await api('/session');currentSession=value.session;hideEntry();$('application').hidden=false;renderRuntimeText();setView('overview');show('');await Promise.allSettled([loadProfiles(),loadTokens(),loadApprovals(),loadAudit()])}
async function logoutAdministrator(){try{await api('/logout',{method:'POST',body:'{}'});currentSession=null;await initialize()}catch(error){show(error)}}
async function changePassword(){try{await api('/password',{method:'PUT',body:JSON.stringify({current_password:$('currentPassword').value,new_password:$('newPassword').value})});$('currentPassword').value=$('newPassword').value='';show(t('passwordUpdated'))}catch(error){$('currentPassword').value=$('newPassword').value='';show(error)}}
async function revokeOthers(){try{await api('/sessions/revoke-others',{method:'POST',body:'{}'});show(t('otherSessionsRevoked'))}catch(error){show(error)}}

async function addProfile(){try{let pinned=$('tls').value==='pinned_fingerprint',fingerprint=$('fingerprint').value.trim(),confirmed=!pinned||confirm(t('trustFingerprint',{fingerprint}));if(!confirmed)return;await api('/profiles',{method:'POST',body:JSON.stringify({name:$('name').value,url:$('url').value,username:$('username').value,tls_mode:$('tls').value,certificate_fingerprint:fingerprint,confirm_certificate_fingerprint:confirmed})});$('name').value=$('url').value=$('username').value=$('fingerprint').value='';$('tls').value='system_ca';toggleFingerprint();show(t('profileAdded'));await loadProfiles()}catch(error){show(error)}}
async function loadProfiles(){try{let value=await api('/profiles'),profiles=value.profiles||[],body=$('profiles');body.textContent='';$('profileCount').textContent=t('profilesCount',{count:profiles.length});setMetric('metricNAS',profiles.length);if(!profiles.length){emptyRow(body,5,t('emptyProfilesTitle'),t('emptyProfilesDetail'));return profiles}for(const profile of profiles){let row=body.insertRow();cell(row,(profile.default?'★ ':'')+profile.name);cell(row,profile.url);cell(row,String(profile.revision));let status=cell(row,'');status.appendChild(profile.session_stored?badge(t('webSession'),'success'):profile.password_stored?badge(t('storedPassword'),'success'):badge(t('notSignedIn'),'warning'));let actions=cell(row,'');button(actions,t('setDefault'),()=>setDefault(profile.name));button(actions,t('test'),()=>testNAS(profile.name));button(actions,t('webLogin'),()=>webLogin(profile.name));button(actions,t('passwordOTP'),()=>passwordLogin(profile.name));button(actions,t('delete'),()=>removeNAS(profile),true)}return profiles}catch(error){show(error);return[]}}
async function setDefault(name){try{await api('/profiles/'+encodeURIComponent(name)+'/default',{method:'POST',body:'{}'});await loadProfiles()}catch(error){show(error)}}
async function testNAS(name){try{show(JSON.stringify(await api('/profiles/'+encodeURIComponent(name)+'/test',{method:'POST',body:'{}'}),null,2))}catch(error){show(error)}}
async function passwordLogin(name){let password=prompt(t('dsmPasswordPrompt'));if(password===null)return;let otp=prompt(t('otpPrompt'))||'';try{await api('/profiles/'+encodeURIComponent(name)+'/credentials/password',{method:'POST',body:JSON.stringify({password,otp})});password=otp='';show(t('dsmLoginSuccess'));await loadProfiles()}catch(error){password=otp='';show(error)}}
async function webLogin(name){try{let start=await api('/profiles/'+encodeURIComponent(name)+'/weblogin/start',{method:'POST',body:'{}'}),popup=window.open(start.login_url,'dsmctl_signin','width=560,height=720'),listener=async event=>{if(event.origin!==start.nas_origin)return;let data=event.data||{};if(!data.code)return;window.removeEventListener('message',listener);try{await api('/profiles/'+encodeURIComponent(name)+'/weblogin/complete',{method:'POST',body:JSON.stringify({enrollment_id:start.enrollment_id,code:data.code,rs:data.rs,state:data.state||start.state})});if(popup)popup.close();show(t('dsmWebLoginSuccess'));await loadProfiles()}catch(error){show(error)}};window.addEventListener('message',listener)}catch(error){show(error)}}
async function removeNAS(profile){if(!confirm(t('deleteProfileConfirm',{name:profile.name})))return;try{await api('/profiles/'+encodeURIComponent(profile.name)+'?revision='+profile.revision,{method:'DELETE',body:'{}'});show(t('profileDeleted'));await loadProfiles()}catch(error){show(error)}}

async function createMCPToken(){try{let scopes=[...document.querySelectorAll('.scope:checked')].map(item=>item.value),nas=$('tokenNAS').value.split(',').map(item=>item.trim()).filter(Boolean),value=await api('/mcp-tokens',{method:'POST',body:JSON.stringify({name:$('tokenName').value,scopes,nas_allowlist:nas})});issuedToken=value.bearer_token;issuedKind='saveToken';renderIssuedToken();await loadTokens()}catch(error){show(error)}}
async function loadTokens(){try{let value=await api('/mcp-tokens'),tokens=value.tokens||[],body=$('tokens');body.textContent='';$('tokenCount').textContent=t('tokensCount',{count:tokens.length});setMetric('metricTokens',tokens.filter(item=>!item.revoked_at&&(!item.expires_at||new Date(item.expires_at)>new Date())).length);if(!tokens.length){emptyRow(body,5,t('emptyTokensTitle'),t('emptyTokensDetail'));return tokens}for(const token of tokens){let row=body.insertRow();cell(row,token.name+'\n'+token.id);cell(row,token.scopes.join(', '));cell(row,token.nas_allowlist.join(', ')||'—');let status=cell(row,'');status.appendChild(token.revoked_at?badge(t('revoked')):token.expires_at?badge(t('expiresOn',{date:formatDate(token.expires_at)}),'warning'):badge(t('active'),'success'));let actions=cell(row,'');button(actions,t('rotate'),()=>rotateToken(token.id));button(actions,t('revoke'),()=>revokeToken(token.id),true)}return tokens}catch(error){show(error);return[]}}
async function rotateToken(id){if(!confirm(t('rotateConfirm')))return;try{let value=await api('/mcp-tokens/'+id+'/rotate',{method:'POST',body:'{}'});issuedToken=value.bearer_token;issuedKind='saveNewToken';renderIssuedToken();await loadTokens()}catch(error){show(error)}}
async function revokeToken(id){if(!confirm(t('revokeConfirm')))return;try{await api('/mcp-tokens/'+id,{method:'DELETE',body:'{}'});show(t('tokenRevoked'));await loadTokens()}catch(error){show(error)}}

async function createApproval(){try{await api('/approvals',{method:'POST',body:JSON.stringify({plan_hash:$('approvalHash').value,nas:$('approvalNAS').value,profile_revision:Number($('approvalRevision').value),requesting_token_id:$('approvalToken').value})});show(t('approvalCreated'));await loadApprovals()}catch(error){show(error)}}
async function loadApprovals(){try{let value=await api('/approvals?include_consumed=true'),approvals=value.approvals||[],body=$('approvals');body.textContent='';$('approvalCount').textContent=t('approvalsCount',{count:approvals.length});setMetric('metricApprovals',approvals.filter(item=>!item.consumed_at&&new Date(item.expires_at)>new Date()).length);if(!approvals.length){emptyRow(body,4,t('emptyApprovalsTitle'),t('emptyApprovalsDetail'));return approvals}for(const approval of approvals){let row=body.insertRow();cell(row,approval.plan_hash+'\n'+approval.nas+' @ '+approval.profile_revision);cell(row,approval.requesting_token_id);cell(row,formatDate(approval.expires_at));let state=approval.consumed_at?'consumed':new Date(approval.expires_at)<new Date()?'expired':'ready',status=cell(row,'');status.appendChild(badge(t(state),state==='ready'?'success':state==='expired'?'warning':''))}return approvals}catch(error){show(error);return[]}}
async function loadAudit(){try{let events=(await api('/audit?limit=100')).events||[];$('audit').textContent=JSON.stringify(events,null,2);return events}catch(error){show(error);return[]}}
async function exportAudit(){try{let response=await fetch(apiBase+'/audit/export?limit=1000',{credentials:'same-origin'});if(!response.ok)throw new Error(t('exportFailed'));let target=URL.createObjectURL(await response.blob()),link=document.createElement('a');link.href=target;link.download='dsmctl-audit.jsonl';link.click();URL.revokeObjectURL(target)}catch(error){show(error)}}
applyLocale();initialize();
</script>
</body>
</html>`
