package admin

const pageHTML = `{{define "layout"}}
<!doctype html>
<html lang="{{if eq .Lang "zh-CN"}}zh-CN{{else}}en{{end}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - ngrok admin</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <header class="top">
    <a class="brand" href="/">ngrok admin</a>
    {{if .Authed}}
    <nav>
      <a href="/">{{tr .Lang "nav_dashboard"}}</a>
      <a href="/config">{{tr .Lang "nav_config"}}</a>
      <a href="/certificate">{{tr .Lang "nav_certificate"}}</a>
      <a href="/build">{{tr .Lang "nav_build"}}</a>
      <a href="/service">{{tr .Lang "nav_service"}}</a>
      <a href="/logout">{{tr .Lang "nav_logout"}}</a>
    </nav>
    {{end}}
    <div class="language">
      <a href="?lang=zh-CN">中文</a>
      <span>/</span>
      <a href="?lang=en">English</a>
    </div>
  </header>
  <main>
    {{if .Message}}<pre class="notice">{{.Message}}</pre>{{end}}
    {{if .Error}}<pre class="error">{{.Error}}</pre>{{end}}
    {{if eq .Page "setup"}}{{template "setup" .}}{{end}}
    {{if eq .Page "login"}}{{template "login" .}}{{end}}
    {{if eq .Page "dashboard"}}{{template "dashboard" .}}{{end}}
    {{if eq .Page "config"}}{{template "config" .}}{{end}}
    {{if eq .Page "certificate"}}{{template "certificate" .}}{{end}}
    {{if eq .Page "nginx"}}{{template "nginx" .}}{{end}}
    {{if eq .Page "build"}}{{template "build" .}}{{end}}
    {{if eq .Page "service"}}{{template "service" .}}{{end}}
  </main>
</body>
</html>
{{end}}

{{define "setup"}}
<section class="panel narrow">
  <h1>{{tr .Lang "Setup"}}</h1>
  <form method="post" action="/setup">
    <label>{{tr .Lang "setup_key"}}<input name="setup_key" type="password" autocomplete="one-time-code" required></label>
    <label>{{tr .Lang "username"}}<input name="username" autocomplete="username" required></label>
    <label>{{tr .Lang "password"}}<input name="password" type="password" autocomplete="new-password" minlength="10" required></label>
    <button type="submit">{{tr .Lang "create_admin"}}</button>
  </form>
</section>
{{end}}

{{define "login"}}
<section class="panel narrow">
  <h1>{{tr .Lang "Login"}}</h1>
  <form method="post" action="/login">
    <label>{{tr .Lang "username"}}<input name="username" autocomplete="username" required></label>
    <label>{{tr .Lang "password"}}<input name="password" type="password" autocomplete="current-password" required></label>
    <button type="submit">{{tr .Lang "login"}}</button>
  </form>
</section>
{{end}}

{{define "dashboard"}}
<section class="panel">
  <h1>{{tr .Lang "guide_title"}}</h1>
  <div class="setup-main">
    <div>
      <h2>{{tr .Lang "guide_dns_title"}}</h2>
      <pre class="snippet">{{.Config.ControlHost}} A/AAAA &lt;server-ip&gt;
*.{{.Config.Domain}} A/AAAA &lt;server-ip&gt;</pre>
    </div>
    <div class="next">
      <div>
        <span class="eyebrow">{{tr .Lang "next_step"}}</span>
        <strong>{{tr .Lang .NextStep.Title}}</strong>
      </div>
      <a class="link-button" href="{{.NextStep.URL}}">{{tr .Lang "open"}}</a>
    </div>
  </div>
  <table class="steps-table">
    {{range .Steps}}
    <tr>
      <th>{{tr $.Lang .Title}}</th>
      <td>{{.Detail}}</td>
      <td><span class="badge">{{state $.Lang .State}}</span></td>
      <td><a href="{{.URL}}">{{tr $.Lang "open"}}</a></td>
    </tr>
    {{end}}
  </table>
</section>
{{end}}

{{define "config"}}
<section class="panel">
  <h1>{{tr .Lang "Config"}}</h1>
  <form method="post" action="/config" class="form-grid">
    <label>{{tr .Lang "domain"}}<input name="domain" value="{{.Config.Domain}}" required></label>
    <label>{{tr .Lang "control_host"}}<input name="control_host" value="{{.Config.ControlHost}}" required></label>
    <label>{{tr .Lang "http_addr"}}<input name="http_addr" value="{{.Config.HTTPAddr}}"></label>
    <label>{{tr .Lang "tunnel_addr"}}<input name="tunnel_addr" value="{{.Config.TunnelAddr}}"></label>
    <label class="wide">{{tr .Lang "auth_token"}}<input name="auth_token" value="{{.Config.AuthToken}}"></label>
    <label class="check"><input type="checkbox" name="new_token" value="1"> {{tr .Lang "new_token"}}</label>
    <div class="actions wide">
      <button type="submit">{{tr .Lang "save"}}</button>
    </div>
  </form>
</section>
{{end}}

{{define "certificate"}}
<section class="panel">
  <h1>{{tr .Lang "Certificate"}}</h1>
  <div class="toolbar">
    <form method="post" action="/certificate/domain" class="inline-form">
      <input name="domain" placeholder="example.com" required>
      <button type="submit">{{tr .Lang "add_domain"}}</button>
    </form>
    <form method="get" action="/certificate" class="inline-form">
      <button type="submit">{{tr .Lang "refresh_dns"}}</button>
    </form>
  </div>
  <table class="domain-table">
    <tr>
      <th>{{tr .Lang "domain_item"}}</th>
      <th>{{tr .Lang "dns_status"}}</th>
      <th>{{tr .Lang "cert_status"}}</th>
      <th>{{tr .Lang "action"}}</th>
    </tr>
    {{range .CertRows}}
    <tr>
      <td><strong>{{.Domain}}</strong></td>
      <td>
        <span class="badge">{{state $.Lang .RootDNS.State}}</span> {{tr $.Lang "root_domain"}}: {{.RootDNS.Detail}}<br>
        <span class="badge">{{state $.Lang .WildcardDNS.State}}</span> {{tr $.Lang "wildcard_domain"}}: {{.WildcardDNS.Detail}}
      </td>
      <td><span class="badge">{{state $.Lang .CertState}}</span> {{.CertDetail}}</td>
      <td>
        <form method="post" action="/certificate/issue" class="table-form">
          <input type="hidden" name="domain" value="{{.Domain}}">
          <button type="submit">{{tr $.Lang "issue"}}</button>
        </form>
      </td>
    </tr>
    {{end}}
  </table>
  <table class="compact-table meta-table">
    <tr><th>{{tr .Lang "output_dir"}}</th><td>{{.CertDir}}</td></tr>
  </table>
</section>
{{end}}

{{define "build"}}
<section class="panel">
  <h1>{{tr .Lang "Build"}}</h1>
  <table>
    <tr><th>{{tr .Lang "work_dir"}}</th><td colspan="3">{{.WorkDir}}</td></tr>
    {{range .Binaries}}
    <tr><th>{{.Name}}</th><td><span class="badge">{{state $.Lang .State}}</span></td><td>{{.Size}}</td><td>{{if .URL}}<a href="{{.URL}}">{{tr $.Lang "download"}}</a>{{end}}</td></tr>
    {{end}}
    <tr><th>client.yml</th><td><span class="badge">{{state .Lang "ok"}}</span></td><td></td><td><a href="/download/client.yml">{{tr .Lang "download"}}</a></td></tr>
  </table>
  <form method="post" action="/build" class="actions build-actions">
    <button name="target" value="server" type="submit">{{tr .Lang "build_server"}}</button>
    <button name="target" value="client" type="submit">{{tr .Lang "build_client"}}</button>
  </form>
  {{if .BuildOutput}}<pre class="logs">{{.BuildOutput}}</pre>{{end}}
</section>
{{end}}

{{define "nginx"}}
<section class="panel">
  <h1>Nginx</h1>
  <form method="post" action="/nginx">
    <label>{{tr .Lang "path"}}<input name="path" value="{{.NginxPath}}"></label>
    <label>{{tr .Lang "config"}}<textarea rows="24" spellcheck="false" readonly>{{.NginxConfig}}</textarea></label>
    <div class="actions">
      <button name="action" value="write" type="submit">{{tr .Lang "write"}}</button>
      <button name="action" value="test" type="submit">{{tr .Lang "test"}}</button>
      <button name="action" value="reload" type="submit">{{tr .Lang "reload"}}</button>
    </div>
  </form>
</section>
{{end}}

{{define "service"}}
<section class="panel">
  <h1>{{tr .Lang "Service"}}</h1>
  <table>
    <tr><th>{{tr .Lang "name"}}</th><td>ngrokd</td></tr>
    <tr><th>{{tr .Lang "state"}}</th><td><span class="badge">{{state .Lang .Service.State}}</span></td></tr>
    <tr><th>{{tr .Lang "error"}}</th><td>{{.Service.Error}}</td></tr>
  </table>
  <form method="post" action="/service" class="actions build-actions">
    <button name="action" value="start" type="submit">{{tr .Lang "start"}}</button>
    <button name="action" value="restart" type="submit">{{tr .Lang "restart"}}</button>
    <button name="action" value="stop" type="submit">{{tr .Lang "stop"}}</button>
  </form>
  <pre class="logs">{{.Service.Logs}}</pre>
</section>
{{end}}

`

const styleCSS = `
:root {
  color-scheme: light;
  --bg: #f5f7fa;
  --panel: #ffffff;
  --text: #20242a;
  --muted: #606975;
  --line: #d9dee7;
  --accent: #1f6feb;
  --danger: #b42318;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
.top {
  min-height: 56px;
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 0 28px;
  border-bottom: 1px solid var(--line);
  background: #fff;
}
.brand {
  color: var(--text);
  font-weight: 700;
  text-decoration: none;
}
nav {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
nav a {
  color: var(--muted);
  text-decoration: none;
}
nav a:hover { color: var(--accent); }
.language {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  white-space: nowrap;
}
.language a {
  color: var(--muted);
  text-decoration: none;
}
.language a:hover { color: var(--accent); }
main {
  width: min(1180px, calc(100vw - 32px));
  margin: 28px auto;
}
h1, h2 {
  margin: 0 0 18px;
  line-height: 1.2;
}
h1 { font-size: 24px; }
h2 { font-size: 18px; }
.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}
.panel {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 22px;
}
.setup-main {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 360px);
  gap: 16px;
  align-items: stretch;
  margin-bottom: 18px;
}
.snippet {
  margin: 0;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: #f8fafc;
  color: var(--text);
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
.narrow {
  max-width: 460px;
  margin: 80px auto 0;
}
form {
  display: grid;
  gap: 14px;
}
.form-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.wide { grid-column: 1 / -1; }
label {
  display: grid;
  gap: 6px;
  color: var(--muted);
  font-weight: 600;
}
input, textarea {
  width: 100%;
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 10px 11px;
  color: var(--text);
  background: #fff;
  font: inherit;
}
textarea {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  resize: vertical;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
}
.check input { width: auto; }
button {
  width: fit-content;
  border: 1px solid #195bc2;
  border-radius: 6px;
  padding: 10px 14px;
  color: #fff;
  background: var(--accent);
  font-weight: 700;
  cursor: pointer;
}
button:hover { filter: brightness(0.95); }
.link-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 38px;
  border: 1px solid #195bc2;
  border-radius: 6px;
  padding: 9px 13px;
  color: #fff;
  background: var(--accent);
  font-weight: 700;
  text-decoration: none;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
}
.inline-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
.inline-form input {
  width: min(320px, 72vw);
}
.table-form {
  display: block;
}
.domain-table td:nth-child(2),
.domain-table td:nth-child(3) {
  overflow-wrap: anywhere;
}
.domain-table th:last-child,
.domain-table td:last-child {
  width: 110px;
  text-align: right;
}
.meta-table {
  margin-top: 18px;
}
.build-actions {
  margin-top: 16px;
}
.next {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: #f8fafc;
}
.next strong {
  display: block;
  margin-top: 3px;
  font-size: 18px;
}
.eyebrow {
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}
.steps {
  display: grid;
  gap: 10px;
}
.step {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--line);
}
.step:last-child { border-bottom: 0; }
.step strong {
  display: block;
  margin-bottom: 3px;
}
.step span:not(.badge) {
  display: block;
  color: var(--muted);
  overflow-wrap: anywhere;
}
.step a {
  color: var(--accent);
  font-weight: 700;
  text-decoration: none;
}
table {
  width: 100%;
  border-collapse: collapse;
}
.steps-table td:last-child,
.compact-table td:last-child {
  width: 90px;
  text-align: right;
}
th, td {
  padding: 9px 0;
  border-bottom: 1px solid var(--line);
  text-align: left;
  vertical-align: top;
}
th {
  width: 150px;
  color: var(--muted);
  font-weight: 700;
}
.badge {
  display: inline-block;
  min-width: 52px;
  padding: 2px 7px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: #f8fafc;
  color: var(--text);
  text-align: center;
}
.notice, .error, .logs {
  white-space: pre-wrap;
  overflow: auto;
  border-radius: 6px;
  padding: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
.notice {
  border: 1px solid #b6d4fe;
  background: #eef6ff;
}
.error {
  border: 1px solid #f1b5b5;
  background: #fff2f2;
  color: var(--danger);
}
.logs {
  min-height: 280px;
  max-height: 520px;
  background: #101418;
  color: #e9edf2;
}
@media (max-width: 800px) {
  .top { align-items: flex-start; flex-direction: column; padding: 14px 18px; }
  .grid, .form-grid { grid-template-columns: 1fr; }
  .setup-main { grid-template-columns: 1fr; }
  .language { margin-left: 0; }
  .step { grid-template-columns: 1fr; }
  main { width: min(100vw - 20px, 1180px); margin-top: 16px; }
  .panel { padding: 16px; }
}
`
