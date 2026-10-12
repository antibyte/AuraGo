package main

import (
	"flag"
	"fmt"

	"aurago/internal/config"
)

type networkInitFlags struct {
	enabled                   bool
	host, mode, domain, email string
	httpsPort, httpPort       int
}

func (o *networkInitFlags) register(fs *flag.FlagSet) {
	fs.BoolVar(&o.enabled, "https", false, "Explicitly enable or disable HTTPS (-https=false)")
	fs.StringVar(&o.host, "host", "", "Server bind address to save")
	fs.StringVar(&o.mode, "cert-mode", "", "Certificate mode: selfsigned, auto (Let's Encrypt), custom")
	fs.StringVar(&o.domain, "domain", "", "Domain for HTTPS")
	fs.StringVar(&o.email, "email", "", "Email for Let's Encrypt")
	fs.IntVar(&o.httpsPort, "https-port", 0, "HTTPS port to save; 0 selects 443 or 8443 during initialization")
	fs.IntVar(&o.httpPort, "http-port", 0, "HTTP redirect/challenge port to save; 0 disables the listener")
}

func (o *networkInitFlags) apply(fs *flag.FlagSet, cfg *config.Config, path string) error {
	present := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { present[f.Name] = true })
	updated := *cfg
	changed := false
	for _, name := range []string{"https", "host", "cert-mode", "domain", "email", "https-port", "http-port"} {
		changed = changed || present[name]
	}
	if !changed {
		return nil
	}
	if present["https"] {
		updated.Server.HTTPS.Enabled = o.enabled
	}
	if present["host"] {
		updated.Server.Host = o.host
	}
	if present["domain"] {
		updated.Server.HTTPS.Domain = o.domain
	}
	if present["email"] {
		updated.Server.HTTPS.Email = o.email
	}
	if present["cert-mode"] {
		mode := config.NormalizeCertMode(o.mode)
		if mode != "selfsigned" && mode != "auto" && mode != "custom" {
			return fmt.Errorf("invalid -cert-mode %q: use selfsigned, auto or custom", o.mode)
		}
		updated.Server.HTTPS.CertMode = mode
	}
	if present["https-port"] {
		port := o.httpsPort
		if port < 0 || port > 65535 {
			return fmt.Errorf("-https-port must be between 0 and 65535")
		}
		if port == 0 {
			var err error
			port, err = config.SelectInitialHTTPSPort(config.EffectiveServerHost(updated.Server.Host))
			if err != nil {
				return err
			}
		}
		updated.Server.HTTPS.HTTPSPort = port
	}
	if present["http-port"] {
		if o.httpPort < 0 || o.httpPort > 65535 {
			return fmt.Errorf("-http-port must be between 0 and 65535")
		}
		updated.Server.HTTPS.HTTPPort = o.httpPort
	}
	values := map[string]interface{}{}
	for flagName, field := range map[string]string{"https": "https.enabled", "host": "host", "cert-mode": "https.cert_mode", "domain": "https.domain", "email": "https.email", "https-port": "https.https_port", "http-port": "https.http_port"} {
		if !present[flagName] {
			continue
		}
		all := map[string]interface{}{"https.enabled": updated.Server.HTTPS.Enabled, "host": updated.Server.Host, "https.cert_mode": updated.Server.HTTPS.CertMode, "https.domain": updated.Server.HTTPS.Domain, "https.email": updated.Server.HTTPS.Email, "https.https_port": updated.Server.HTTPS.HTTPSPort, "https.http_port": updated.Server.HTTPS.HTTPPort}
		values[field] = all[field]
	}
	if err := config.SaveInitialNetworkSettings(path, values); err != nil {
		return fmt.Errorf("save network configuration: %w", err)
	}
	*cfg = updated
	return nil
}
