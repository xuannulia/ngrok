# ngrok v1 自托管分支

[English README](README.md)

这是基于已归档 ngrok v1 的自托管维护分支。

目标是保留 ngrok v1 简单的使用方式，同时补齐当前公网环境下必须处理的部分：鉴权、依赖清理、多域名、证书热更新、服务化部署和更安全的默认值。

它适合开发者自用、团队内网穿透、私有基础设施；不定位为商业隧道平台。

## 当前能力

- 服务端和客户端必须配置认证 token。
- 服务端 TLS 最低版本为 TLS 1.2。
- ngrok 协议消息大小有限制。
- `-maxConnections` 限制公网连接数量。
- 默认禁止客户端指定公网 TCP 端口。
- HTTP inspection 请求/响应体最多保留 1 MB。
- inspection UI 暴露到非本机地址时必须配置鉴权。
- 支持多域名。
- 支持通过重复 `-domainCert` 配置 SNI 证书。
- 证书文件续期后支持热更新。
- 提供 `systemd`、`launchd`、Nginx 模板。
- 移除或替换了一批过时依赖。

## 文档

- [迁移说明](MIGRATION.md)
- [安全策略](SECURITY.md)
- [自托管参考](docs/SELFHOSTING.md)
- [第三方组件说明](THIRD_PARTY_NOTICES.md)

## 构建

项目仍使用原始 GOPATH 结构，直接用 Makefile：

```bash
make all
make release-all
```

输出：

```text
bin/ngrok
bin/ngrokd
```

## 最小部署流程

按下面流程可以完成一个位于 Nginx 后面的最小可用部署：

1. 创建 DNS 记录：

   ```text
   ngrok.example.com A/AAAA <server-ip>
   *.example.com     A/AAAA <server-ip>
   ```

2. 通过 DNS-01 签发公开 CA 泛域名证书：

   ```text
   example.com
   *.example.com
   ```

3. 在服务器构建并安装：

   ```bash
   make release-all
   sudo sh deploy/install-systemd.sh
   ```

4. 编辑 `/etc/ngrok/ngrokd.env`：

   ```text
   NGROKD_DOMAIN=example.com
   NGROKD_TLS_CRT=/etc/ngrok/tls/fullchain.pem
   NGROKD_TLS_KEY=/etc/ngrok/tls/privkey.pem
   NGROKD_AUTH_TOKEN=<long-random-token>
   NGROKD_HTTP_ADDR=127.0.0.1:8080
   NGROKD_HTTPS_ADDR=
   NGROKD_TUNNEL_ADDR=0.0.0.0:4443
   ```

5. 配置 Nginx，把 `*.example.com` 反代到 `127.0.0.1:8080`。
   可以使用本文里的 Nginx 示例，或参考 `deploy/nginx/ngrok-http.conf`。

6. 启动服务端：

   ```bash
   sudo systemctl enable --now ngrokd
   sudo systemctl status ngrokd
   ```

7. 配置客户端：

   ```yaml
   server_addr: ngrok.example.com:4443
   auth_token: <long-random-token>
   trust_host_root_certs: true

   tunnels:
     app:
       proto:
         http: 127.0.0.1:3000
       subdomain: app
   ```

8. 启动客户端：

   ```bash
   ./bin/ngrok -config=client.yml start-all
   ```

预期结果：

```text
http://app.example.com
https://app.example.com
```

## 服务端

最小启动方式：

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -authToken=change-me-long-random-token
```

也可以通过环境变量提供 token：

```bash
NGROK_AUTH_TOKEN=change-me-long-random-token ./bin/ngrokd -domain=example.com
```

不要在公网运行空 token、弱 token 或误共享的 token。

常用参数：

```text
-domain              主域名，subdomain 隧道只基于它生成。
-domainCert          额外域名证书映射，可重复。
-tlsCrt              默认 TLS 证书文件。
-tlsKey              默认 TLS 私钥文件。
-httpAddr            HTTP 隧道入口，空值表示禁用。
-httpsAddr           HTTPS 隧道入口，空值表示禁用。
-tunnelAddr          客户端控制/代理连接入口。
-authToken           客户端认证 token。
-maxConnections      最大公网/隧道连接数，默认 1024。
-allowRemotePorts    是否允许客户端指定 TCP 公网端口，默认 false。
```

## 客户端

示例配置：

```yaml
server_addr: ngrok.example.com:4443
auth_token: change-me-long-random-token
trust_host_root_certs: true
inspect_addr: 127.0.0.1:4040

tunnels:
  app:
    proto:
      http: 127.0.0.1:3000
    subdomain: app
```

启动：

```bash
./bin/ngrok -config=client.yml start-all
```

`subdomain: app` 只会作用于服务端主 `-domain`：

```text
app.example.com
```

如果要使用其它已配置域名，写显式 `hostname`：

```yaml
tunnels:
  app-example-net:
    proto:
      http: 127.0.0.1:3000
    hostname: app.example.net
```

服务端会拒绝不属于已配置域名后缀的 hostname。

## 多域名

一个服务端可同时服务多个基础域名：

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/example.net/fullchain.pem:/etc/ngrok/tls/example.net/privkey.pem \
  -domainCert=example.org:/etc/ngrok/tls/example.org/fullchain.pem:/etc/ngrok/tls/example.org/privkey.pem \
  -authToken=change-me-long-random-token
```

DNS：

```text
*.example.com    A/AAAA  <server-ip>
*.example.net    A/AAAA  <server-ip>
*.example.org    A/AAAA  <server-ip>
ngrok.example.com A/AAAA <server-ip>
```

根域名是否解析取决于你是否要直接使用根域名作为隧道地址。

## 证书

优先使用公开 CA 签发的证书。客户端配置：

```yaml
trust_host_root_certs: true
```

公开 CA 证书正常续期时，客户端不需要重新编译。

### DNS-01 泛域名证书

泛域名证书需要 DNS-01 验证。DNS 服务商不固定，只要你的 ACME 客户端能通过 API 修改 DNS 记录即可。

常见 ACME 客户端：

- `acme.sh`
- `lego`
- 带 DNS 插件的 `certbot`

通用流程：

```bash
# acme.sh。把 dns_provider 换成你使用的 DNS 插件。
export DNS_PROVIDER_API_KEY=...
acme.sh --issue --dns dns_provider -d example.com -d '*.example.com'
acme.sh --install-cert -d example.com \
  --fullchain-file /etc/ngrok/tls/fullchain.pem \
  --key-file /etc/ngrok/tls/privkey.pem \
  --reloadcmd 'systemctl reload nginx'

# lego。把 provider 和凭据换成你使用的 DNS 插件。
DNS_PROVIDER_API_KEY=... lego \
  --dns provider \
  --domains example.com \
  --domains '*.example.com' \
  --path /etc/ngrok/acme \
  run

# certbot。把 dns-provider 换成已安装的 DNS 插件名。
certbot certonly \
  --authenticator dns-provider \
  -d example.com \
  -d '*.example.com'
```

续期后按服务用户需要安装证书文件和权限。只有 Nginx 负责浏览器 HTTPS 时才需要重载 Nginx。

如果 `ngrokd` 负责 `-tunnelAddr` 或 `-httpsAddr` 的 TLS，它通常不需要重启。它会持有当前内存证书，只在证书接近过期时检查证书/私钥文件是否变化，并在新 TLS 握手时使用新证书。已有连接不受影响。

如果浏览器 HTTPS 由 Nginx 终止，证书续期后需要重载 Nginx。`ngrokd` 的证书热更新仍适用于客户端控制通道。

## 与 Nginx 并用

当服务器已有 Nginx 占用 80/443 时，不要让 ngrokd 直接抢占这两个端口。

### 方案 A：Nginx 终止 HTTPS

适合让 Nginx 统一管理公网 Web TLS，然后转发到 ngrokd HTTP 入口。

ngrokd：

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -httpAddr=127.0.0.1:8080 \
  -httpsAddr= \
  -tunnelAddr=0.0.0.0:4443 \
  -authToken=change-me-long-random-token
```

Nginx：

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    "" close;
}

server {
    listen 80;
    server_name *.example.com;

    client_max_body_size 0;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_http_version 1.1;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:8080;
    }
}

server {
    listen 443 ssl;
    http2 on;
    server_name *.example.com;

    ssl_certificate /etc/ngrok/tls/fullchain.pem;
    ssl_certificate_key /etc/ngrok/tls/privkey.pem;

    client_max_body_size 0;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_http_version 1.1;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:8080;
    }
}
```

### 方案 B：HTTPS 透传给 ngrokd

适合让 ngrokd 自己终止 HTTPS 隧道，并通过 SNI 选择证书。

ngrokd：

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example.com/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example.com/privkey.pem \
  -httpsAddr=127.0.0.1:8443 \
  -tunnelAddr=0.0.0.0:4443 \
  -authToken=change-me-long-random-token
```

Nginx `stream {}`：

```nginx
map $ssl_preread_server_name $backend {
    ~(^|\.)example\.com$ 127.0.0.1:8443;
    default 127.0.0.1:9443;
}

server {
    listen 443;
    proxy_pass $backend;
    ssl_preread on;
}
```

`default` 后端应指向你原本的 HTTPS 服务，避免影响其它站点。

模板：

```text
deploy/nginx/ngrok-http.conf
deploy/nginx/ngrok-stream.conf
```

## 服务化运行

Linux 服务端：

```bash
make release-all
sudo sh deploy/install-systemd.sh
sudo editor /etc/ngrok/ngrokd.env
sudo systemctl enable --now ngrokd
sudo systemctl status ngrokd
```

日志：

```bash
journalctl -u ngrokd -f
```

Linux 客户端：

```bash
sudo cp deploy/systemd/client.yml.example /etc/ngrok/client.yml
sudo editor /etc/ngrok/client.yml
sudo systemctl enable --now ngrok-client@client
```

macOS 模板：

```text
deploy/launchd/com.example.ngrokd.plist
deploy/launchd/com.example.ngrok-client.plist
```

## 部署案例

目标：

```text
主域名：       example.com
额外域名：     example.net, example.org
控制服务：     ngrok.example.com:4443
隧道域名：     app.example.com, api.example.net, admin.example.org
Nginx 占用：   80 和 443
ngrokd HTTP：  127.0.0.1:8080
ngrokd 控制端：0.0.0.0:4443
```

DNS：

```text
ngrok.example.com A 203.0.113.10
*.example.com     A 203.0.113.10
*.example.net     A 203.0.113.10
*.example.org     A 203.0.113.10
```

服务端：

```bash
NGROK_AUTH_TOKEN=change-me-long-random-token ./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/fullchain.pem:/etc/ngrok/tls/privkey.pem \
  -domainCert=example.org:/etc/ngrok/tls/fullchain.pem:/etc/ngrok/tls/privkey.pem \
  -httpAddr=127.0.0.1:8080 \
  -httpsAddr= \
  -tunnelAddr=0.0.0.0:4443
```

客户端：

```yaml
server_addr: ngrok.example.com:4443
auth_token: change-me-long-random-token
trust_host_root_certs: true

tunnels:
  web:
    proto:
      http: 127.0.0.1:3000
    subdomain: app

  api:
    proto:
      http: 127.0.0.1:4000
    hostname: api.example.net

  admin:
    proto:
      http: 127.0.0.1:5000
    hostname: admin.example.org
```

结果：

```text
http://app.example.com
https://app.example.com
http://api.example.net
https://api.example.net
http://admin.example.org
https://admin.example.org
```

## 安全注意事项

- 使用足够长的随机 `auth_token`。
- 客户端配置文件只允许运行客户端的用户读取。
- 服务端环境文件只允许 root 和服务用户读取。
- inspection UI 尽量绑定 `127.0.0.1`；如果暴露到其它地址，必须配置 `inspect_auth`。
- 不要给不可信客户端开启 `-allowRemotePorts`。
- 公网服务前面仍应配置防火墙和限流。
- DNS API 密钥如果进过聊天记录、日志或 issue，应轮换。

## 许可证

本分支使用 Apache License 2.0。见 [LICENSE](LICENSE)。

本仓库包含来自 ngrok v1 的派生代码和第三方组件。见 [NOTICE](NOTICE) 和
[MODIFICATIONS.md](MODIFICATIONS.md)、[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
