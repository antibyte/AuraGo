package tools

import "testing"

func TestIsSensitiveHostDirectory(t *testing.T) {
	for dir, want := range map[string]bool{
		"/":                         true,
		"/etc":                      true,
		"/etc/wikipedia":            true,
		"/root/wiki":                true,
		"/proc/1":                   true,
		"/var/lib/docker/volumes/x": true,
		"/usr/share/wikipedia":      true,
		"/run/aurago":               true,
		"/mnt":                      true,
		"/mnt/":                     true,
		"/mnt/data/wikipedia":       false,
		"/srv/wikipedia":            false,
		"/home/aurago/wikipedia":    false,
		"/var/lib/aurago/wikipedia": false,
		"/opt/aurago/data/wiki":     false,
		`C:\`:                       true,
		`D:`:                        true,
		`C:\Windows\wiki`:           true,
		`C:\Program Files\wiki`:     true,
		`C:\ProgramData\wiki`:       true,
		`D:\Wikipedia`:              false,
		`C:\Users\andi\Wikipedia`:   false,
	} {
		if got := IsSensitiveHostDirectory(dir); got != want {
			t.Fatalf("IsSensitiveHostDirectory(%q) = %v, want %v", dir, got, want)
		}
	}
}
