package client

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"ngrok/log"
	"os"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Configuration struct {
	HttpProxy          string                          `yaml:"http_proxy,omitempty"`
	ServerAddr         string                          `yaml:"server_addr,omitempty"`
	InspectAddr        string                          `yaml:"inspect_addr,omitempty"`
	InspectAuth        string                          `yaml:"inspect_auth,omitempty"`
	TrustHostRootCerts bool                            `yaml:"trust_host_root_certs,omitempty"`
	AuthToken          string                          `yaml:"auth_token,omitempty"`
	Tunnels            map[string]*TunnelConfiguration `yaml:"tunnels,omitempty"`
	LogTo              string                          `yaml:"-"`
	Path               string                          `yaml:"-"`
}

type TunnelConfiguration struct {
	Subdomain  string            `yaml:"subdomain,omitempty"`
	Hostname   string            `yaml:"hostname,omitempty"`
	Protocols  map[string]string `yaml:"proto,omitempty"`
	HttpAuth   string            `yaml:"auth,omitempty"`
	RemotePort uint16            `yaml:"remote_port,omitempty"`
}

func LoadConfiguration(opts *Options) (config *Configuration, err error) {
	configPath := opts.config
	if configPath == "" {
		configPath = defaultPath()
	}

	log.Info("Reading configuration file %s", configPath)
	configBuf, err := os.ReadFile(configPath)
	if err != nil {
		// failure to read a configuration file is only a fatal error if
		// the user specified one explicitly
		if opts.config != "" {
			err = fmt.Errorf("Failed to read configuration file %s: %v", configPath, err)
			return
		}
	}

	config = new(Configuration)
	if err = parseConfiguration(configBuf, config); err != nil {
		err = fmt.Errorf("Error parsing configuration file %s: %v", configPath, err)
		return
	}

	// try to parse the old .ngrok format for backwards compatibility
	matched := false
	content := strings.TrimSpace(string(configBuf))
	if matched, err = regexp.MatchString("^[0-9a-zA-Z_\\-!]+$", content); err != nil {
		return
	} else if matched {
		config = &Configuration{AuthToken: content}
	}

	// set configuration defaults
	if config.ServerAddr == "" {
		config.ServerAddr = defaultServerAddr
	}

	if config.InspectAddr == "" {
		config.InspectAddr = defaultInspectAddr
	}

	if config.HttpProxy == "" {
		config.HttpProxy = os.Getenv("http_proxy")
	}

	// validate and normalize configuration
	if config.InspectAddr != "disabled" {
		if config.InspectAddr, err = normalizeAddress(config.InspectAddr, "inspect_addr"); err != nil {
			return
		}
		if config.InspectAuth == "" {
			if config.InspectAuth = opts.inspectauth; config.InspectAuth == "" && !isLoopbackAddress(config.InspectAddr) {
				err = fmt.Errorf("inspect_addr %s is not loopback; set inspect_auth or use inspect_addr: disabled", config.InspectAddr)
				return
			}
		}
	}

	if config.ServerAddr, err = normalizeAddress(config.ServerAddr, "server_addr"); err != nil {
		return
	}

	if config.HttpProxy != "" {
		var proxyUrl *url.URL
		if proxyUrl, err = url.Parse(config.HttpProxy); err != nil {
			return
		} else {
			if proxyUrl.Scheme != "http" && proxyUrl.Scheme != "https" {
				err = fmt.Errorf("Proxy url scheme must be 'http' or 'https', got %v", proxyUrl.Scheme)
				return
			}
		}
	}

	for name, t := range config.Tunnels {
		if t == nil || t.Protocols == nil || len(t.Protocols) == 0 {
			err = fmt.Errorf("Tunnel %s does not specify any protocols to tunnel.", name)
			return
		}

		for k, addr := range t.Protocols {
			tunnelName := fmt.Sprintf("for tunnel %s[%s]", name, k)
			if t.Protocols[k], err = normalizeAddress(addr, tunnelName); err != nil {
				return
			}

			if err = validateProtocol(k, tunnelName); err != nil {
				return
			}
		}

		// use the name of the tunnel as the subdomain if none is specified
		if t.Hostname == "" && t.Subdomain == "" {
			// XXX: a crude heuristic, really we should be checking if the last part
			// is a TLD
			if len(strings.Split(name, ".")) > 1 {
				t.Hostname = name
			} else {
				t.Subdomain = name
			}
		}
	}

	// override configuration with command-line options
	config.LogTo = opts.logto
	config.Path = configPath
	if opts.authtoken != "" {
		config.AuthToken = opts.authtoken
	}

	switch opts.command {
	// start a single tunnel, the default, simple ngrok behavior
	case "default":
		config.Tunnels = make(map[string]*TunnelConfiguration)
		config.Tunnels["default"] = &TunnelConfiguration{
			Subdomain: opts.subdomain,
			Hostname:  opts.hostname,
			HttpAuth:  opts.httpauth,
			Protocols: make(map[string]string),
		}

		for _, proto := range strings.Split(opts.protocol, "+") {
			if err = validateProtocol(proto, "default"); err != nil {
				return
			}

			if config.Tunnels["default"].Protocols[proto], err = normalizeAddress(opts.args[0], ""); err != nil {
				return
			}
		}

	// list tunnels
	case "list":
		for name, _ := range config.Tunnels {
			fmt.Println(name)
		}
		os.Exit(0)

	// start tunnels
	case "start":
		if len(opts.args) == 0 {
			err = fmt.Errorf("You must specify at least one tunnel to start")
			return
		}

		requestedTunnels := make(map[string]bool)
		for _, arg := range opts.args {
			requestedTunnels[arg] = true

			if _, ok := config.Tunnels[arg]; !ok {
				err = fmt.Errorf("Requested to start tunnel %s which is not defined in the config file.", arg)
				return
			}
		}

		for name, _ := range config.Tunnels {
			if !requestedTunnels[name] {
				delete(config.Tunnels, name)
			}
		}

	case "start-all":
		return

	default:
		err = fmt.Errorf("Unknown command: %s", opts.command)
		return
	}

	return
}

func isLoopbackAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func defaultPath() string {
	user, err := user.Current()

	// user.Current() does not work on linux when cross compiling because
	// it requires CGO; use os.Getenv("HOME") hack until we compile natively
	homeDir := os.Getenv("HOME")
	if err != nil {
		log.Warn("Failed to get user's home directory: %s. Using $HOME: %s", err.Error(), homeDir)
	} else {
		homeDir = user.HomeDir
	}

	return path.Join(homeDir, ".ngrok")
}

func normalizeAddress(addr string, propName string) (string, error) {
	// normalize port to address
	if _, err := strconv.Atoi(addr); err == nil {
		addr = ":" + addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("Invalid address %s '%s': %s", propName, addr, err.Error())
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return fmt.Sprintf("%s:%s", host, port), nil
}

func validateProtocol(proto, propName string) (err error) {
	switch proto {
	case "http", "https", "http+https", "tcp":
	default:
		err = fmt.Errorf("Invalid protocol for %s: %s", propName, proto)
	}

	return
}

func SaveAuthToken(configPath, authtoken string) (err error) {
	// empty configuration by default for the case that we can't read it
	c := new(Configuration)

	// read the configuration
	oldConfigBytes, err := os.ReadFile(configPath)
	if err == nil {
		// unmarshal if we successfully read the configuration file
		if err = parseConfiguration(oldConfigBytes, c); err != nil {
			return
		}
	}

	// no need to save, the authtoken is already the correct value
	if c.AuthToken == authtoken {
		return
	}

	// update auth token
	c.AuthToken = authtoken

	// rewrite configuration
	newConfigBytes := []byte(formatConfiguration(c))
	err = os.WriteFile(configPath, newConfigBytes, 0600)
	return
}

func parseConfiguration(data []byte, config *Configuration) error {
	if config.Tunnels == nil {
		config.Tunnels = make(map[string]*TunnelConfiguration)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var section string
	var currentTunnel *TunnelConfiguration
	var inProto bool

	for lineNo := 1; scanner.Scan(); lineNo++ {
		raw := stripComment(scanner.Text())
		if strings.TrimSpace(raw) == "" {
			continue
		}

		indent := leadingSpaces(raw)
		key, value, ok := splitKeyValue(strings.TrimSpace(raw))
		if !ok {
			return fmt.Errorf("line %d: expected key: value", lineNo)
		}

		switch indent {
		case 0:
			section = ""
			currentTunnel = nil
			inProto = false
			switch key {
			case "http_proxy":
				config.HttpProxy = value
			case "server_addr":
				config.ServerAddr = value
			case "inspect_addr":
				config.InspectAddr = value
			case "inspect_auth":
				config.InspectAuth = value
			case "trust_host_root_certs":
				config.TrustHostRootCerts = parseBool(value)
			case "auth_token":
				config.AuthToken = value
			case "tunnels":
				section = "tunnels"
			default:
				return fmt.Errorf("line %d: unknown config key %q", lineNo, key)
			}

		case 2:
			if section != "tunnels" {
				return fmt.Errorf("line %d: unexpected nested key %q", lineNo, key)
			}
			inProto = false
			currentTunnel = &TunnelConfiguration{Protocols: make(map[string]string)}
			config.Tunnels[key] = currentTunnel

		case 4:
			if currentTunnel == nil {
				return fmt.Errorf("line %d: tunnel option without tunnel", lineNo)
			}
			inProto = false
			switch key {
			case "subdomain":
				currentTunnel.Subdomain = value
			case "hostname":
				currentTunnel.Hostname = value
			case "auth":
				currentTunnel.HttpAuth = value
			case "remote_port":
				port, err := strconv.ParseUint(value, 10, 16)
				if err != nil {
					return fmt.Errorf("line %d: invalid remote_port", lineNo)
				}
				currentTunnel.RemotePort = uint16(port)
			case "proto":
				inProto = true
			default:
				return fmt.Errorf("line %d: unknown tunnel key %q", lineNo, key)
			}

		case 6:
			if currentTunnel == nil || !inProto {
				return fmt.Errorf("line %d: protocol without proto section", lineNo)
			}
			currentTunnel.Protocols[key] = value

		default:
			return fmt.Errorf("line %d: unsupported indentation", lineNo)
		}
	}

	return scanner.Err()
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func splitKeyValue(line string) (key string, value string, ok bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key = strings.TrimSpace(parts[0])
	value = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	return key, value, key != ""
}

func parseBool(value string) bool {
	switch strings.ToLower(value) {
	case "true", "yes", "1", "on":
		return true
	default:
		return false
	}
}

func formatConfiguration(c *Configuration) string {
	var b strings.Builder
	writeString := func(key, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s: %s\n", key, value)
		}
	}

	writeString("http_proxy", c.HttpProxy)
	writeString("server_addr", c.ServerAddr)
	writeString("inspect_addr", c.InspectAddr)
	writeString("inspect_auth", c.InspectAuth)
	if c.TrustHostRootCerts {
		b.WriteString("trust_host_root_certs: true\n")
	}
	writeString("auth_token", c.AuthToken)

	if len(c.Tunnels) > 0 {
		b.WriteString("tunnels:\n")
		for name, tunnel := range c.Tunnels {
			fmt.Fprintf(&b, "  %s:\n", name)
			writeTunnelString(&b, "subdomain", tunnel.Subdomain)
			writeTunnelString(&b, "hostname", tunnel.Hostname)
			writeTunnelString(&b, "auth", tunnel.HttpAuth)
			if tunnel.RemotePort != 0 {
				fmt.Fprintf(&b, "    remote_port: %d\n", tunnel.RemotePort)
			}
			if len(tunnel.Protocols) > 0 {
				b.WriteString("    proto:\n")
				for proto, addr := range tunnel.Protocols {
					fmt.Fprintf(&b, "      %s: %s\n", proto, addr)
				}
			}
		}
	}

	return b.String()
}

func writeTunnelString(b *strings.Builder, key, value string) {
	if value != "" {
		fmt.Fprintf(b, "    %s: %s\n", key, value)
	}
}
