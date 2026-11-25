package desktop

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	SSHHostKeyUnknown  = "unknown_host_key"
	SSHHostKeyMismatch = "host_key_mismatch"
)

var ErrSSHDeviceNotFound = errors.New("device not found")

// SSHAccess is resolved access data of one SSH device. Secret must never be serialised.
type SSHAccess struct {
	DeviceID, Name, Host string
	Port                 int
	Username             string
	Secret               []byte `json:"-"`
}

// SSHHostKeyError: known_hosts does not vouch for the presented key.
type SSHHostKeyError struct{ Code, Host, KeyType, Fingerprint string }

func (e *SSHHostKeyError) Error() string {
	if e.Code == SSHHostKeyMismatch {
		return fmt.Sprintf("SSH host key of %s changed (%s %s)", e.Host, e.KeyType, e.Fingerprint)
	}
	return fmt.Sprintf("unknown SSH host key for %s (%s %s)", e.Host, e.KeyType, e.Fingerprint)
}

type SSHDialOptions struct {
	TrustFingerprint string        // stored in known_hosts only if it equals the presented key's SHA256 fingerprint
	KnownHostsFile   string        // "" = ~/.ssh/known_hosts
	Timeout          time.Duration // 0 = 15 s
	Logger           *slog.Logger
}

// ResolveSSHDevice loads a device and its secret (same rules as Quick Connect SSH).
func ResolveSSHDevice(inventoryDB *sql.DB, vault *security.Vault, deviceID string) (SSHAccess, error) {
	device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return SSHAccess{}, fmt.Errorf("%w: %s", ErrSSHDeviceNotFound, deviceID)
		}
		return SSHAccess{}, err
	}
	host, port, user, secret, err := resolveSSHAccess(device, inventoryDB, vault)
	if err != nil {
		return SSHAccess{}, err
	}
	return SSHAccess{DeviceID: device.ID, Name: device.Name, Host: host, Port: port, Username: user, Secret: secret}, nil
}

// DescribeSSHDevice returns host/port/user without reading the vault (Secret stays nil).
func DescribeSSHDevice(inventoryDB *sql.DB, device inventory.DeviceRecord) SSHAccess {
	a := SSHAccess{DeviceID: device.ID, Name: device.Name, Host: strings.TrimSpace(device.IPAddress),
		Port: device.Port, Username: strings.TrimSpace(device.Username)}
	if a.Host == "" {
		a.Host = strings.TrimSpace(device.Name)
	}
	if a.Port <= 0 {
		a.Port = 22
	}
	if id := strings.TrimSpace(device.CredentialID); id != "" && inventoryDB != nil {
		if cred, err := credentials.GetByID(inventoryDB, id); err == nil {
			if strings.TrimSpace(cred.Host) != "" {
				a.Host = strings.TrimSpace(cred.Host)
			}
			if strings.TrimSpace(cred.Username) != "" {
				a.Username = strings.TrimSpace(cred.Username)
			}
		}
	}
	return a
}

// DialSSH connects without any interactive prompt. Unknown keys return *SSHHostKeyError
// unless TrustFingerprint matches; changed keys always return *SSHHostKeyError (mismatch).
func DialSSH(ctx context.Context, access SSHAccess, opts SSHDialOptions) (*ssh.Client, error) {
	var auth []ssh.AuthMethod
	if signer, err := ssh.ParsePrivateKey(access.Secret); err == nil {
		auth = append(auth, ssh.PublicKeys(signer))
	} else if bytes.Contains(access.Secret, []byte("-----BEGIN")) {
		// Never send key material as a password: passphrase-protected or unsupported keys fail here.
		return nil, errors.New("the stored SSH private key could not be used (passphrase-protected or unsupported key format)")
	} else {
		auth = append(auth, ssh.Password(string(access.Secret)))
	}
	var captured *SSHHostKeyError
	callback := ssh.InsecureIgnoreHostKey() //nolint:gosec // explicit homelab opt-in, like Quick Connect
	if !remote.InsecureHostKey {
		file := opts.KnownHostsFile
		if file == "" {
			var err error
			if file, err = defaultKnownHostsFile(); err != nil {
				return nil, err
			}
		}
		callback = trustingKnownHostsCallback(file, strings.TrimSpace(opts.TrustFingerprint), opts.Logger, &captured)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	cfg := &ssh.ClientConfig{User: access.Username, Auth: auth, HostKeyCallback: callback, Timeout: timeout}
	client, err := dialDesktopSSH(ctx, net.JoinHostPort(access.Host, strconv.Itoa(access.Port)), cfg)
	if err != nil {
		if captured != nil {
			return nil, captured
		}
		return nil, err
	}
	return client, nil
}

func trustingKnownHostsCallback(file, trust string, logger *slog.Logger, captured **SSHHostKeyError) ssh.HostKeyCallback {
	return func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
		hk := &SSHHostKeyError{Code: SSHHostKeyUnknown, Host: knownhosts.Normalize(hostname),
			KeyType: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)}
		cb, err := knownhosts.New(file)
		switch {
		case err == nil:
			verr := cb(hostname, remoteAddr, key)
			if verr == nil {
				return nil
			}
			var keyErr *knownhosts.KeyError
			if !errors.As(verr, &keyErr) {
				return fmt.Errorf("SSH host key verification failed: %w", verr)
			}
			if len(keyErr.Want) > 0 {
				hk.Code = SSHHostKeyMismatch
				*captured = hk
				return hk
			}
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("SSH host key verification failed: load known_hosts: %w", err)
		}
		if trust == "" || trust != hk.Fingerprint {
			*captured = hk
			return hk
		}
		if err := appendKnownHostKey(file, hostname, remoteAddr, key); err != nil {
			return fmt.Errorf("store SSH host key: %w", err)
		}
		if logger != nil {
			logger.Info("stored SSH host key from Code Studio", "host", hk.Host, "key_type", hk.KeyType)
		}
		return nil
	}
}
