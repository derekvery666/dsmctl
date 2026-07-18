package admin

const indexHTML = `<!doctype html>
<html lang="zh-Hant"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>dsmctl Gateway</title><style>
body{font:15px system-ui,sans-serif;max-width:980px;margin:2rem auto;padding:0 1rem;color:#172033;background:#f6f8fb}
h1{margin-bottom:.25rem}.card{background:white;border:1px solid #dce2ea;border-radius:10px;padding:1rem;margin:1rem 0}
input,select,button{font:inherit;padding:.5rem;margin:.2rem;border:1px solid #aeb8c5;border-radius:6px}button{cursor:pointer;background:#1d6fe8;color:white}
button.danger{background:#b42318}table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:.6rem;border-bottom:1px solid #e4e8ee}
#message{white-space:pre-wrap;color:#b42318}.muted{color:#627084;font-size:.9rem}</style></head><body>
<h1>dsmctl Gateway</h1><p class="muted">在這裡新增與登入最多 32 台 NAS；管理 token 只保存在目前瀏覽器分頁的 sessionStorage。</p>
<div class="card"><h2>管理登入</h2><input id="token" size="48" placeholder="Admin token"><button onclick="saveToken()">使用 token</button><br>
<input id="bootstrap" size="48" placeholder="首次啟用 bootstrap token"><button onclick="bootstrapAdmin()">首次啟用</button></div>
<div class="card"><h2>新增 NAS</h2><input id="name" placeholder="名稱"><input id="url" size="32" placeholder="https://nas:5001">
<input id="username" placeholder="帳號（web-login 可留空）"><select id="tls"><option value="system_ca">System CA</option><option value="pinned_fingerprint">Pinned fingerprint</option></select>
<input id="fingerprint" size="36" placeholder="SHA-256 fingerprint"><button onclick="addProfile()">新增</button></div>
<div class="card"><h2>NAS 清單</h2><button onclick="loadProfiles()">重新整理</button><div id="message"></div><table><thead><tr><th>名稱</th><th>URL</th><th>修訂</th><th>憑證</th><th>操作</th></tr></thead><tbody id="profiles"></tbody></table></div>
<script>
const $=id=>document.getElementById(id); $('token').value=sessionStorage.getItem('dsmctlAdminToken')||'';
function saveToken(){sessionStorage.setItem('dsmctlAdminToken',$('token').value.trim());loadProfiles()}
async function api(path,options={}){options.headers=Object.assign({'Content-Type':'application/json','Authorization':'Bearer '+(sessionStorage.getItem('dsmctlAdminToken')||'')},options.headers||{});let r=await fetch('/admin/api'+path,options),t=await r.text(),v;try{v=JSON.parse(t)}catch{v={error:t}}if(!r.ok)throw new Error(v.error||r.statusText);return v}
function show(e){$('message').textContent=e instanceof Error?e.message:String(e||'')}
async function bootstrapAdmin(){try{let v=await fetch('/admin/api/bootstrap',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({token:$('bootstrap').value})}).then(async r=>{let v=await r.json();if(!r.ok)throw new Error(v.error);return v});$('token').value=v.admin_token;saveToken();$('bootstrap').value=''}catch(e){show(e)}}
async function addProfile(){try{let pinned=$('tls').value==='pinned_fingerprint',fp=$('fingerprint').value.trim(),confirmed=!pinned||confirm('確認將此 SHA-256 certificate fingerprint 設為信任根：\n'+fp);if(!confirmed)return;await api('/profiles',{method:'POST',body:JSON.stringify({name:$('name').value,url:$('url').value,username:$('username').value,tls_mode:$('tls').value,certificate_fingerprint:fp,confirm_certificate_fingerprint:confirmed})});await loadProfiles()}catch(e){show(e)}}
async function loadProfiles(){try{let v=await api('/profiles'),b=$('profiles');b.textContent='';for(const p of v.profiles){let tr=document.createElement('tr');tr.innerHTML='<td></td><td></td><td></td><td></td><td></td>';tr.children[0].textContent=(p.default?'★ ':'')+p.name;tr.children[1].textContent=p.url;tr.children[2].textContent=p.revision;tr.children[3].textContent=p.session_stored?'web session':p.password_stored?'password':'未登入';let actions=tr.children[4];for(const [label,fn,cls] of [['設預設',()=>setDefault(p.name)],['測試',()=>testNAS(p.name)],['Web login',()=>webLogin(p.name)],['密碼/OTP',()=>passwordLogin(p.name)],['刪除',()=>removeNAS(p),'danger']]){let x=document.createElement('button');x.textContent=label;x.onclick=fn;if(cls)x.className=cls;actions.appendChild(x)}b.appendChild(tr)}show('')}catch(e){show(e)}}
async function setDefault(n){try{await api('/profiles/'+encodeURIComponent(n)+'/default',{method:'POST',body:'{}'});loadProfiles()}catch(e){show(e)}}
async function testNAS(n){try{show(JSON.stringify(await api('/profiles/'+encodeURIComponent(n)+'/test',{method:'POST',body:'{}'}),null,2))}catch(e){show(e)}}
async function passwordLogin(n){let password=prompt('DSM password（只用於這次 enrollment）');if(password===null)return;let otp=prompt('OTP（沒有可留空）')||'';try{await api('/profiles/'+encodeURIComponent(n)+'/credentials/password',{method:'POST',body:JSON.stringify({password,otp})});password=otp='';loadProfiles()}catch(e){password=otp='';show(e)}}
async function webLogin(n){try{let s=await api('/profiles/'+encodeURIComponent(n)+'/weblogin/start',{method:'POST',body:'{}'}),popup=window.open(s.login_url,'dsmctl_signin','width=560,height=720');let listener=async e=>{if(e.origin!==s.nas_origin)return;let d=e.data||{};if(!d.code)return;window.removeEventListener('message',listener);try{await api('/profiles/'+encodeURIComponent(n)+'/weblogin/complete',{method:'POST',body:JSON.stringify({enrollment_id:s.enrollment_id,code:d.code,rs:d.rs,state:d.state||s.state})});if(popup)popup.close();loadProfiles()}catch(x){show(x)}};window.addEventListener('message',listener)}catch(e){show(e)}}
async function removeNAS(p){if(!confirm('刪除 '+p.name+' 及其 credentials？'))return;try{await api('/profiles/'+encodeURIComponent(p.name)+'?revision='+p.revision,{method:'DELETE'});loadProfiles()}catch(e){show(e)}}
if($('token').value)loadProfiles();
</script></body></html>`
