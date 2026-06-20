package admin

const pageHTML = `{{define "layout"}}
<!doctype html>
<html lang="en">
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
      <a href="/">Dashboard</a>
      <a href="/config">Config</a>
      <a href="/certificate">Certificate</a>
      <a href="/nginx">Nginx</a>
      <a href="/service">Service</a>
      <a href="/client">Client</a>
      <a href="/logout">Logout</a>
    </nav>
    {{end}}
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
    {{if eq .Page "service"}}{{template "service" .}}{{end}}
    {{if eq .Page "client"}}{{template "client" .}}{{end}}
  </main>
</body>
</html>
{{end}}

{{define "setup"}}
<section class="panel narrow">
  <h1>Setup</h1>
  <form method="post" action="/setup">
    <label>Setup key<input name="setup_key" type="password" autocomplete="one-time-code" required></label>
    <label>Username<input name="username" autocomplete="username" required></label>
    <label>Password<input name="password" type="password" autocomplete="new-password" minlength="10" required></label>
    <button type="submit">Create admin</button>
  </form>
</section>
{{end}}

{{define "login"}}
<section class="panel narrow">
  <h1>Login</h1>
  <form method="post" action="/login">
    <label>Username<input name="username" autocomplete="username" required></label>
    <label>Password<input name="password" type="password" autocomplete="current-password" required></label>
    <button type="submit">Login</button>
  </form>
</section>
{{end}}

{{define "dashboard"}}
<section class="grid">
  <div class="panel">
    <h1>Dashboard</h1>
    <table>
      <tr><th>Env</th><td>{{.EnvPath}}</td></tr>
      <tr><th>Domain</th><td>{{.Config.Domain}}</td></tr>
      <tr><th>Control</th><td>{{.Config.ControlHost}}:{{port .Config.TunnelAddr}}</td></tr>
      <tr><th>Service</th><td><span class="badge">{{.Service.State}}</span></td></tr>
      <tr><th>Certificate</th><td>{{if .Cert.Error}}{{.Cert.Error}}{{else}}{{.Cert.NotAfter}}{{end}}</td></tr>
    </table>
  </div>
  <div class="panel">
    <h2>Checks</h2>
    <table>
      {{range .Checks}}
      <tr><th>{{.Name}}</th><td><span class="badge">{{.State}}</span></td><td>{{.Detail}}</td></tr>
      {{end}}
    </table>
  </div>
</section>
{{end}}

{{define "config"}}
<section class="panel">
  <h1>Config</h1>
  <form method="post" action="/config" class="form-grid">
    <label>Domain<input name="domain" value="{{.Config.Domain}}" required></label>
    <label>Control host<input name="control_host" value="{{.Config.ControlHost}}" required></label>
    <label>TLS cert<input name="tls_crt" value="{{.Config.TLSCrt}}" required></label>
    <label>TLS key<input name="tls_key" value="{{.Config.TLSKey}}" required></label>
    <label>Auth token<input name="auth_token" value="{{.Config.AuthToken}}"></label>
    <label>HTTP addr<input name="http_addr" value="{{.Config.HTTPAddr}}"></label>
    <label>HTTPS addr<input name="https_addr" value="{{.Config.HTTPSAddr}}"></label>
    <label>Tunnel addr<input name="tunnel_addr" value="{{.Config.TunnelAddr}}"></label>
    <label>Log level<input name="log_level" value="{{.Config.LogLevel}}"></label>
    <label>Max connections<input name="max_connections" value="{{.Config.MaxConnections}}"></label>
    <label class="wide">Extra args<input name="extra_args" value="{{.Config.ExtraArgs}}"></label>
    <label class="check"><input type="checkbox" name="new_token" value="1"> New token</label>
    <div class="actions wide">
      <button type="submit">Save</button>
    </div>
  </form>
</section>
{{end}}

{{define "certificate"}}
<section class="grid">
  <div class="panel">
    <h1>Certificate</h1>
    <table>
      <tr><th>Path</th><td>{{.Cert.Path}}</td></tr>
      <tr><th>Subject</th><td>{{.Cert.Subject}}</td></tr>
      <tr><th>Issuer</th><td>{{.Cert.Issuer}}</td></tr>
      <tr><th>Expires</th><td>{{.Cert.NotAfter}}</td></tr>
      <tr><th>Domain</th><td><span class="badge">{{.Cert.DomainOK}}</span></td></tr>
      <tr><th>Wildcard</th><td><span class="badge">{{.Cert.WildcardOK}}</span></td></tr>
      <tr><th>Names</th><td>{{.Cert.DNSNames}}</td></tr>
    </table>
  </div>
  <div class="panel">
    <h2>ACME</h2>
    <form method="post" action="/certificate/issue">
      <label>DNS plugin<input name="dns_plugin" placeholder="dns_cf" required></label>
      <label>Email<input name="account_email" type="email"></label>
      <label>Env<textarea name="env_vars" rows="7" spellcheck="false" placeholder="CF_Token=..."></textarea></label>
      <button type="submit">Issue</button>
    </form>
  </div>
</section>
{{end}}

{{define "nginx"}}
<section class="panel">
  <h1>Nginx</h1>
  <form method="post" action="/nginx">
    <label>Path<input name="path" value="{{.NginxPath}}"></label>
    <label>Config<textarea rows="24" spellcheck="false" readonly>{{.NginxConfig}}</textarea></label>
    <div class="actions">
      <button name="action" value="write" type="submit">Write</button>
      <button name="action" value="test" type="submit">Test</button>
      <button name="action" value="reload" type="submit">Reload</button>
    </div>
  </form>
</section>
{{end}}

{{define "service"}}
<section class="grid">
  <div class="panel">
    <h1>Service</h1>
    <table>
      <tr><th>Name</th><td>ngrokd</td></tr>
      <tr><th>State</th><td><span class="badge">{{.Service.State}}</span></td></tr>
      <tr><th>Error</th><td>{{.Service.Error}}</td></tr>
    </table>
    <form method="post" action="/service" class="actions">
      <button name="action" value="start" type="submit">Start</button>
      <button name="action" value="restart" type="submit">Restart</button>
      <button name="action" value="stop" type="submit">Stop</button>
    </form>
  </div>
  <div class="panel">
    <h2>Logs</h2>
    <pre class="logs">{{.Service.Logs}}</pre>
  </div>
</section>
{{end}}

{{define "client"}}
<section class="panel">
  <h1>Client</h1>
  <form method="post" action="/client">
    <label>Path<input value="{{.ClientPath}}" readonly></label>
    <label>Config<textarea rows="16" spellcheck="false" readonly>{{.ClientConfig}}</textarea></label>
    <button type="submit">Write</button>
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
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
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
  main { width: min(100vw - 20px, 1180px); margin-top: 16px; }
  .panel { padding: 16px; }
}
`
