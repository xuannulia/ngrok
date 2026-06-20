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
      <a href="/nginx">{{tr .Lang "nav_nginx"}}</a>
      <a href="/build">{{tr .Lang "nav_build"}}</a>
      <a href="/service">{{tr .Lang "nav_service"}}</a>
      <a href="/client">{{tr .Lang "nav_client"}}</a>
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
    {{if eq .Page "client"}}{{template "client" .}}{{end}}
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
<section class="grid flow-grid">
  <div class="panel">
    <h1>{{tr .Lang "guide_title"}}</h1>
    <div class="guide-block">
      <h2>{{tr .Lang "guide_dns_title"}}</h2>
      <pre class="snippet">{{.Config.ControlHost}} A/AAAA &lt;server-ip&gt;
*.{{.Config.Domain}} A/AAAA &lt;server-ip&gt;</pre>
    </div>
    <div class="guide-block">
      <h2>{{tr .Lang "guide_order_title"}}</h2>
      <ol class="guide-list">
        <li>{{tr .Lang "guide_step_cert"}}</li>
        <li>{{tr .Lang "guide_step_nginx"}}</li>
        <li>{{tr .Lang "guide_step_build"}}</li>
        <li>{{tr .Lang "guide_step_service"}}</li>
        <li>{{tr .Lang "guide_step_client"}}</li>
      </ol>
    </div>
    <div class="next">
      <div>
        <span class="eyebrow">{{tr .Lang "next_step"}}</span>
        <strong>{{tr .Lang .NextStep.Title}}</strong>
      </div>
      <a class="link-button" href="{{.NextStep.URL}}">{{tr .Lang "open"}}</a>
    </div>
    <div class="steps">
      {{range .Steps}}
      <div class="step {{.State}}">
        <div>
          <strong>{{tr $.Lang .Title}}</strong>
          <span>{{.Detail}}</span>
        </div>
        <span class="badge">{{state $.Lang .State}}</span>
        <a href="{{.URL}}">{{tr $.Lang "open"}}</a>
      </div>
      {{end}}
    </div>
  </div>
  <div class="panel">
    <h2>{{tr .Lang "status"}}</h2>
    <table>
      <tr><th>{{tr .Lang "env"}}</th><td>{{.EnvPath}}</td></tr>
      <tr><th>{{tr .Lang "domain"}}</th><td>{{.Config.Domain}}</td></tr>
      <tr><th>{{tr .Lang "control"}}</th><td>{{.Config.ControlHost}}:{{port .Config.TunnelAddr}}</td></tr>
      <tr><th>{{tr .Lang "service"}}</th><td><span class="badge">{{state .Lang .Service.State}}</span></td></tr>
      <tr><th>{{tr .Lang "certificate"}}</th><td>{{if .Cert.Error}}{{.Cert.Error}}{{else}}{{.Cert.NotAfter}}{{end}}</td></tr>
    </table>
  </div>
  <div class="panel">
    <h2>{{tr .Lang "checks"}}</h2>
    <table>
      {{range .Checks}}
      <tr><th>{{tr $.Lang .Name}}</th><td><span class="badge">{{state $.Lang .State}}</span></td><td>{{.Detail}}</td></tr>
      {{end}}
    </table>
  </div>
</section>
{{end}}

{{define "config"}}
<section class="panel">
  <h1>{{tr .Lang "Config"}}</h1>
  <form method="post" action="/config" class="form-grid">
    <label>{{tr .Lang "domain"}}<input name="domain" value="{{.Config.Domain}}" required></label>
    <label>{{tr .Lang "control_host"}}<input name="control_host" value="{{.Config.ControlHost}}" required></label>
    <label>{{tr .Lang "tls_cert"}}<input name="tls_crt" value="{{.Config.TLSCrt}}" required></label>
    <label>{{tr .Lang "tls_key"}}<input name="tls_key" value="{{.Config.TLSKey}}" required></label>
    <label>{{tr .Lang "auth_token"}}<input name="auth_token" value="{{.Config.AuthToken}}"></label>
    <label>{{tr .Lang "http_addr"}}<input name="http_addr" value="{{.Config.HTTPAddr}}"></label>
    <label>{{tr .Lang "https_addr"}}<input name="https_addr" value="{{.Config.HTTPSAddr}}"></label>
    <label>{{tr .Lang "tunnel_addr"}}<input name="tunnel_addr" value="{{.Config.TunnelAddr}}"></label>
    <label>{{tr .Lang "log_level"}}<input name="log_level" value="{{.Config.LogLevel}}"></label>
    <label>{{tr .Lang "max_connections"}}<input name="max_connections" value="{{.Config.MaxConnections}}"></label>
    <label class="wide">{{tr .Lang "extra_args"}}<input name="extra_args" value="{{.Config.ExtraArgs}}"></label>
    <label class="check"><input type="checkbox" name="new_token" value="1"> {{tr .Lang "new_token"}}</label>
    <div class="actions wide">
      <button type="submit">{{tr .Lang "save"}}</button>
    </div>
  </form>
</section>
{{end}}

{{define "certificate"}}
<section class="grid">
  <div class="panel">
    <h1>{{tr .Lang "Certificate"}}</h1>
    <table>
      <tr><th>{{tr .Lang "path"}}</th><td>{{.Cert.Path}}</td></tr>
      <tr><th>{{tr .Lang "subject"}}</th><td>{{.Cert.Subject}}</td></tr>
      <tr><th>{{tr .Lang "issuer"}}</th><td>{{.Cert.Issuer}}</td></tr>
      <tr><th>{{tr .Lang "expires"}}</th><td>{{.Cert.NotAfter}}</td></tr>
      <tr><th>{{tr .Lang "domain"}}</th><td><span class="badge">{{state .Lang .Cert.DomainOK}}</span></td></tr>
      <tr><th>{{tr .Lang "wildcard"}}</th><td><span class="badge">{{state .Lang .Cert.WildcardOK}}</span></td></tr>
      <tr><th>{{tr .Lang "names"}}</th><td>{{.Cert.DNSNames}}</td></tr>
      <tr><th>{{tr .Lang "output_dir"}}</th><td>{{.CertDir}}</td></tr>
    </table>
  </div>
  <div class="panel">
    <h2>{{tr .Lang "dns_check"}}</h2>
    <table>
      {{range .DNSChecks}}
      <tr><th>{{.Name}}</th><td><span class="badge">{{state $.Lang .State}}</span></td><td>{{.Detail}}</td></tr>
      {{end}}
    </table>
  </div>
  <div class="panel">
    <h2>ACME</h2>
    <form method="post" action="/certificate/issue">
      <label>{{tr .Lang "domains"}}<textarea name="domains" rows="6" spellcheck="false">{{.Domains}}</textarea></label>
      <label>{{tr .Lang "dns_plugin"}}<input name="dns_plugin" placeholder="dns_cf" required></label>
      <label>{{tr .Lang "email"}}<input name="account_email" type="email"></label>
      <label>{{tr .Lang "env_vars"}}<textarea name="env_vars" rows="7" spellcheck="false" placeholder="CF_Token=..."></textarea></label>
      <button type="submit">{{tr .Lang "issue"}}</button>
    </form>
  </div>
</section>
{{end}}

{{define "build"}}
<section class="grid">
  <div class="panel">
    <h1>{{tr .Lang "Build"}}</h1>
    <table>
      <tr><th>{{tr .Lang "work_dir"}}</th><td>{{.WorkDir}}</td></tr>
      {{range .Binaries}}
      <tr><th>{{.Name}}</th><td><span class="badge">{{state $.Lang .State}}</span></td><td>{{.Size}}</td><td><a href="{{.URL}}">{{tr $.Lang "download"}}</a></td></tr>
      {{end}}
      <tr><th>client.yml</th><td><span class="badge">{{state .Lang "ok"}}</span></td><td></td><td><a href="/download/client.yml">{{tr .Lang "download"}}</a></td></tr>
    </table>
    <form method="post" action="/build" class="actions build-actions">
      <button name="target" value="server" type="submit">{{tr .Lang "build_server"}}</button>
      <button name="target" value="client" type="submit">{{tr .Lang "build_client"}}</button>
      <button name="target" value="admin" type="submit">{{tr .Lang "build_admin"}}</button>
      <button name="target" value="all" type="submit">{{tr .Lang "build_all"}}</button>
    </form>
  </div>
  <div class="panel">
    <h2>{{tr .Lang "output"}}</h2>
    <pre class="logs">{{.BuildOutput}}</pre>
  </div>
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
<section class="grid">
  <div class="panel">
    <h1>{{tr .Lang "Service"}}</h1>
    <table>
      <tr><th>{{tr .Lang "name"}}</th><td>ngrokd</td></tr>
      <tr><th>{{tr .Lang "state"}}</th><td><span class="badge">{{state .Lang .Service.State}}</span></td></tr>
      <tr><th>{{tr .Lang "error"}}</th><td>{{.Service.Error}}</td></tr>
    </table>
    <form method="post" action="/service" class="actions">
      <button name="action" value="start" type="submit">{{tr .Lang "start"}}</button>
      <button name="action" value="restart" type="submit">{{tr .Lang "restart"}}</button>
      <button name="action" value="stop" type="submit">{{tr .Lang "stop"}}</button>
    </form>
  </div>
  <div class="panel">
    <h2>{{tr .Lang "logs"}}</h2>
    <pre class="logs">{{.Service.Logs}}</pre>
  </div>
</section>
{{end}}

{{define "client"}}
<section class="panel">
  <h1>{{tr .Lang "Client"}}</h1>
  <form method="post" action="/client">
    <label>{{tr .Lang "path"}}<input value="{{.ClientPath}}" readonly></label>
    <label>{{tr .Lang "config"}}<textarea rows="16" spellcheck="false" readonly>{{.ClientConfig}}</textarea></label>
    <button type="submit">{{tr .Lang "write"}}</button>
  </form>
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
.flow-grid {
  grid-template-columns: minmax(0, 1.25fr) minmax(320px, .75fr);
}
.panel {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 22px;
}
.guide-block {
  margin-bottom: 18px;
}
.guide-block h2 {
  margin-bottom: 10px;
}
.guide-list {
  margin: 0;
  padding-left: 22px;
}
.guide-list li {
  margin: 7px 0;
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
  .language { margin-left: 0; }
  .step { grid-template-columns: 1fr; }
  main { width: min(100vw - 20px, 1180px); margin-top: 16px; }
  .panel { padding: 16px; }
}
`
