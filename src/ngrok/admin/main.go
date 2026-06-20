package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultAddr        = "127.0.0.1:9090"
	defaultStatePath   = "/etc/ngrok/admin.json"
	defaultEnvPath     = "/etc/ngrok/ngrokd.env"
	defaultClientPath  = "/etc/ngrok/client.yml"
	defaultNginxPath   = "/etc/nginx/conf.d/ngrok.conf"
	defaultServiceName = "ngrokd"
	sessionCookie      = "ngrok_admin_session"
	passwordIterations = 210000
)

type options struct {
	addr        string
	statePath   string
	envPath     string
	clientPath  string
	nginxPath   string
	workDir     string
	certDir     string
	serviceName string
	timeout     time.Duration
}

type app struct {
	opts     options
	state    adminState
	setupKey string

	mu       sync.Mutex
	sessions map[string]time.Time
}

type adminState struct {
	Username     string `json:"username"`
	Salt         string `json:"salt"`
	PasswordHash string `json:"password_hash"`
	Iterations   int    `json:"iterations"`
	CreatedAt    string `json:"created_at"`
}

type serverConfig struct {
	Domain         string
	ControlHost    string
	TLSCrt         string
	TLSKey         string
	AuthToken      string
	HTTPAddr       string
	HTTPSAddr      string
	TunnelAddr     string
	LogLevel       string
	MaxConnections string
	ExtraArgs      string
	VHost          string
}

type certStatus struct {
	Path       string
	Subject    string
	Issuer     string
	NotAfter   string
	DNSNames   string
	DomainOK   string
	WildcardOK string
	Error      string
}

type serviceStatus struct {
	State string
	Error string
	Logs  string
}

type checkItem struct {
	Name   string
	State  string
	Detail string
}

type domainCertRow struct {
	Domain      string
	RootDNS     checkItem
	WildcardDNS checkItem
	CertState   string
	CertDetail  string
}

type binaryItem struct {
	Name  string
	Path  string
	State string
	Size  string
	URL   string
}

type setupStep struct {
	Key    string
	Title  string
	State  string
	Detail string
	URL    string
	Action string
}

type viewData struct {
	Title       string
	Page        string
	Lang        string
	Authed      bool
	Message     string
	Error       string
	Config      serverConfig
	Cert        certStatus
	Service     serviceStatus
	Checks      []checkItem
	CertRows    []domainCertRow
	Binaries    []binaryItem
	Steps       []setupStep
	NextStep    setupStep
	NginxConfig string
	BuildOutput string
	NginxPath   string
	WorkDir     string
	CertDir     string
	EnvPath     string
	Addr        string
	Now         string
}

func Main() {
	var opts options
	flag.StringVar(&opts.addr, "addr", defaultAddr, "admin listen address")
	flag.StringVar(&opts.statePath, "state", defaultStatePath, "admin state file")
	flag.StringVar(&opts.envPath, "env", defaultEnvPath, "ngrokd environment file")
	flag.StringVar(&opts.clientPath, "client-config", defaultClientPath, "client config output path")
	flag.StringVar(&opts.nginxPath, "nginx-conf", defaultNginxPath, "nginx config output path")
	flag.StringVar(&opts.workDir, "work-dir", "", "project work directory")
	flag.StringVar(&opts.certDir, "cert-dir", "", "certificate output directory")
	flag.StringVar(&opts.serviceName, "service", defaultServiceName, "systemd service name")
	flag.DurationVar(&opts.timeout, "timeout", 90*time.Second, "command timeout")
	flag.Parse()
	opts.applyDefaults()

	a, err := newApp(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ngrok-admin: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ngrok admin listening on http://%s\n", opts.addr)
	if !a.hasAdmin() {
		fmt.Printf("setup key: %s\n", a.setupKey)
	}

	srv := &http.Server{
		Addr:              opts.addr,
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "ngrok-admin: %v\n", err)
		os.Exit(1)
	}
}

func newApp(opts options) (*app, error) {
	state, err := loadAdminState(opts.statePath)
	if err != nil {
		return nil, err
	}
	setupKey := ""
	if state.Username == "" {
		setupKey, err = randomKey()
		if err != nil {
			return nil, err
		}
	}
	return &app{
		opts:     opts,
		state:    state,
		setupKey: setupKey,
		sessions: make(map[string]time.Time),
	}, nil
}

func (o *options) applyDefaults() {
	if o.workDir == "" {
		o.workDir = detectWorkDir()
	}
	if o.certDir == "" {
		o.certDir = filepath.Join(o.workDir, "certs")
	}
}

func detectWorkDir() string {
	if cwd, err := os.Getwd(); err == nil && fileExists(filepath.Join(cwd, "Makefile")) && fileExists(filepath.Join(cwd, "src", "ngrok")) {
		return cwd
	}
	for _, path := range []string{"/root/ngrok", "/opt/ngrok", "/usr/local/src/ngrok"} {
		if fileExists(filepath.Join(path, "Makefile")) && fileExists(filepath.Join(path, "src", "ngrok")) {
			return path
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleHome)
	mux.HandleFunc("/setup", a.handleSetup)
	mux.HandleFunc("/login", a.handleLogin)
	mux.HandleFunc("/logout", a.handleLogout)
	mux.HandleFunc("/config", a.handleConfig)
	mux.HandleFunc("/certificate", a.handleCertificate)
	mux.HandleFunc("/certificate/domain", a.handleCertificateDomain)
	mux.HandleFunc("/certificate/issue", a.handleCertificateIssue)
	mux.HandleFunc("/nginx", a.handleNginx)
	mux.HandleFunc("/build", a.handleBuild)
	mux.HandleFunc("/service", a.handleService)
	mux.HandleFunc("/download/client.yml", a.handleDownloadClientConfig)
	mux.HandleFunc("/download/ngrok", a.handleDownloadBinary("ngrok"))
	mux.HandleFunc("/static/style.css", handleStyle)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lang := normalizeLang(r.URL.Query().Get("lang")); lang != "" {
			http.SetCookie(w, &http.Cookie{
				Name:     "ngrok_admin_lang",
				Value:    lang,
				Path:     "/",
				SameSite: http.SameSiteLaxMode,
				Expires:  time.Now().Add(365 * 24 * time.Hour),
			})
			q := r.URL.Query()
			q.Del("lang")
			r.URL.RawQuery = q.Encode()
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *app) hasAdmin() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state.Username != ""
}

func (a *app) baseData(r *http.Request, title, page string) viewData {
	cfg := readServerConfig(a.opts.envPath)
	lang := requestLang(r)
	nginxPath := detectNginxPath(cfg, a.opts.nginxPath)
	return viewData{
		Title:     tr(lang, title),
		Page:      page,
		Lang:      lang,
		Authed:    a.currentUser(r) != "",
		Message:   messageText(lang, r.URL.Query().Get("msg")),
		Error:     r.URL.Query().Get("err"),
		Config:    cfg,
		NginxPath: nginxPath,
		WorkDir:   a.opts.workDir,
		CertDir:   a.opts.certDir,
		EnvPath:   a.opts.envPath,
		Addr:      a.opts.addr,
		Now:       time.Now().Format(time.RFC3339),
	}
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !a.hasAdmin() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if !a.requireAuth(w, r) {
		return
	}
	data := a.baseData(r, "Dashboard", "dashboard")
	data.Cert = readCertStatus(data.Config)
	data.Service = readServiceStatus(a.opts.serviceName)
	data.Checks = runChecks(data.Config, a.opts, data.NginxPath)
	data.Steps = setupSteps(data.Config, data.Cert, data.Service, a.opts, data.NginxPath)
	data.NextStep = nextStep(data.Steps)
	render(w, data)
}

func (a *app) handleSetup(w http.ResponseWriter, r *http.Request) {
	if a.hasAdmin() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.renderError(w, r, "Setup", "setup", err)
			return
		}
		if r.Form.Get("setup_key") != a.setupKey {
			a.renderError(w, r, "Setup", "setup", errors.New("invalid setup key"))
			return
		}
		username := strings.TrimSpace(r.Form.Get("username"))
		password := r.Form.Get("password")
		if username == "" || password == "" {
			a.renderError(w, r, "Setup", "setup", errors.New("username and password are required"))
			return
		}
		if len(password) < 10 {
			a.renderError(w, r, "Setup", "setup", errors.New("password is too short"))
			return
		}
		state, err := newAdminState(username, password)
		if err != nil {
			a.renderError(w, r, "Setup", "setup", err)
			return
		}
		if err := saveAdminState(a.opts.statePath, state); err != nil {
			a.renderError(w, r, "Setup", "setup", err)
			return
		}
		a.mu.Lock()
		a.state = state
		a.setupKey = ""
		a.mu.Unlock()
		a.setSession(w, username)
		http.Redirect(w, r, "/?msg=admin_ready", http.StatusFound)
		return
	}
	render(w, a.baseData(r, "Setup", "setup"))
}

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !a.hasAdmin() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.renderError(w, r, "Login", "login", err)
			return
		}
		username := strings.TrimSpace(r.Form.Get("username"))
		password := r.Form.Get("password")
		if a.checkPassword(username, password) {
			a.setSession(w, username)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		a.renderError(w, r, "Login", "login", errors.New("login failed"))
		return
	}
	render(w, a.baseData(r, "Login", "login"))
}

func (a *app) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (a *app) handleConfig(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.renderError(w, r, "Config", "config", err)
			return
		}
		existing := readServerConfig(a.opts.envPath)
		cfg := existing
		cfg.Domain = strings.TrimSpace(r.Form.Get("domain"))
		cfg.ControlHost = strings.TrimSpace(r.Form.Get("control_host"))
		cfg.AuthToken = strings.TrimSpace(r.Form.Get("auth_token"))
		cfg.HTTPAddr = strings.TrimSpace(r.Form.Get("http_addr"))
		cfg.TunnelAddr = strings.TrimSpace(r.Form.Get("tunnel_addr"))
		allDomains := configuredDomainList(a.opts.envPath, existing)
		if defaultLikeDomain(existing.Domain) {
			allDomains = nil
		}
		cfg.VHost = strings.Join(appendDomain(allDomains, cfg.Domain), ",")
		if r.Form.Get("new_token") == "1" || cfg.AuthToken == "" {
			token, err := randomToken()
			if err != nil {
				a.renderError(w, r, "Config", "config", err)
				return
			}
			cfg.AuthToken = token
		}
		cfg.applyDefaults()
		if err := cfg.validate(); err != nil {
			a.renderErrorWithConfig(w, r, "Config", "config", err, cfg)
			return
		}
		if err := writeServerConfig(a.opts.envPath, cfg); err != nil {
			a.renderErrorWithConfig(w, r, "Config", "config", err, cfg)
			return
		}
		http.Redirect(w, r, "/?msg=config_saved", http.StatusFound)
		return
	}
	render(w, a.baseData(r, "Config", "config"))
}

func (a *app) handleCertificate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	data := a.baseData(r, "Certificate", "certificate")
	data.Cert = readCertStatus(data.Config)
	domains := configuredDomainList(a.opts.envPath, data.Config)
	data.CertRows = domainCertRows(a.opts.certDir, domains, data.Config)
	render(w, data)
}

func (a *app) handleCertificateDomain(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/certificate", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Certificate", "certificate", err)
		return
	}
	domains := parseDomainInput(r.Form.Get("domain"))
	if len(domains) != 1 {
		a.renderError(w, r, "Certificate", "certificate", errors.New("domain is invalid"))
		return
	}
	cfg := readServerConfig(a.opts.envPath)
	cfg.applyDefaults()
	allDomains := configuredDomainList(a.opts.envPath, cfg)
	if defaultLikeDomain(cfg.Domain) {
		allDomains = nil
		cfg.Domain = domains[0]
		if defaultLikeControlHost(cfg.ControlHost) {
			cfg.ControlHost = "ngrok." + domains[0]
		}
		cfg.TLSCrt = filepath.Join(domainCertDir(a.opts.certDir, domains[0]), "fullchain.pem")
		cfg.TLSKey = filepath.Join(domainCertDir(a.opts.certDir, domains[0]), "privkey.pem")
	}
	allDomains = appendDomain(allDomains, domains[0])
	cfg.VHost = strings.Join(allDomains, ",")
	if err := writeServerConfig(a.opts.envPath, cfg); err != nil {
		a.renderError(w, r, "Certificate", "certificate", err)
		return
	}
	http.Redirect(w, r, "/certificate?msg=domain_saved", http.StatusFound)
}

func (a *app) handleCertificateIssue(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/certificate", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Certificate", "certificate", err)
		return
	}
	cfg := readServerConfig(a.opts.envPath)
	cfg.applyDefaults()
	domains := parseDomainInput(r.Form.Get("domain"))
	if len(domains) != 1 {
		a.renderError(w, r, "Certificate", "certificate", errors.New("domain is invalid"))
		return
	}
	domain := domains[0]
	if failed := firstFailedCheck(domainDNSChecks([]string{domain})); failed != nil {
		data := a.baseData(r, "Certificate", "certificate")
		data.Cert = readCertStatus(data.Config)
		data.CertRows = domainCertRows(a.opts.certDir, configuredDomainList(a.opts.envPath, data.Config), data.Config)
		data.Error = failed.Detail
		render(w, data)
		return
	}
	certDir := domainCertDir(a.opts.certDir, domain)
	crtPath := filepath.Join(certDir, "fullchain.pem")
	keyPath := filepath.Join(certDir, "privkey.pem")
	out, err := issueWithAcmeSH(cfg, domain, certDir, a.opts.timeout)
	if err == nil {
		allDomains := configuredDomainList(a.opts.envPath, cfg)
		if defaultLikeDomain(cfg.Domain) {
			allDomains = nil
		}
		allDomains = appendDomain(allDomains, domain)
		if defaultLikeDomain(cfg.Domain) || cfg.Domain == domain {
			cfg.Domain = domain
			if defaultLikeControlHost(cfg.ControlHost) {
				cfg.ControlHost = "ngrok." + domain
			}
			cfg.TLSCrt = crtPath
			cfg.TLSKey = keyPath
		} else {
			cfg.ExtraArgs = upsertDomainCertArg(cfg.ExtraArgs, domain, crtPath, keyPath)
		}
		cfg.VHost = strings.Join(allDomains, ",")
		if writeErr := writeServerConfig(a.opts.envPath, cfg); writeErr != nil {
			err = writeErr
		}
	}
	data := a.baseData(r, "Certificate", "certificate")
	data.Cert = readCertStatus(data.Config)
	data.CertRows = domainCertRows(a.opts.certDir, configuredDomainList(a.opts.envPath, data.Config), data.Config)
	data.Message = tail(out, 4000)
	if err != nil {
		data.Error = err.Error()
	}
	render(w, data)
}

func (a *app) handleNginx(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	cfg := readServerConfig(a.opts.envPath)
	cfg.applyDefaults()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.renderError(w, r, "Nginx", "nginx", err)
			return
		}
		path := strings.TrimSpace(r.Form.Get("path"))
		if path == "" {
			path = a.opts.nginxPath
		}
		action := r.Form.Get("action")
		var err error
		var msg string
		switch action {
		case "write":
			err = writeFileRoot(path, []byte(nginxConfig(cfg)), 0644)
			msg = "nginx config written"
		case "test":
			msg, err = runCommand(a.opts.timeout, nil, "nginx", "-t")
		case "reload":
			msg, err = runCommand(a.opts.timeout, nil, "systemctl", "reload", "nginx")
		default:
			err = errors.New("unknown action")
		}
		data := a.baseData(r, "Nginx", "nginx")
		data.NginxConfig = nginxConfig(cfg)
		data.NginxPath = path
		data.Message = tail(msg, 4000)
		if err != nil {
			data.Error = err.Error()
		}
		render(w, data)
		return
	}
	data := a.baseData(r, "Nginx", "nginx")
	data.NginxConfig = nginxConfig(cfg)
	render(w, data)
}

func (a *app) handleBuild(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	data := a.baseData(r, "Build", "build")
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.renderError(w, r, "Build", "build", err)
			return
		}
		target, err := buildTarget(r.Form.Get("target"))
		if err != nil {
			data.Error = err.Error()
			data.Binaries = binaryItems(a.opts.workDir)
			render(w, data)
			return
		}
		out, err := runCommandInDir(a.opts.timeout, nil, a.opts.workDir, "make", target)
		data.BuildOutput = tail(out, 8000)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Message = messageText(data.Lang, "build_done")
		}
	}
	data.Binaries = binaryItems(a.opts.workDir)
	render(w, data)
}

func (a *app) handleService(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.renderError(w, r, "Service", "service", err)
			return
		}
		action := r.Form.Get("action")
		var out string
		var err error
		switch action {
		case "start":
			out, err = runCommand(a.opts.timeout, nil, "systemctl", "enable", "--now", a.opts.serviceName)
		case "restart":
			out, err = runCommand(a.opts.timeout, nil, "systemctl", "restart", a.opts.serviceName)
		case "stop":
			out, err = runCommand(a.opts.timeout, nil, "systemctl", "stop", a.opts.serviceName)
		default:
			err = errors.New("unknown action")
		}
		data := a.baseData(r, "Service", "service")
		data.Service = readServiceStatus(a.opts.serviceName)
		data.Message = tail(out, 4000)
		if err != nil {
			data.Error = err.Error()
		}
		render(w, data)
		return
	}
	data := a.baseData(r, "Service", "service")
	data.Service = readServiceStatus(a.opts.serviceName)
	render(w, data)
}

func (a *app) handleDownloadClientConfig(w http.ResponseWriter, r *http.Request) {
	if !a.requireAuth(w, r) {
		return
	}
	cfg := readServerConfig(a.opts.envPath)
	cfg.applyDefaults()
	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="client.yml"`)
	_, _ = io.WriteString(w, clientConfig(cfg))
}

func (a *app) handleDownloadBinary(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.requireAuth(w, r) {
			return
		}
		path := filepath.Join(a.opts.workDir, "bin", name)
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		http.ServeFile(w, r, path)
	}
}

func buildTarget(target string) (string, error) {
	switch target {
	case "server":
		return "release-server", nil
	case "client":
		return "release-client", nil
	default:
		return "", errors.New("unsupported build target")
	}
}

func (a *app) renderError(w http.ResponseWriter, r *http.Request, title, page string, err error) {
	data := a.baseData(r, title, page)
	data.Error = err.Error()
	render(w, data)
}

func (a *app) renderErrorWithConfig(w http.ResponseWriter, r *http.Request, title, page string, err error, cfg serverConfig) {
	data := a.baseData(r, title, page)
	data.Config = cfg
	data.Error = err.Error()
	render(w, data)
}

func (a *app) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if !a.hasAdmin() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return false
	}
	if a.currentUser(r) == "" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return false
	}
	return true
}

func (a *app) currentUser(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[cookie.Value]
	if !ok || time.Now().After(exp) {
		delete(a.sessions, cookie.Value)
		return ""
	}
	return a.state.Username
}

func (a *app) setSession(w http.ResponseWriter, username string) {
	id, err := randomSessionID()
	if err != nil {
		return
	}
	a.mu.Lock()
	a.sessions[id] = time.Now().Add(12 * time.Hour)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(12 * time.Hour),
	})
}

func (a *app) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (a *app) checkPassword(username, password string) bool {
	a.mu.Lock()
	state := a.state
	a.mu.Unlock()
	if username != state.Username || state.Username == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(state.Salt)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(state.PasswordHash)
	if err != nil {
		return false
	}
	got := pbkdf2SHA256([]byte(password), salt, state.Iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func loadAdminState(path string) (adminState, error) {
	var state adminState
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return state, nil
	}
	err = json.Unmarshal(b, &state)
	return state, err
}

func newAdminState(username, password string) (adminState, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return adminState{}, err
	}
	hash := pbkdf2SHA256([]byte(password), salt, passwordIterations, 32)
	return adminState{
		Username:     username,
		Salt:         base64.StdEncoding.EncodeToString(salt),
		PasswordHash: base64.StdEncoding.EncodeToString(hash),
		Iterations:   passwordIterations,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

func saveAdminState(path string, state adminState) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileRoot(path, append(b, '\n'), 0600)
}

func writeFileRoot(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	uid, gid := fileOwner(path)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	if uid >= 0 || gid >= 0 {
		if err := os.Chown(tmp, uid, gid); err != nil && os.Geteuid() == 0 {
			return err
		}
	}
	return os.Rename(tmp, path)
}

func fileOwner(path string) (int, int) {
	if st, err := os.Stat(path); err == nil {
		if sys, ok := st.Sys().(*syscall.Stat_t); ok {
			return int(sys.Uid), int(sys.Gid)
		}
	}
	if st, err := os.Stat(filepath.Dir(path)); err == nil {
		if sys, ok := st.Sys().(*syscall.Stat_t); ok {
			return os.Geteuid(), int(sys.Gid)
		}
	}
	return -1, -1
}

func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	hLen := 32
	numBlocks := (keyLen + hLen - 1) / hLen
	var out []byte
	for block := 1; block <= numBlocks; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iter; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

func randomKey() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hexed := hex.EncodeToString(b)
	return hexed[0:4] + "-" + hexed[4:8] + "-" + hexed[8:12] + "-" + hexed[12:], nil
}

func randomSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func readServerConfig(path string) serverConfig {
	cfg := defaultServerConfig()
	values := readEnvFile(path)
	cfg.Domain = first(values["NGROKD_DOMAIN"], cfg.Domain)
	cfg.ControlHost = first(values["NGROKD_CONTROL_HOST"], cfg.ControlHost)
	cfg.TLSCrt = first(values["NGROKD_TLS_CRT"], cfg.TLSCrt)
	cfg.TLSKey = first(values["NGROKD_TLS_KEY"], cfg.TLSKey)
	cfg.AuthToken = first(values["NGROKD_AUTH_TOKEN"], cfg.AuthToken)
	cfg.HTTPAddr = first(values["NGROKD_HTTP_ADDR"], cfg.HTTPAddr)
	cfg.HTTPSAddr = first(values["NGROKD_HTTPS_ADDR"], cfg.HTTPSAddr)
	cfg.TunnelAddr = first(values["NGROKD_TUNNEL_ADDR"], cfg.TunnelAddr)
	cfg.LogLevel = first(values["NGROKD_LOG_LEVEL"], cfg.LogLevel)
	cfg.MaxConnections = first(values["NGROKD_MAX_CONNECTIONS"], cfg.MaxConnections)
	cfg.ExtraArgs = first(values["NGROKD_EXTRA_ARGS"], cfg.ExtraArgs)
	cfg.VHost = values["VHOST"]
	cfg.applyDefaults()
	return cfg
}

func defaultServerConfig() serverConfig {
	return serverConfig{
		Domain:         "example.com",
		ControlHost:    "ngrok.example.com",
		TLSCrt:         "/etc/ngrok/tls/fullchain.pem",
		TLSKey:         "/etc/ngrok/tls/privkey.pem",
		HTTPAddr:       "127.0.0.1:8080",
		HTTPSAddr:      "",
		TunnelAddr:     "0.0.0.0:4443",
		LogLevel:       "INFO",
		MaxConnections: "1024",
	}
}

func (c *serverConfig) applyDefaults() {
	if c.Domain == "" {
		c.Domain = "example.com"
	}
	if c.ControlHost == "" || strings.HasPrefix(c.ControlHost, "ngrok.example.") {
		c.ControlHost = "ngrok." + c.Domain
	}
	if c.TLSCrt == "" {
		c.TLSCrt = "/etc/ngrok/tls/fullchain.pem"
	}
	if c.TLSKey == "" {
		c.TLSKey = "/etc/ngrok/tls/privkey.pem"
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = "127.0.0.1:8080"
	}
	if c.TunnelAddr == "" {
		c.TunnelAddr = "0.0.0.0:4443"
	}
	if c.LogLevel == "" {
		c.LogLevel = "INFO"
	}
	if c.MaxConnections == "" {
		c.MaxConnections = "1024"
	}
}

func (c serverConfig) validate() error {
	if c.Domain == "" {
		return errors.New("domain is required")
	}
	if strings.ContainsAny(c.Domain, " /") {
		return errors.New("domain is invalid")
	}
	if c.ControlHost == "" {
		return errors.New("control host is required")
	}
	if c.TLSCrt == "" || c.TLSKey == "" {
		return errors.New("certificate and key paths are required")
	}
	if c.AuthToken == "" || c.AuthToken == "change-me-long-random-token" {
		return errors.New("auth token is required")
	}
	if _, err := strconv.Atoi(c.MaxConnections); err != nil {
		return errors.New("max connections must be a number")
	}
	return nil
}

func writeServerConfig(path string, cfg serverConfig) error {
	cfg.applyDefaults()
	var b strings.Builder
	b.WriteString("# Managed by ngrok-admin.\n")
	writeEnv(&b, "NGROKD_DOMAIN", cfg.Domain)
	writeEnv(&b, "NGROKD_CONTROL_HOST", cfg.ControlHost)
	writeEnv(&b, "NGROKD_TLS_CRT", cfg.TLSCrt)
	writeEnv(&b, "NGROKD_TLS_KEY", cfg.TLSKey)
	writeEnv(&b, "NGROKD_AUTH_TOKEN", cfg.AuthToken)
	writeEnv(&b, "NGROKD_HTTP_ADDR", cfg.HTTPAddr)
	writeEnv(&b, "NGROKD_HTTPS_ADDR", cfg.HTTPSAddr)
	writeEnv(&b, "NGROKD_TUNNEL_ADDR", cfg.TunnelAddr)
	writeEnv(&b, "NGROKD_LOG_LEVEL", cfg.LogLevel)
	writeEnv(&b, "NGROKD_MAX_CONNECTIONS", cfg.MaxConnections)
	writeEnv(&b, "NGROKD_EXTRA_ARGS", cfg.ExtraArgs)
	if cfg.VHost != "" {
		writeEnv(&b, "VHOST", cfg.VHost)
	}
	return writeFileRoot(path, []byte(b.String()), 0640)
}

func readEnvFile(path string) map[string]string {
	values := make(map[string]string)
	b, err := os.ReadFile(path)
	if err != nil {
		return values
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		values[key] = value
	}
	return values
}

func writeEnv(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteByte('=')
	b.WriteString(envValue(value))
	b.WriteByte('\n')
}

func envValue(value string) string {
	if value == "" {
		return ""
	}
	for _, r := range value {
		if !(r == '/' || r == ':' || r == '.' || r == '-' || r == '_' || r == ',' || r == '=' || r == '*' || r == '@' || r == '+' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return strconv.Quote(value)
		}
	}
	return value
}

func readCertStatus(cfg serverConfig) certStatus {
	cfg.applyDefaults()
	status := certStatus{Path: cfg.TLSCrt}
	b, err := os.ReadFile(cfg.TLSCrt)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	block, _ := pem.Decode(b)
	if block == nil {
		status.Error = "no PEM certificate found"
		return status
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Subject = cert.Subject.CommonName
	status.Issuer = cert.Issuer.CommonName
	status.NotAfter = cert.NotAfter.Format("2006-01-02 15:04 MST")
	status.DNSNames = strings.Join(cert.DNSNames, ", ")
	status.DomainOK = boolState(cert.VerifyHostname(cfg.Domain) == nil)
	status.WildcardOK = boolState(cert.VerifyHostname("test."+cfg.Domain) == nil)
	return status
}

func readServiceStatus(service string) serviceStatus {
	out, err := exec.Command("systemctl", "is-active", service).CombinedOutput()
	status := serviceStatus{State: strings.TrimSpace(string(out))}
	if err != nil {
		if status.State == "" {
			status.State = "unavailable"
		}
		status.Error = err.Error()
	}
	logs, logErr := exec.Command("journalctl", "-u", service, "-n", "80", "--no-pager").CombinedOutput()
	if logErr == nil {
		status.Logs = string(logs)
	}
	return status
}

func runChecks(cfg serverConfig, opts options, nginxPath string) []checkItem {
	cfg.applyDefaults()
	var checks []checkItem
	checks = append(checks, pathCheck("env", opts.envPath))
	checks = append(checks, pathCheck("certificate", cfg.TLSCrt))
	checks = append(checks, pathCheck("private key", cfg.TLSKey))
	checks = append(checks, dnsCheck("control DNS", cfg.ControlHost))
	checks = append(checks, dnsCheck("wildcard DNS", "probe-"+strconv.FormatInt(time.Now().Unix(), 10)+"."+cfg.Domain))
	checks = append(checks, tcpCheck("tunnel port", cfg.ControlHost, tunnelPort(cfg.TunnelAddr)))
	checks = append(checks, pathCheck("nginx config", nginxPath))
	return checks
}

func setupSteps(cfg serverConfig, cert certStatus, service serviceStatus, opts options, nginxPath string) []setupStep {
	cfg.applyDefaults()
	configState := "done"
	configDetail := cfg.Domain + " / " + cfg.ControlHost
	if _, err := os.Stat(opts.envPath); err != nil || cfg.AuthToken == "" || cfg.AuthToken == "change-me-long-random-token" {
		configState = "todo"
		configDetail = opts.envPath
	}

	certState := "done"
	certDetail := cert.NotAfter
	if cert.Error != "" || cert.DomainOK != "ok" || cert.WildcardOK != "ok" {
		certState = "todo"
		if cert.Error != "" {
			certDetail = cert.Error
		} else {
			certDetail = cert.Path
		}
	}

	nginxState := "done"
	nginxDetail := nginxPath
	if _, err := os.Stat(nginxPath); err != nil {
		nginxState = "todo"
	}

	buildState := "done"
	buildDetail := filepath.Join(opts.workDir, "bin")
	for _, item := range binaryItems(opts.workDir) {
		if item.State != "ok" {
			buildState = "todo"
			buildDetail = item.Path
			break
		}
	}

	serviceState := "done"
	serviceDetail := service.State
	if service.State != "active" {
		serviceState = "todo"
	}

	return []setupStep{
		{Key: "step_config", Title: "Config", State: configState, Detail: configDetail, URL: "/config", Action: "Open"},
		{Key: "step_certificate", Title: "Certificate", State: certState, Detail: certDetail, URL: "/certificate", Action: "Open"},
		{Key: "step_nginx", Title: "Nginx", State: nginxState, Detail: nginxDetail, URL: "/nginx", Action: "Open"},
		{Key: "step_build", Title: "Build", State: buildState, Detail: buildDetail, URL: "/build", Action: "Open"},
		{Key: "step_service", Title: "Service", State: serviceState, Detail: serviceDetail, URL: "/service", Action: "Open"},
	}
}

func nextStep(steps []setupStep) setupStep {
	for _, step := range steps {
		if step.State != "done" {
			return step
		}
	}
	return setupStep{Key: "ready", Title: "Ready", State: "done", Detail: "", URL: "/build", Action: "Open"}
}

func pathCheck(name, path string) checkItem {
	if _, err := os.Stat(path); err != nil {
		return checkItem{Name: name, State: "missing", Detail: path}
	}
	return checkItem{Name: name, State: "ok", Detail: path}
}

func detectNginxPath(cfg serverConfig, fallback string) string {
	cfg.applyDefaults()
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	patterns := []string{
		"/etc/nginx/conf.d/*.conf",
		"/etc/nginx/sites-enabled/*",
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			text := string(b)
			if strings.Contains(text, cfg.HTTPAddr) || (strings.Contains(strings.ToLower(text), "ngrok") && strings.Contains(text, "proxy_pass")) {
				return path
			}
		}
	}
	return fallback
}

func configuredDomainList(envPath string, cfg serverConfig) []string {
	cfg.applyDefaults()
	seen := make(map[string]bool)
	var result []string
	add := func(domain string) {
		domain = normalizeDomain(domain)
		if domain == "" || seen[domain] || !validDomainName(domain) {
			return
		}
		seen[domain] = true
		result = append(result, domain)
	}
	add(cfg.Domain)
	vhost := cfg.VHost
	if vhost == "" {
		values := readEnvFile(envPath)
		vhost = values["VHOST"]
	}
	for _, item := range strings.Split(vhost, ",") {
		add(item)
	}
	if len(result) == 0 {
		add("example.com")
	}
	return result
}

func parseDomainInput(input string) []string {
	seen := make(map[string]bool)
	var result []string
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	for _, field := range fields {
		domain := normalizeDomain(field)
		if domain == "" || seen[domain] || !validDomainName(domain) {
			continue
		}
		seen[domain] = true
		result = append(result, domain)
	}
	return result
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimSuffix(domain, ".")
	return stripPort(domain)
}

func validDomainName(domain string) bool {
	if len(domain) < 3 || len(domain) > 253 || strings.Contains(domain, "..") {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func domainDNSChecks(domains []string) []checkItem {
	var checks []checkItem
	for _, domain := range domains {
		checks = append(checks, dnsCheck(domain, domain))
		checks = append(checks, dnsCheck("*."+domain, "probe-"+strconv.FormatInt(time.Now().Unix(), 10)+"."+domain))
	}
	return checks
}

func domainCertRows(certDir string, domains []string, cfg serverConfig) []domainCertRow {
	rows := make([]domainCertRow, 0, len(domains))
	for _, domain := range domains {
		root := dnsCheck(domain, domain)
		wildcard := dnsCheck("*."+domain, "probe-"+strconv.FormatInt(time.Now().Unix(), 10)+"."+domain)
		crtPath, _ := certPathsForDomain(certDir, domain, cfg)
		certState, certDetail := certStateForDomain(domain, crtPath)
		rows = append(rows, domainCertRow{
			Domain:      domain,
			RootDNS:     root,
			WildcardDNS: wildcard,
			CertState:   certState,
			CertDetail:  certDetail,
		})
	}
	return rows
}

func firstFailedCheck(checks []checkItem) *checkItem {
	for i := range checks {
		if checks[i].State != "ok" {
			return &checks[i]
		}
	}
	return nil
}

func dnsCheck(name, host string) checkItem {
	addrs, err := net.LookupHost(host)
	if err != nil {
		return checkItem{Name: name, State: "fail", Detail: err.Error()}
	}
	return checkItem{Name: name, State: "ok", Detail: strings.Join(addrs, ", ")}
}

func appendDomain(domains []string, domain string) []string {
	domain = normalizeDomain(domain)
	if domain == "" || !validDomainName(domain) {
		return domains
	}
	for _, item := range domains {
		if item == domain {
			return domains
		}
	}
	return append(domains, domain)
}

func domainCertDir(baseDir, domain string) string {
	return filepath.Join(baseDir, normalizeDomain(domain))
}

func certPathsForDomain(certDir, domain string, cfg serverConfig) (string, string) {
	cfg.applyDefaults()
	domain = normalizeDomain(domain)
	if domain == normalizeDomain(cfg.Domain) {
		return cfg.TLSCrt, cfg.TLSKey
	}
	if crt, key, ok := domainCertArgPaths(cfg.ExtraArgs, domain); ok {
		return crt, key
	}
	base := domainCertDir(certDir, domain)
	return filepath.Join(base, "fullchain.pem"), filepath.Join(base, "privkey.pem")
}

func certStateForDomain(domain, certPath string) (string, string) {
	if certPath == "" {
		return "missing", ""
	}
	b, err := os.ReadFile(certPath)
	if err != nil {
		return "missing", certPath
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "fail", "no PEM certificate found"
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "fail", err.Error()
	}
	if time.Now().After(cert.NotAfter) {
		return "fail", cert.NotAfter.Format("2006-01-02 15:04 MST")
	}
	if err := cert.VerifyHostname(domain); err != nil {
		return "fail", err.Error()
	}
	if err := cert.VerifyHostname("test." + domain); err != nil {
		return "fail", err.Error()
	}
	return "ok", cert.NotAfter.Format("2006-01-02 15:04 MST")
}

func binaryItems(workDir string) []binaryItem {
	names := []string{"ngrokd", "ngrok"}
	var items []binaryItem
	for _, name := range names {
		path := filepath.Join(workDir, "bin", name)
		item := binaryItem{Name: name, Path: path, State: "missing"}
		if name == "ngrok" {
			item.URL = "/download/ngrok"
		}
		if st, err := os.Stat(path); err == nil {
			item.State = "ok"
			item.Size = fmt.Sprintf("%.1f MB", float64(st.Size())/1024/1024)
		}
		items = append(items, item)
	}
	return items
}

func runCommandInDir(timeout time.Duration, extraEnv []string, dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	timer := time.AfterFunc(timeout, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	out, err := cmd.CombinedOutput()
	timer.Stop()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w", strings.Join(append([]string{name}, args...), " "), err)
	}
	return string(out), nil
}

func tcpCheck(name, host, port string) checkItem {
	target := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", target, 2*time.Second)
	if err != nil {
		return checkItem{Name: name, State: "fail", Detail: err.Error()}
	}
	_ = conn.Close()
	return checkItem{Name: name, State: "ok", Detail: target}
}

func issueWithAcmeSH(cfg serverConfig, domain, certDir string, timeout time.Duration) (string, error) {
	cfg.applyDefaults()
	domain = normalizeDomain(domain)
	if domain == "" || !validDomainName(domain) {
		return "", errors.New("domain is invalid")
	}
	dnsPlugin := firstNonEmpty(os.Getenv("NGROK_ADMIN_DNS_PLUGIN"), os.Getenv("ACME_DNS_PLUGIN"), "dns_cf")
	if dnsPlugin == "" {
		return "", errors.New("dns plugin is required")
	}
	bin, err := findAcmeSH()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(certDir, 0750); err != nil {
		return "", err
	}
	accountEmail := strings.TrimSpace(os.Getenv("ACME_ACCOUNT_EMAIL"))
	args := []string{"--issue", "--dns", dnsPlugin, "-d", domain, "-d", "*." + domain}
	if accountEmail != "" {
		args = append(args, "--accountemail", accountEmail)
	}
	out1, err := runCommand(timeout, nil, bin, args...)
	if err != nil {
		return out1, err
	}
	out2, err := runCommand(timeout, nil, bin,
		"--install-cert",
		"-d", domain,
		"--fullchain-file", filepath.Join(certDir, "fullchain.pem"),
		"--key-file", filepath.Join(certDir, "privkey.pem"),
		"--reloadcmd", "systemctl reload nginx || true",
	)
	return out1 + "\n" + out2, err
}

func findAcmeSH() (string, error) {
	if bin, err := exec.LookPath("acme.sh"); err == nil {
		return bin, nil
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".acme.sh", "acme.sh"),
		"/root/.acme.sh/acme.sh",
		"/usr/local/bin/acme.sh",
	}
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("acme.sh not found")
}

func domainCertArgPaths(extraArgs, domain string) (string, string, bool) {
	prefix := domain + ":"
	fields := strings.Fields(extraArgs)
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if field == "-domainCert" && i+1 < len(fields) {
			if crt, key, ok := parseDomainCertValue(fields[i+1], prefix); ok {
				return crt, key, true
			}
			i++
			continue
		}
		if strings.HasPrefix(field, "-domainCert=") {
			if crt, key, ok := parseDomainCertValue(strings.TrimPrefix(field, "-domainCert="), prefix); ok {
				return crt, key, true
			}
		}
	}
	return "", "", false
}

func parseDomainCertValue(value, prefix string) (string, string, bool) {
	if !strings.HasPrefix(value, prefix) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(value, prefix), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func upsertDomainCertArg(extraArgs, domain, crtPath, keyPath string) string {
	prefix := domain + ":"
	fields := strings.Fields(extraArgs)
	filtered := make([]string, 0, len(fields)+1)
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if field == "-domainCert" && i+1 < len(fields) {
			if strings.HasPrefix(fields[i+1], prefix) {
				i++
				continue
			}
			filtered = append(filtered, field)
			continue
		}
		if strings.HasPrefix(field, "-domainCert="+prefix) {
			continue
		}
		filtered = append(filtered, field)
	}
	filtered = append(filtered, "-domainCert="+domain+":"+crtPath+":"+keyPath)
	return strings.Join(filtered, " ")
}

func runCommand(timeout time.Duration, extraEnv []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	timer := time.AfterFunc(timeout, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	out, err := cmd.CombinedOutput()
	timer.Stop()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w", strings.Join(append([]string{name}, args...), " "), err)
	}
	return string(out), nil
}

func nginxConfig(cfg serverConfig) string {
	cfg.applyDefaults()
	return fmt.Sprintf(`map $http_upgrade $connection_upgrade {
    default upgrade;
    "" close;
}

server {
    listen 80;
    server_name *.%s;

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
        proxy_pass http://%s;
    }
}

server {
    listen 443 ssl;
    http2 on;
    server_name *.%s;

    ssl_certificate %s;
    ssl_certificate_key %s;

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
        proxy_pass http://%s;
    }
}
`, cfg.Domain, cfg.HTTPAddr, cfg.Domain, cfg.TLSCrt, cfg.TLSKey, cfg.HTTPAddr)
}

func clientConfig(cfg serverConfig) string {
	cfg.applyDefaults()
	return fmt.Sprintf(`server_addr: %s:%s
auth_token: %s
trust_host_root_certs: true
inspect_addr: 127.0.0.1:4040

tunnels:
  app:
    proto:
      http: 127.0.0.1:3000
    subdomain: app
`, cfg.ControlHost, tunnelPort(cfg.TunnelAddr), cfg.AuthToken)
}

func tunnelPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "4443"
	}
	return port
}

func render(w http.ResponseWriter, data viewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = io.WriteString(w, styleCSS)
}

func boolState(ok bool) string {
	if ok {
		return "ok"
	}
	return "fail"
}

func normalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "zh", "zh-cn", "cn":
		return "zh-CN"
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return ""
	}
}

func requestLang(r *http.Request) string {
	if cookie, err := r.Cookie("ngrok_admin_lang"); err == nil {
		if lang := normalizeLang(cookie.Value); lang != "" {
			return lang
		}
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Accept-Language")), "zh") {
		return "zh-CN"
	}
	return "en"
}

func tr(lang, key string) string {
	if lang == "" {
		lang = "en"
	}
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if v, ok := translations["en"][key]; ok {
		return v
	}
	return key
}

func stateText(lang, state string) string {
	switch state {
	case "done", "ok", "active":
		return tr(lang, "state_ok")
	case "todo", "missing", "fail", "failed", "inactive", "unavailable":
		return tr(lang, "state_todo")
	case "running":
		return tr(lang, "state_running")
	default:
		if state == "" {
			return tr(lang, "state_unknown")
		}
		return state
	}
}

func messageText(lang, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if strings.ContainsAny(code, " \t\n") {
		return code
	}
	return tr(lang, "msg_"+code)
}

func first(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultLikeControlHost(host string) bool {
	return host == "" || strings.HasPrefix(host, "ngrok.example.")
}

func defaultLikeDomain(domain string) bool {
	return domain == "" || domain == "example.com"
}

func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

var pageTemplate = template.Must(template.New("admin").Funcs(template.FuncMap{
	"port":  tunnelPort,
	"tr":    tr,
	"state": stateText,
}).Parse(pageHTML))
