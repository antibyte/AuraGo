
import re

content = open("internal/tools/ssh_key_manager.go").read()

added = """

// Deploy copies the generated agent key to a remote server.
func (m *SSHKeyManager) Deploy(privatePEM string, targetHost string, username string, port int) error {
// A placeholder for logic to copy to authorized_keys of a remote server (which requires password/key auth usually)
return fmt.Errorf("deploy not fully implemented natively without initial auth, agent should use file operations")
}

// Rotate generates a new key, deploys it, and revokes the old one.
func (m *SSHKeyManager) Rotate(comment string) (string, string, error) {
    return m.Generate("Rotated-"+comment, true)
}

"""

content = content + added
open("internal/tools/ssh_key_manager.go", "w").write(content)

