package dockerutil

import (
	"errors"
	"testing"
)

const (
	testSelfContainerID  = "4f6c1d0b9a2e8c7f53e1a0b2c4d6e8f0a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1"
	testOtherContainerID = "9e8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d"
)

func procFixture(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if text, ok := files[path]; ok {
			return []byte(text), nil
		}
		return nil, errors.New("no such file")
	}
}

func TestOwnContainerIDReadsMountinfoThenCgroupV1(t *testing.T) {
	t.Parallel()

	standard := "1520 1351 0:132 / / rw - overlay overlay rw\n" +
		"1531 1520 8:1 /var/lib/docker/containers/" + testSelfContainerID + "/resolv.conf /etc/resolv.conf rw - ext4 /dev/sda1 rw\n" +
		"1532 1520 8:1 /var/lib/docker/containers/" + testSelfContainerID + "/hostname /etc/hostname rw - ext4 /dev/sda1 rw\n" +
		"1533 1520 8:1 /var/lib/docker/containers/" + testSelfContainerID + "/hosts /etc/hosts rw - ext4 /dev/sda1 rw\n" +
		"1535 1520 8:1 /var/lib/docker/containers/" + testOtherContainerID + "/hostname /mnt/other-hostname ro - ext4 /dev/sda1 rw\n"
	for name, tc := range map[string]struct {
		files map[string]string
		want  string
	}{
		"standard /etc mounts":            {map[string]string{"/proc/self/mountinfo": standard, "/proc/self/cgroup": "0::/\n"}, testSelfContainerID},
		"dedicated containers filesystem": {map[string]string{"/proc/self/mountinfo": "1532 1520 8:17 /" + testSelfContainerID + "/hostname /etc/hostname rw - ext4 /dev/sdb1 rw\n"}, testSelfContainerID},
		"cgroup v1 cgroupfs":              {map[string]string{"/proc/self/cgroup": "12:memory:/docker/" + testSelfContainerID + "\n1:name=systemd:/docker/" + testSelfContainerID + "\n"}, testSelfContainerID},
		"cgroup v1 systemd":               {map[string]string{"/proc/self/cgroup": "9:pids:/system.slice/docker-" + testSelfContainerID + ".scope\n"}, testSelfContainerID},
		"no proc files":                   {nil, ""},
		"cgroup v2 private namespace":     {map[string]string{"/proc/self/mountinfo": "1520 1351 0:132 / / rw - overlay overlay rw\n", "/proc/self/cgroup": "0::/\n"}, ""},
		"foreign file outside /etc only":  {map[string]string{"/proc/self/mountinfo": "1535 1520 8:1 /var/lib/docker/containers/" + testOtherContainerID + "/hostname /mnt/other-hostname ro - ext4 /dev/sda1 rw\n"}, ""},
		"conflicting /etc mounts": {map[string]string{"/proc/self/mountinfo": "1531 1520 8:1 /var/lib/docker/containers/" + testSelfContainerID + "/hostname /etc/hostname rw - ext4 /dev/sda1 rw\n" +
			"1532 1520 8:1 /var/lib/docker/containers/" + testOtherContainerID + "/hosts /etc/hosts rw - ext4 /dev/sda1 rw\n"}, ""},
		"conflicting cgroup lines": {map[string]string{"/proc/self/cgroup": "12:memory:/docker/" + testSelfContainerID + "\n11:pids:/docker/" + testOtherContainerID + "\n"}, ""},
		"short ID":                 {map[string]string{"/proc/self/mountinfo": "1531 1520 8:1 /var/lib/docker/containers/4f6c1d0b9a2e/hostname /etc/hostname rw - ext4 /dev/sda1 rw\n"}, ""},
	} {
		if got := OwnContainerID(procFixture(tc.files)); got != tc.want {
			t.Errorf("%s: OwnContainerID = %q, want %q", name, got, tc.want)
		}
	}
}

func TestDefaultContainerHostnameAcceptsOnlyContainerIDPrefixes(t *testing.T) {
	t.Parallel()

	for hostname, want := range map[string]string{
		"0123456789ab":                  "0123456789ab",
		" 0123456789AB ":                "0123456789ab",
		testSelfContainerID:             testSelfContainerID,
		"0123456789a":                   "",
		testSelfContainerID + "0":       "",
		"aurago":                        "",
		"lxc-host":                      "",
		"":                              "",
		"0123456789ag":                  "",
		"0123456789ab.example.internal": "",
	} {
		if got := DefaultContainerHostname(hostname); got != want {
			t.Errorf("DefaultContainerHostname(%q) = %q, want %q", hostname, got, want)
		}
	}
}
