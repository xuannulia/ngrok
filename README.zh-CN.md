# ngrok v1 自托管分支

[English README](README.md)

这是一个基于 ngrok v1 的自托管维护分支。原版 ngrok v1 已经归档，这个仓库保留它简单直接的使用方式，并补上现在放到公网时通常需要的东西：客户端认证、TLS 下限、连接限制、多域名、证书热更新，以及常见的服务化部署模板。

它更适合个人、团队或私有基础设施自用。如果你想搭一个可控的内网穿透服务，这个分支可以直接作为起点；如果你要做面向外部用户售卖的商业隧道平台，它不是为那个场景设计的。

## 这个分支改了什么

- 服务端和客户端都必须配置认证 token。
- 服务端 TLS 监听器最低使用 TLS 1.2。
- ngrok 协议消息有大小上限，避免异常消息占用过多资源。
- `-maxConnections` 可以限制公网和隧道连接数。
- 默认不允许客户端指定公网 TCP 端口。
- HTTP inspection 最多保留 1 MB 的请求/响应体。
- inspection UI 如果绑定到非本机地址，必须配置访问认证。
- 支持多个基础域名。
- 可以重复传入 `-domainCert`，为不同域名配置 SNI 证书。
- 证书文件续期后，服务端可以在新 TLS 握手时加载新证书。
- 提供 `systemd`、`launchd`、Nginx 配置模板。
- 移除或替换了一批过时依赖。

## 相关文档

- [迁移说明](MIGRATION.md)
- [安全策略](SECURITY.md)
- [自托管参考](docs/SELFHOSTING.md)
- [第三方组件说明](THIRD_PARTY_NOTICES.md)

## 构建

项目仍然沿用原来的 GOPATH 目录结构，直接使用 Makefile 构建即可：

```bash
make all
make release-all
```

构建完成后会生成：

```text
bin/ngrok
bin/ngrokd
```

## Web 面板

`ngrok-admin` 用于首次配置：

```bash
sudo ./bin/ngrok-admin
```

默认监听 `127.0.0.1:9090`，启动后会在终端输出 setup key。用这个 key
创建管理员账号。

面板可以写入 `ngrokd.env`、生成客户端配置、通过 `acme.sh` 申请 DNS-01
证书、写入 Nginx 配置，并启动或重启 `ngrokd` 服务。

## 最小部署流程

下面是一套最小但接近实际生产环境的部署方式：Nginx 负责 80/443，`ngrokd` 负责客户端控制连接和 HTTP 隧道入口。

1. 先准备 DNS 记录：

   ```text
   ngrok.example.com A/AAAA <server-ip>
   *.example.com     A/AAAA <server-ip>
   ```

2. 通过 DNS-01 签发公开 CA 泛域名证书，证书至少覆盖：

   ```text
   example.com
   *.example.com
   ```

3. 在服务器上构建并安装服务：

   ```bash
   make release-all
   sudo sh deploy/install-systemd.sh
   ```

4. 编辑 `/etc/ngrok/ngrokd.env`：

   ```text
   NGROKD_DOMAIN=example.com
   NGROKD_CONTROL_HOST=ngrok.example.com
   NGROKD_TLS_CRT=/etc/ngrok/tls/fullchain.pem
   NGROKD_TLS_KEY=/etc/ngrok/tls/privkey.pem
   NGROKD_AUTH_TOKEN=<long-random-token>
   NGROKD_HTTP_ADDR=127.0.0.1:8080
   NGROKD_HTTPS_ADDR=
   NGROKD_TUNNEL_ADDR=0.0.0.0:4443
   ```

5. 配置 Nginx，把 `*.example.com` 转发到 `127.0.0.1:8080`。
   可以使用本文后面的 Nginx 示例，也可以直接参考 `deploy/nginx/ngrok-http.conf`。

6. 启动服务端：

   ```bash
   sudo systemctl enable --now ngrokd
   sudo systemctl status ngrokd
   ```

7. 准备客户端配置：

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

访问地址应当是：

```text
http://app.example.com
https://app.example.com
```

## 服务端

最小启动命令：

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/privkey.pem \
  -authToken=change-me-long-random-token
```

也可以把 token 放到环境变量里：

```bash
NGROK_AUTH_TOKEN=change-me-long-random-token ./bin/ngrokd -domain=example.com
```

不要在公网使用空 token、弱 token，也不要复用已经发给他人的 token。

常用参数：

```text
-domain              主域名。使用 subdomain 时，地址会基于这个域名生成。
-domainCert          额外域名的证书映射，可重复传入。
-tlsCrt              默认 TLS 证书文件。
-tlsKey              默认 TLS 私钥文件。
-httpAddr            HTTP 隧道入口。设为空值表示禁用。
-httpsAddr           HTTPS 隧道入口。设为空值表示禁用。
-tunnelAddr          客户端控制连接和代理连接入口。
-authToken           客户端必须使用的认证 token。
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

启动所有已配置的隧道：

```bash
./bin/ngrok -config=client.yml start-all
```

`subdomain: app` 只会使用服务端主 `-domain`：

```text
app.example.com
```

如果要使用其它已经配置到服务端的域名，请写完整的 `hostname`：

```yaml
tunnels:
  app-example-net:
    proto:
      http: 127.0.0.1:3000
    hostname: app.example.net
```

服务端会拒绝不属于已配置域名后缀的 `hostname`。

## 多域名

一个 `ngrokd` 可以同时服务多个基础域名：

```bash
./bin/ngrokd \
  -domain=example.com \
  -tlsCrt=/etc/ngrok/tls/example/fullchain.pem \
  -tlsKey=/etc/ngrok/tls/example/privkey.pem \
  -domainCert=example.net:/etc/ngrok/tls/example.net/fullchain.pem:/etc/ngrok/tls/example.net/privkey.pem \
  -domainCert=example.org:/etc/ngrok/tls/example.org/fullchain.pem:/etc/ngrok/tls/example.org/privkey.pem \
  -authToken=change-me-long-random-token
```

DNS 至少需要这些记录：

```text
*.example.com     A/AAAA <server-ip>
*.example.net     A/AAAA <server-ip>
*.example.org     A/AAAA <server-ip>
ngrok.example.com A/AAAA <server-ip>
```

是否解析根域名取决于你是否要把根域名本身也作为隧道地址使用。

## 证书

能用公开 CA 证书时，优先使用公开 CA。客户端配置：

```yaml
trust_host_root_certs: true
```

这样公开 CA 证书正常续期时，客户端不需要重新编译。

### DNS-01 泛域名证书

泛域名证书需要 DNS-01 验证。DNS 服务商没有特殊要求，只要你使用的 ACME 客户端可以通过 API 修改 DNS 记录即可。

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

续期后，把证书和私钥安装到服务用户可读取的位置，并设置合适的文件权限。只有浏览器 HTTPS 由 Nginx 终止时，才需要重载 Nginx。

如果 `ngrokd` 负责 `-tunnelAddr` 或 `-httpsAddr` 的 TLS，一般不需要因为续期而重启。它会继续使用内存中的当前证书，并在证书接近过期时检查证书/私钥文件是否更新；检查成功后，新的 TLS 握手会使用新证书，已有连接不受影响。

如果浏览器 HTTPS 由 Nginx 终止，证书安装完成后重载 Nginx。`ngrokd` 的证书热更新仍然适用于客户端控制通道。

## 与 Nginx 并用

如果服务器上已经有 Nginx 占用 80/443，不要让 `ngrokd` 直接监听这两个端口。通常有两种做法。

### 方案 A：Nginx 终止 HTTPS

这是最常见的方式：Nginx 统一处理浏览器 HTTPS，然后把请求转发到 `ngrokd` 的 HTTP 入口。

`ngrokd`：

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

如果你希望 Nginx 管理所有公网 Web TLS，并且不让 `ngrokd` 接触 443，选这个方案。

### 方案 B：HTTPS 透传给 ngrokd

如果你希望 `ngrokd` 自己终止 HTTPS 隧道流量，并通过 SNI 选择证书，可以让 Nginx 只做 TCP 透传。

`ngrokd`：

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

`default` 后端应指向你原本的 HTTPS 服务，避免隧道域名影响其它站点。

可参考的模板：

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

查看日志：

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

假设要部署成下面这样：

```text
主域名：        example.com
额外域名：      example.net, example.org
控制服务：      ngrok.example.com:4443
隧道域名：      app.example.com, api.example.net, admin.example.org
Nginx 占用：    80 和 443
ngrokd HTTP：   127.0.0.1:8080
ngrokd 控制端： 0.0.0.0:4443
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

最终可以访问：

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
- inspection UI 尽量只绑定 `127.0.0.1`；如果必须暴露到其它地址，一定要配置 `inspect_auth`。
- 不要给不可信客户端开启 `-allowRemotePorts`。
- 公网入口前面仍然应该配置防火墙和限流。
- DNS API 密钥如果出现在聊天记录、日志或 issue 里，应立即轮换。

## 许可证

本分支使用 Apache License 2.0。见 [LICENSE](LICENSE)。

本仓库包含来自 ngrok v1 的派生代码和第三方组件。见 [NOTICE](NOTICE)、
[MODIFICATIONS.md](MODIFICATIONS.md) 和 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
