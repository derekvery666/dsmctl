package weblogin

import (
	"net/url"

	"github.com/ychiu1211/dsmctl/internal/webassets"
)

// buildPage renders the loopback helper page that hosts the DSM sign-in
// popup and relays the one-time code to the local callback.
//
// The visual identity (brand/slate token scales, semantic aliases,
// typography, card language) is copied verbatim from the gateway
// administration UI; internal/gateway/admin/ui.go is the source of truth.
// TestPageCarriesSharedDesignTokens pins the shared literals here and
// internal/gateway/admin/handler_test.go pins them there, so drift fails a
// build. Copy is localized (en, zh-TW, zh-CN, ja, de) from
// navigator.language; the terminal state (success or error) is chosen by
// the /callback HTTP status, never by injecting server response text.
func buildPage(loginURL, dsmOrigin string) string {
	// loginURL and dsmOrigin are produced internally from a validated base URL,
	// so they are safe to embed in the JS string literals below. The heading
	// names the bare host; the target line keeps the full origin so scheme and
	// port stay checkable.
	host := dsmOrigin
	if parsed, err := url.Parse(dsmOrigin); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="` + webassets.ThemeColor + `">
<link rel="icon" href="/favicon.svg" type="image/svg+xml" sizes="any">
<title>dsmctl sign-in</title>
<style>
:root{
  color-scheme:light;
  --brand-50:#eef7ff;
  --brand-100:#d9ecff;
  --brand-200:#b9dcff;
  --brand-300:#88c3ff;
  --brand-400:#4da5f4;
  --brand-500:#2588df;
  --brand-600:#146fbd;
  --brand-700:#155b97;
  --brand-800:#174b78;
  --brand-900:#173d61;
  --brand-950:#0d263f;
  --slate-25:#fbfcfe;
  --slate-50:#f7f9fc;
  --slate-100:#eef2f6;
  --slate-200:#dde5ed;
  --slate-300:#c6d1dc;
  --slate-400:#93a3b5;
  --slate-500:#66778b;
  --slate-600:#485a70;
  --slate-700:#34465b;
  --slate-800:#223246;
  --slate-900:#162334;
  --slate-950:#0f1927;
  --white:#ffffff;
  --color-action:var(--brand-500);
  --color-action-hover:var(--brand-600);
  --color-action-soft:var(--brand-50);
  --color-action-text:var(--white);
  --color-focus:rgba(37,136,223,.28);
  --color-focus-soft:rgba(37,136,223,.12);
  --canvas:var(--slate-100);
  --surface:var(--white);
  --surface-soft:var(--slate-50);
  --text:var(--slate-900);
  --muted:var(--slate-500);
  --line:var(--slate-200);
  --line-strong:var(--slate-300);
  --success:#1f9d68;
  --success-vivid:#35b47b;
  --success-soft:#eaf8f2;
  --success-text:#267b59;
  --danger:#cf3f3f;
  --danger-soft:#fff1f1;
  --danger-text:#a43131;
  --shadow-large:0 28px 80px rgba(25,54,86,.18);
  font-family:Inter,"Segoe UI","Noto Sans TC",system-ui,-apple-system,sans-serif;
}
*{box-sizing:border-box}
html,body{margin:0;min-height:100%}
body{display:flex;align-items:center;justify-content:center;min-height:100vh;padding:24px;background:radial-gradient(circle at 86% 12%,rgba(77,165,244,.16),transparent 28%),linear-gradient(145deg,var(--brand-50) 0,var(--slate-50) 52%,var(--slate-100) 100%);color:var(--text);font-size:14px;line-height:1.55}
button{font:inherit;cursor:pointer}
:focus-visible{outline:3px solid var(--color-focus);outline-offset:2px}
.card{width:min(430px,100%);padding:34px;border:1px solid rgba(198,209,220,.92);border-radius:18px;background:rgba(255,255,255,.92);box-shadow:var(--shadow-large);backdrop-filter:blur(14px)}
.brand{display:flex;align-items:center;gap:11px;margin-bottom:26px}
.brand-mark{display:grid;grid-template-columns:repeat(2,9px);grid-template-rows:repeat(2,9px);gap:3px;padding:8px;border-radius:10px;background:linear-gradient(145deg,var(--brand-400),var(--brand-600));box-shadow:0 7px 16px rgba(20,111,189,.25)}
.brand-mark i{display:block;border-radius:2px;background:rgba(255,255,255,.94)}
.brand-copy strong{display:block;font-size:15px;line-height:1.15;letter-spacing:.01em}
.brand-copy span{display:block;margin-top:2px;color:var(--muted);font-size:11px}
h1{margin:0 0 6px;font-size:25px;letter-spacing:-.02em}
.target{margin:0 0 20px;color:var(--slate-600);font-size:13px;word-break:break-all}
.status{display:flex;gap:11px;align-items:flex-start;margin:0 0 22px;padding:12px 14px;border-radius:10px;background:var(--color-action-soft);color:var(--brand-700);font-size:13px}
.dot{flex:none;width:9px;height:9px;margin-top:5px;border-radius:50%;background:var(--color-action)}
[data-state="waiting"] .dot{animation:pulse 1.6s ease-in-out infinite}
[data-state="exchanging"] .dot{animation:pulse .8s ease-in-out infinite}
[data-state="success"] .status{background:var(--success-soft);color:var(--success-text)}
[data-state="success"] .dot{background:var(--success)}
[data-state="error"] .status{background:var(--danger-soft);color:var(--danger-text)}
[data-state="error"] .dot{background:var(--danger)}
@keyframes pulse{0%,100%{box-shadow:0 0 0 0 var(--color-focus-soft)}50%{box-shadow:0 0 0 7px var(--color-focus-soft)}}
.msg{display:none;margin:0}
[data-state="waiting"] .msg-waiting{display:block}
[data-state="exchanging"] .msg-exchanging{display:block}
[data-state="success"] .msg-success{display:block}
[data-state="error"] .msg-error{display:block}
.primary{display:none;width:100%;min-height:40px;align-items:center;justify-content:center;gap:7px;padding:8px 15px;border:1px solid transparent;border-radius:7px;background:var(--color-action);color:var(--color-action-text);font-weight:600;box-shadow:0 2px 4px rgba(20,111,189,.16);transition:background .15s,transform .15s,box-shadow .15s}
.primary:hover{background:var(--color-action-hover);box-shadow:0 5px 12px rgba(20,111,189,.2)}
.primary:active{transform:translateY(1px)}
[data-state="waiting"] .primary{display:inline-flex}
.foot{margin:20px 0 0;padding:0;list-style:none;display:flex;flex-direction:column;gap:6px;color:var(--muted);font-size:12px}
.foot li{display:flex;align-items:center;gap:6px}
.foot li:before{content:"\2713";color:var(--success-vivid)}
.field{display:flex;flex-direction:column;gap:6px;margin-bottom:13px}
.field label{color:var(--slate-600);font-size:12px;font-weight:600}
.control{width:100%;min-height:40px;padding:9px 11px;border:1px solid var(--line-strong);border-radius:7px;background:var(--surface);color:var(--text);font:inherit}
.control:focus{border-color:var(--color-action);box-shadow:0 0 0 3px var(--color-focus-soft);outline:none}
.submit{width:100%;min-height:40px;border:1px solid transparent;border-radius:7px;background:var(--color-action);color:var(--color-action-text);font-weight:600;margin-top:2px}
.submit:hover{background:var(--color-action-hover)}
.submit:disabled{opacity:.55;cursor:not-allowed}
.divider{display:flex;align-items:center;gap:10px;margin:18px 0;color:var(--muted);font-size:12px}
.divider:before,.divider:after{content:"";height:1px;flex:1;background:var(--line)}
.weblogin{width:100%;min-height:40px;border:1px solid var(--line-strong);border-radius:7px;background:var(--surface);color:var(--slate-700);font-weight:600;display:inline-flex;align-items:center;justify-content:center;gap:8px}
.weblogin:hover{border-color:var(--slate-400);background:var(--surface-soft)}
.err{display:none;margin:0 0 14px;padding:10px 12px;border-radius:8px;background:var(--danger-soft);color:var(--danger-text);font-size:12px}
body[data-state="choose"] #statusbox{display:none}
body:not([data-state="choose"]) #choices{display:none}
</style>
</head>
<body data-state="choose">
<main class="card">
  <div class="brand"><span class="brand-mark" aria-hidden="true"><i></i><i></i><i></i><i></i></span><span class="brand-copy"><strong>dsmctl</strong><span data-i18n="brandSub">DSM sign-in</span></span></div>
  <h1 data-i18n="heading">Sign in to ` + host + `</h1>
  <p class="target">` + dsmOrigin + `</p>
  <div id="choices">
    <p class="err" id="err"></p>
    <form id="pwform" autocomplete="on">
      <div class="field"><label for="acc" data-i18n="account">DSM account</label><input class="control" id="acc" autocomplete="username" required></div>
      <div class="field"><label for="pw" data-i18n="password">Password</label><input class="control" id="pw" type="password" autocomplete="current-password" required></div>
      <div class="field"><label for="otp" data-i18n="otp">OTP (only if enabled)</label><input class="control" id="otp" inputmode="numeric" autocomplete="one-time-code"></div>
      <button class="submit" type="submit" data-i18n="signin">Sign in</button>
    </form>
    <div class="divider" data-i18n="or">or</div>
    <button class="weblogin" id="go" type="button" data-i18n="weblogin">Sign in with Web Login</button>
  </div>
  <div class="status" role="status" id="statusbox">
    <span class="dot" aria-hidden="true"></span>
    <span>
      <p class="msg msg-exchanging" data-i18n="exchanging">Completing sign-in…</p>
      <p class="msg msg-success" data-i18n="success">Signed in. You can close this window and return to the terminal.</p>
      <p class="msg msg-error" data-i18n="failure">Sign-in failed. Return to the terminal for details.</p>
    </span>
  </div>
  <ul class="foot">
    <li data-i18n="footWeb">Web Login enters the password only on the NAS's own page</li>
    <li data-i18n="footPw">Account + password is sent over the local loopback to dsmctl and stored encrypted</li>
  </ul>
</main>
<script>
var loginUrl = "` + loginURL + `";
var dsmOrigin = "` + dsmOrigin + `";
var host = "` + host + `";
var strings = {
en:{brandSub:"DSM sign-in",heading:"Sign in to {host}",account:"DSM account",password:"Password",otp:"OTP (only if enabled)",signin:"Sign in",or:"or",weblogin:"Sign in with Web Login",exchanging:"Completing sign-in…",success:"Signed in. You can close this window and return to the terminal.",failure:"Sign-in failed. Check the account and password, or return to the terminal for details.",footWeb:"Web Login enters the password only on the NAS's own page",footPw:"Account + password is sent over the local loopback to dsmctl and stored encrypted"},
"zh-TW":{brandSub:"DSM 登入",heading:"登入 {host}",account:"DSM 帳號",password:"密碼",otp:"OTP(有啟用才需要)",signin:"登入",or:"或",weblogin:"用 Web Login 登入",exchanging:"正在完成登入…",success:"已登入。可以關閉此視窗，回到終端機。",failure:"登入失敗。請檢查帳號與密碼，或回到終端機查看詳細資訊。",footWeb:"Web Login 只在 NAS 自己的頁面輸入密碼",footPw:"帳號密碼經本機 loopback 傳給 dsmctl,並加密儲存"},
"zh-CN":{brandSub:"DSM 登录",heading:"登录 {host}",account:"DSM 账号",password:"密码",otp:"OTP(仅启用时需要)",signin:"登录",or:"或",weblogin:"用 Web Login 登录",exchanging:"正在完成登录…",success:"已登录。可以关闭此窗口，回到终端。",failure:"登录失败。请检查账号与密码，或回到终端查看详细信息。",footWeb:"Web Login 只在 NAS 自己的页面输入密码",footPw:"账号密码经本机 loopback 传给 dsmctl，并加密存储"},
ja:{brandSub:"DSM サインイン",heading:"{host} にサインイン",account:"DSM アカウント",password:"パスワード",otp:"OTP（有効な場合のみ）",signin:"サインイン",or:"または",weblogin:"Web Login でサインイン",exchanging:"サインインを完了しています…",success:"サインインしました。このウィンドウを閉じてターミナルに戻れます。",failure:"サインインに失敗しました。アカウントとパスワードを確認するか、ターミナルをご確認ください。",footWeb:"Web Login はパスワードを NAS 自身のページでのみ入力します",footPw:"アカウントとパスワードはローカルの loopback で dsmctl に送られ、暗号化保存されます"},
de:{brandSub:"DSM-Anmeldung",heading:"Bei {host} anmelden",account:"DSM-Konto",password:"Passwort",otp:"OTP (nur wenn aktiviert)",signin:"Anmelden",or:"oder",weblogin:"Mit Web Login anmelden",exchanging:"Anmeldung wird abgeschlossen …",success:"Angemeldet. Sie können dieses Fenster schließen und zum Terminal zurückkehren.",failure:"Anmeldung fehlgeschlagen. Prüfen Sie Konto und Passwort oder das Terminal.",footWeb:"Web Login gibt das Passwort nur auf der Seite des NAS ein",footPw:"Konto und Passwort werden über den lokalen Loopback an dsmctl gesendet und verschlüsselt gespeichert"}
};
function normalizeLocale(value){var input=String(value||"").toLowerCase();if(input.indexOf("zh-hant")===0||input.indexOf("zh-tw")===0||input.indexOf("zh-hk")===0)return "zh-TW";if(input.indexOf("zh")===0)return "zh-CN";if(input.indexOf("ja")===0)return "ja";if(input.indexOf("de")===0)return "de";return "en"}
var locale = normalizeLocale(navigator.language);
document.documentElement.lang = {"zh-TW":"zh-Hant","zh-CN":"zh-Hans",ja:"ja",de:"de",en:"en"}[locale];
var table = strings[locale];
for (var nodes = document.querySelectorAll("[data-i18n]"), i = 0; i < nodes.length; i++) {
  nodes[i].textContent = table[nodes[i].getAttribute("data-i18n")].replace("{host}", host);
}
function setState(s){ document.body.setAttribute("data-state", s); }
function showErr(msg){ var e = document.getElementById("err"); e.textContent = msg; e.style.display = "block"; }
function start(){ window.open(loginUrl, "dsmctl_signin", "width=560,height=720"); }
document.getElementById("go").onclick = start;
document.getElementById("pwform").onsubmit = function(ev){
  ev.preventDefault();
  document.getElementById("err").style.display = "none";
  setState("exchanging");
  fetch("/password", {method:"POST", headers:{"Content-Type":"application/json"},
    body: JSON.stringify({account:document.getElementById("acc").value, password:document.getElementById("pw").value, otp:document.getElementById("otp").value})})
    .then(function(r){ if (r.ok) { setState("success"); } else { setState("choose"); showErr(table.failure); } })
    .catch(function(){ setState("choose"); showErr(table.failure); });
};
window.addEventListener("message", function(e){
  if (e.origin !== dsmOrigin) return;
  var d = e.data || {};
  if (!d.code) return;
  setState("exchanging");
  fetch("/callback", {method:"POST", headers:{"Content-Type":"application/json"},
    body: JSON.stringify({code:d.code, rs:d.rs, state:d.state || ""})})
    .then(function(r){ setState(r.ok?"success":"error"); })
    .catch(function(){ setState("error"); });
});
</script>
</body></html>`
}
