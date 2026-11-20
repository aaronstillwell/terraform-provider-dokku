package provider

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var DOKKU_VERSION semver.Version

// Provider -
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"ssh_host": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_HOST", nil),
				Description: "The hostname of your Dokku server. Can be set via DOKKU_SSH_HOST environment variable.",
			},
			"ssh_user": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_USER", "dokku"),
				Description: "The SSH user to connect to your Dokku server. Defaults to 'dokku'. Can be set via DOKKU_SSH_USER environment variable.",
			},
			"ssh_port": {
				Type:        schema.TypeInt,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_PORT", 22),
				Description: "The SSH port of your Dokku server. Defaults to 22. Can be set via DOKKU_SSH_PORT environment variable.",
			},
			"ssh_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_CERT", nil),
				Description: "Either a path to the SSH private key for connecting to your Dokku server OR the source for an SSH key directly. When unset, the provider resolves ~/.ssh/config via ssh -G (IdentityFile including .pub files matched against the SSH agent such as 1Password bookmarks, IdentitiesOnly, and IdentityAgent), falling back to active SSH agent keys and ~/.ssh/id_{ed25519,rsa,ecdsa,dsa}. Can be set via DOKKU_SSH_CERT environment variable.",
			},
			"ssh_passphrase": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SSH_PASSPHRASE", nil),
				Description: "An optional passphrase to be used in conjunction with the provided SSH key.",
			},
			"fail_on_untested_version": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_FAIL_ON_UNTESTED_VERSION", true),
				Description: "Whether to fail if the Dokku version has not been tested with this provider. Defaults to true. Can be set via DOKKU_FAIL_ON_UNTESTED_VERSION environment variable.",
			},
			"skip_known_hosts_check": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("DOKKU_SKIP_KNOWN_HOSTS_CHECK", false),
				Description: "Whether to skip SSH known hosts verification. Defaults to false. Can be set via DOKKU_SKIP_KNOWN_HOSTS_CHECK environment variable.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"dokku_app":                     resourceApp(),
			"dokku_postgres_service":        resourcePostgresService(),
			"dokku_postgres_service_link":   resourcePostgresServiceLink(),
			"dokku_redis_service":           resourceRedisService(),
			"dokku_redis_service_link":      resourceRedisServiceLink(),
			"dokku_mysql_service":           resourceMysqlService(),
			"dokku_mysql_service_link":      resourceMysqlServiceLink(),
			"dokku_clickhouse_service":      resourceClickhouseService(),
			"dokku_clickhouse_service_link": resourceClickhouseServiceLink(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	host := d.Get("ssh_host").(string)
	user := d.Get("ssh_user").(string)
	port := uint(d.Get("ssh_port").(int))
	ssh_cert := d.Get("ssh_cert").(string)
	ssh_passphrase := d.Get("ssh_passphrase").(string)

	auth, err := resolveSSHAuth(ssh_cert, ssh_passphrase, host, user, port)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	log.Printf("[DEBUG] establishing SSH connection\n")
	log.Printf("[DEBUG] host %v\n", host)
	log.Printf("[DEBUG] user %v\n", user)
	log.Printf("[DEBUG] port %v\n", port)
	log.Printf("[DEBUG] skip_known_hosts_check %v\n", d.Get("skip_known_hosts_check").(bool))

	var diags diag.Diagnostics

	// Check known hosts
	// https://github.com/melbahja/goph/blob/6258fe9f54bb1f738543020ade7ab22c1dd233d7/examples/goph/main.go#L75-L109
	verifyFn := func(host string, remote net.Addr, key ssh.PublicKey) error {
		// See https://github.com/aaronstillwell/terraform-provider-dokku/issues/15 - this option
		// has been implemented to support using the provider on Terraform Cloud
		if d.Get("skip_known_hosts_check").(bool) == false {
			hostFound, err := goph.CheckKnownHost(host, remote, key, "")

			// Host in known hosts but key mismatch
			if hostFound && err != nil {
				return err
			}

			// handshake because public key already exists.
			if hostFound && err == nil {
				return nil
			}
		} else {
			log.Printf("[WARN]: skip_known_hosts_check is set to true, no key verification will be run against the SSH host")
		}
		return goph.AddKnownHost(host, remote, key, "")
	}

	sshConfig := &goph.Config{
		Auth:     auth,
		Addr:     host,
		Port:     port,
		User:     user,
		Callback: verifyFn,
	}

	client, err := goph.NewConn(sshConfig)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return nil, diag.Errorf("Could not establish SSH connection: %v", err)
	}

	res := run(client, "version")

	// Check for 127 status code... suggests that we're not authenticating
	// with a dokku user (see https://github.com/aaronstillwell/terraform-provider-dokku/issues/1)
	if res.status == 127 {
		log.Printf("[ERROR] must use a dokku user for authentication, see the docs")
		return nil, diag.Errorf("[ERROR] must use a dokku user for authentication, see the docs")
	}

	re := regexp.MustCompile("[0-9]+\\.[0-9]+\\.[0-9]+")
	found := re.FindString(res.stdout)

	hostVersion, err := semver.Parse(string(found))

	DOKKU_VERSION = hostVersion

	log.Printf("[DEBUG] host version %v", hostVersion)

	testedVersions := ">=0.30.0 <0.37.0"
	testedErrMsg := fmt.Sprintf("This provider has not been tested against Dokku version %s. Tested version range: %s", string(found), testedVersions)

	if err == nil {
		compat, _ := semver.ParseRange(testedVersions)

		if !compat(hostVersion) {

			log.Printf("[DEBUG] fail_on_untested_version: %v", d.Get("fail_on_untested_version").(bool))

			if d.Get("fail_on_untested_version").(bool) {
				return client, diag.Errorf(testedErrMsg)
			}
			warn := diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  testedErrMsg,
			}
			diags = append(diags, warn)
			return client, diags
		}
	} else {
		return client, diag.Errorf("Could not detect dokku version - tested version range: %s", testedVersions)
	}

	return client, diags
}

func resolveSSHAuth(sshCert, sshPassphrase, host, user string, port uint) (goph.Auth, error) {
	if sshCert != "" {
		var err error
		if sshPassphrase != "" {
			_, err = ssh.ParsePrivateKeyWithPassphrase([]byte(sshCert), []byte(sshPassphrase))
		} else {
			_, err = ssh.ParsePrivateKey([]byte(sshCert))
		}

		if err != nil {
			log.Printf("[DEBUG] attempting to load SSH cert from file path %s\n", sshCert)
			log.Printf("[DEBUG] %v\n", err)
			// could not parse a key directly, try it as a filepath
			auth, err := goph.Key(sshCert, sshPassphrase)
			if err != nil {
				return nil, fmt.Errorf("Attempted to load private key from file but failed")
			}
			return auth, nil
		}

		log.Printf("[DEBUG] SSH cert looks like its being provided inline\n")
		auth, err := goph.RawKey(sshCert, sshPassphrase)
		if err != nil {
			// could not proceed with inline ssh cert
			return nil, fmt.Errorf("Could not auth with inline SSH key: %v", err)
		}
		return auth, nil
	}

	log.Printf("[DEBUG] no SSH cert specified, attempting to load default SSH keys\n")
	home, _ := os.UserHomeDir()
	return resolveDefaultSSHAuth(os.Getenv("SSH_AUTH_SOCK"), home, sshPassphrase, host, user, port)
}

func resolveDefaultSSHAuth(agentSock, homeDir, sshPassphrase, host, user string, port uint) (goph.Auth, error) {
	var idFiles []string
	var identitiesOnly bool

	if host != "" && homeDir != "" {
		cfgPath := filepath.Join(homeDir, ".ssh", "config")
		if _, err := os.Stat(cfgPath); err == nil {
			cfg := loadSSHConfigViaBinary(cfgPath, homeDir, host, user, port)
			idFiles = cfg.identityFiles
			identitiesOnly = cfg.identitiesOnly
			if cfg.identityAgent != "" {
				agentSock = resolveIdentityAgent(cfg.identityAgent, agentSock, homeDir, host, user, port)
			}
		}
	}

	var sshAgentConn net.Conn
	var agentSigners []ssh.Signer

	if agentSock != "" {
		conn, err := net.Dial("unix", agentSock)
		if err != nil {
			log.Printf("[DEBUG] failed to connect to SSH agent at %s: %v\n", agentSock, err)
		} else {
			agentClient := agent.NewClient(conn)
			loaded, err := agentClient.Signers()
			if err != nil {
				log.Printf("[DEBUG] failed to get signers from SSH agent: %v\n", err)
				conn.Close()
			} else if len(loaded) == 0 {
				log.Printf("[DEBUG] SSH agent has no keys loaded\n")
				conn.Close()
			} else {
				log.Printf("[DEBUG] loaded %d key(s) from SSH agent\n", len(loaded))
				sshAgentConn = conn
				agentSigners = loaded
			}
		}
	}

	var signers []ssh.Signer
	seen := make(map[string]struct{})
	usedAgentSigner := false

	addSigner := func(s ssh.Signer) bool {
		if s == nil {
			return false
		}
		k := string(s.PublicKey().Marshal())
		if _, exists := seen[k]; exists {
			return false
		}
		seen[k] = struct{}{}
		signers = append(signers, s)
		return true
	}

	matchAgentSigners := func(pubKey ssh.PublicKey) bool {
		if pubKey == nil {
			return false
		}
		target := pubKey.Marshal()
		matched := false
		for _, s := range agentSigners {
			if bytes.Equal(s.PublicKey().Marshal(), target) {
				if addSigner(s) {
					usedAgentSigner = true
				}
				matched = true
			}
		}
		return matched
	}

	hasExplicitID := false
	disableDefaultFiles := false

	for _, raw := range idFiles {
		if strings.EqualFold(raw, "none") {
			disableDefaultFiles = true
			continue
		}
		hasExplicitID = true
		p := expandSSHPath(raw, homeDir, host, user, port)

		pubPath := p
		if !strings.HasSuffix(p, ".pub") {
			pubPath = p + ".pub"
		}
		if pubKey, ok := parsePublicKeyFile(pubPath); ok {
			matchAgentSigners(pubKey)
		}

		if !strings.HasSuffix(p, ".pub") {
			if _, err := os.Stat(p); err == nil {
				signer, err := goph.GetSigner(p, sshPassphrase)
				if err != nil {
					log.Printf("[DEBUG] failed to get signer for IdentityFile %s: %v\n", p, err)
				} else if addSigner(signer) {
					log.Printf("[DEBUG] loaded SSH key from %s\n", p)
				}
			}
		}
	}

	if !identitiesOnly {
		for _, s := range agentSigners {
			if addSigner(s) {
				usedAgentSigner = true
			}
		}
	}

	if !hasExplicitID && !disableDefaultFiles && homeDir != "" {
		defaultKeyFiles := []string{
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ecdsa"),
			filepath.Join(homeDir, ".ssh", "id_dsa"),
		}

		for _, keyFile := range defaultKeyFiles {
			if identitiesOnly {
				if pubKey, ok := parsePublicKeyFile(keyFile + ".pub"); ok {
					matchAgentSigners(pubKey)
				}
			}
			if _, err := os.Stat(keyFile); err == nil {
				signer, err := goph.GetSigner(keyFile, sshPassphrase)
				if err != nil {
					log.Printf("[DEBUG] failed to get signer for default key %s: %v\n", keyFile, err)
					continue
				}
				if identitiesOnly {
					matchAgentSigners(signer.PublicKey())
				}
				if addSigner(signer) {
					log.Printf("[DEBUG] loaded default SSH key from %s\n", keyFile)
				}
			}
		}
	}

	if sshAgentConn != nil && !usedAgentSigner {
		sshAgentConn.Close()
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no SSH cert specified and no default SSH keys found in SSH agent or ~/.ssh")
	}

	// SSH only tries one auth method per type. Keep prioritized/agent keys first,
	// with disk keys in the same public-key method so they are tried in order.
	return goph.Auth{ssh.PublicKeys(signers...)}, nil
}

type sshHostSettings struct {
	identityFiles  []string
	identitiesOnly bool
	identityAgent  string
}

func isValidSSHArg(s string) bool {
	return !strings.HasPrefix(s, "-") && !strings.ContainsAny(s, "\n\r\x00")
}

func loadSSHConfigViaBinary(cfgPath, homeDir, host, user string, port uint) sshHostSettings {
	if host == "" || !isValidSSHArg(host) || (user != "" && !isValidSSHArg(user)) {
		return sshHostSettings{}
	}

	args := []string{"-G", "-F", cfgPath}
	if port > 0 {
		args = append(args, "-p", strconv.FormatUint(uint64(port), 10))
	}
	if user != "" {
		args = append(args, "-l", user)
	}
	args = append(args, "--", host)

	cmd := exec.Command("ssh", args...)
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HOME=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "HOME="+homeDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[DEBUG] ssh -G failed for %s: %v (%s)\n", cfgPath, err, strings.TrimSpace(stderr.String()))
		return sshHostSettings{}
	}

	var settings sshHostSettings
	for _, rawLine := range strings.Split(stdout.String(), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" {
			continue
		}
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(line[:idx])
		val := unquoteSSHValue(strings.TrimSpace(line[idx+1:]))
		if val == "" {
			continue
		}
		switch key {
		case "identityfile":
			settings.identityFiles = append(settings.identityFiles, val)
		case "identitiesonly":
			settings.identitiesOnly = strings.EqualFold(val, "yes")
		case "identityagent":
			settings.identityAgent = val
		}
	}

	if isDefaultOpenSSHIdentityFiles(settings.identityFiles) {
		settings.identityFiles = nil
	}
	return settings
}

func isDefaultOpenSSHIdentityFiles(files []string) bool {
	if len(files) <= 1 || files[0] != "~/.ssh/id_rsa" {
		return false
	}
	for _, f := range files {
		switch f {
		case "~/.ssh/id_rsa", "~/.ssh/id_ecdsa", "~/.ssh/id_ecdsa_sk",
			"~/.ssh/id_ed25519", "~/.ssh/id_ed25519_sk", "~/.ssh/id_dsa",
			"~/.ssh/id_xmss", "~/.ssh/id_mldsa44_ed25519":
		default:
			return false
		}
	}
	return true
}

func unquoteSSHValue(v string) string {
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		return v[1 : len(v)-1]
	}
	return v
}

func resolveIdentityAgent(val, envSock, homeDir, host, user string, port uint) string {
	val = unquoteSSHValue(strings.TrimSpace(val))
	if strings.EqualFold(val, "none") {
		return ""
	}
	if val == "SSH_AUTH_SOCK" || val == "$SSH_AUTH_SOCK" || val == "${SSH_AUTH_SOCK}" {
		return envSock
	}
	if strings.HasPrefix(val, "$") {
		return os.Expand(val, os.Getenv)
	}
	return expandSSHPath(val, homeDir, host, user, port)
}

func expandSSHPath(raw, homeDir, host, user string, port uint) string {
	p := unquoteSSHValue(strings.TrimSpace(raw))
	if p == "~" && homeDir != "" {
		p = homeDir
	} else if strings.HasPrefix(p, "~/") && homeDir != "" {
		p = filepath.Join(homeDir, p[2:])
	}

	portStr := ""
	if port > 0 {
		portStr = strconv.FormatUint(uint64(port), 10)
	}

	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '%' && i+1 < len(p) {
			switch p[i+1] {
			case '%':
				b.WriteByte('%')
				i++
				continue
			case 'd':
				b.WriteString(homeDir)
				i++
				continue
			case 'h', 'n':
				b.WriteString(host)
				i++
				continue
			case 'r':
				b.WriteString(user)
				i++
				continue
			case 'p':
				b.WriteString(portStr)
				i++
				continue
			}
		}
		b.WriteByte(p[i])
	}
	return b.String()
}

func parsePublicKeyFile(pubPath string) (ssh.PublicKey, bool) {
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, false
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] == '#' {
			continue
		}
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey(trimmed)
		if err == nil {
			return pubKey, true
		}
	}
	return nil, false
}
