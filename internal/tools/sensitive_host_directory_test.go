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

func TestIsSensitiveHostDirectoryWindowsDeviceAndUNCForms(t *testing.T) {
	for dir, want := range map[string]bool{
		// \\?\ and \\.\ prefixes are stripped before the drive checks.
		`\\?\C:\Windows\wiki`:             true,
		`\\.\C:\Windows`:                  true,
		`\\?\C:\`:                         true,
		`\\?\C:`:                          true,
		`\\.\D:\`:                         true,
		`\\?\c:\program files (x86)\wiki`: true,
		`//?/C:/ProgramData/wiki`:         true,
		`\\?\D:\Wikipedia`:                false,
		`\\.\D:\data\wikipedia`:           false,
		`\\?\C:\Users\andi\Wikipedia`:     false,
		// Device paths that name neither a drive nor a share cannot be classified.
		`\\?\Volume{0b8d6b4e-0000-0000-0000-100000000000}\wiki`: true,
		`\\.\PhysicalDrive0`:                         true,
		`\\?\GLOBALROOT\Device\HarddiskVolume1\wiki`: true,
		`\\?\`: true,
		// UNC: administrative shares expose the system drive; plain shares are fine.
		`\\localhost\C$\Windows`:       true,
		`\\localhost\C$\Windows\wiki`:  true,
		`\\host\c$`:                    true,
		`\\host\D$\data`:               true,
		`\\host\ADMIN$\system32`:       true,
		`//host/c$/wiki`:               true,
		`//localhost/C$/Windows`:       true,
		`\\?\UNC\localhost\C$\Windows`: true,
		`\\.\UNC\host\admin$`:          true,
		`\\host\\C$\Windows`:           true,
		`\\nas\wikipedia`:              false,
		`\\nas\share\wiki`:             false,
		`//nas/wikipedia/data`:         false,
		`\\?\UNC\nas\wikipedia\data`:   false,
		`\\nas\data$\wiki`:             false,
	} {
		if got := IsSensitiveHostDirectory(dir); got != want {
			t.Fatalf("IsSensitiveHostDirectory(%q) = %v, want %v", dir, got, want)
		}
	}
}

func TestIsSensitiveHostDirectoryWSLMounts(t *testing.T) {
	for dir, want := range map[string]bool{
		"/mnt/c":                       true,
		"/mnt/c/":                      true,
		"/mnt/d":                       true,
		"/mnt/c/Windows":               true,
		"/mnt/c/Windows/wiki":          true,
		"/mnt/C/WINDOWS/wiki":          true,
		"/MNT/c/windows":               true,
		"/mnt/c/Program Files/wiki":    true,
		"/mnt/c/Program Files (x86)/x": true,
		"/mnt/c/ProgramData":           true,
		"/mnt/c/Users/andi/Wikipedia":  false,
		"/mnt/d/Wikipedia":             false,
		"/mnt/d/wiki/":                 false,
		"/mnt/wsl":                     true,
		"/mnt/wsl/shared":              true,
		"/mnt/wslg":                    true,
		"/mnt/wslg/runtime-dir":        true,
		"/mnt/data":                    false,
		"/mnt/data/wikipedia":          false,
		"/mnt/ab/wikipedia":            false,
		"/mnt/disk1/etc":               false,
	} {
		if got := IsSensitiveHostDirectory(dir); got != want {
			t.Fatalf("IsSensitiveHostDirectory(%q) = %v, want %v", dir, got, want)
		}
	}
}

func TestIsSensitiveHostDirectoryMacOSTrees(t *testing.T) {
	for dir, want := range map[string]bool{
		"/System":                                     true,
		"/System/Library/Caches":                      true,
		"/System/Volumes/Data/wiki":                   true,
		"/Library":                                    true,
		"/Library/Application Support/wiki":           true,
		"/Applications/Wikipedia":                     true,
		"/private":                                    true,
		"/private/etc":                                true,
		"/private/etc/wiki":                           true,
		"/private/var/db/wiki":                        true,
		"/private/tmp":                                true,
		"/usr/local/share/wiki":                       true,
		"/Volumes":                                    true,
		"/Volumes/":                                   true,
		"/Volumes/Data":                               false,
		"/Volumes/Data/wikipedia":                     false,
		"/Volumes/External SSD/Wikipedia/":            false,
		"/Users/andi/Library":                         true,
		"/Users/andi/Library/Application Support/x":   true,
		"/users/ANDI/LIBRARY/Caches":                  true,
		"/Users/andi/wikipedia":                       false,
		"/Users/andi/Documents/Library/wikipedia":     false,
		"/Users/Shared/wikipedia":                     false,
		"/Users/andi":                                 false,
		"/Users/Library":                              false,
		"/var/folders/xy/T/aurago-data":               false,
		"/Applications/../Volumes/Data/wikipedia":     false,
		"/Volumes/Data/../../System/Library":          true,
		"/Volumes/Data/../../Users/andi/Library/wiki": true,
	} {
		if got := IsSensitiveHostDirectory(dir); got != want {
			t.Fatalf("IsSensitiveHostDirectory(%q) = %v, want %v", dir, got, want)
		}
	}
}

func TestIsSensitiveHostDirectoryNormalisation(t *testing.T) {
	for dir, want := range map[string]bool{
		// ".." is resolved before the check.
		"/srv/../etc/wiki":            true,
		"/srv/wiki/../../etc":         true,
		"/mnt/data/../../etc":         true,
		"/mnt/data/..":                true,
		"/mnt/data/../c/Windows/wiki": true,
		"/etc/../srv/wiki":            false,
		"/srv/wiki/..":                false,
		"/srv/wiki/../other":          false,
		`C:\Users\..\Windows\wiki`:    true,
		`D:\wiki\..\..\Windows`:       true,
		`C:\Windows\..\Users\andi\w`:  false,
		`\\?\C:\Users\..\Windows`:     true,
		// Case is folded: Windows and macOS volumes are case-insensitive.
		"/ETC/wiki":                  true,
		"/Usr/share/x":               true,
		"/Run/aurago":                true,
		"/Mnt":                       true,
		"/MNT/":                      true,
		"/MNT/Data/Wikipedia":        false,
		`C:\WINDOWS\wiki`:            true,
		`c:\windows`:                 true,
		`c:\program files (x86)\wik`: true,
		`D:\WIKIPEDIA`:               false,
		// Trailing and repeated separators.
		"/etc/":                    true,
		"/etc//wiki":               true,
		"//etc/wiki":               true,
		"///etc":                   true,
		"/run/":                    true,
		"/srv/wikipedia/":          false,
		"/srv//wikipedia":          false,
		"/mnt/data/wikipedia/":     false,
		"/mnt//data//wikipedia":    false,
		"/mnt//c//Windows":         true,
		`C:\Windows\`:              true,
		`C:/Windows//wiki/`:        true,
		`D:\Wikipedia\`:            false,
		`C:\Users\andi\Wikipedia\`: false,
		// Surrounding whitespace is ignored.
		"  /etc/wiki  ":      true,
		"  /srv/wikipedia  ": false,
		// Relative input is never sensitive; callers check IsAbs themselves.
		"":                false,
		".":               false,
		"wikipedia":       false,
		"./wiki":          false,
		"../etc":          false,
		`data\wiki`:       false,
		"etc":             false,
		"mnt/c/Windows":   false,
		"usr/share/wiki":  false,
		"Library/wiki":    false,
		`..\Windows\wiki`: false,
	} {
		if got := IsSensitiveHostDirectory(dir); got != want {
			t.Fatalf("IsSensitiveHostDirectory(%q) = %v, want %v", dir, got, want)
		}
	}
}
