//go:build linux

package rtlsdr

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Devices enumerates USB nodes without opening or claiming an interface.
func Devices() []Device {
	result := []Device{}
	entries, _ := filepath.Glob("/sys/bus/usb/devices/*")
	for _, path := range entries {
		read := func(name string) string {
			b, _ := os.ReadFile(filepath.Join(path, name))
			return strings.TrimSpace(string(b))
		}
		vendor, product := read("idVendor"), read("idProduct")
		if vendor != "0bda" || (product != "2832" && product != "2838") {
			continue
		}
		bus, e1 := strconv.Atoi(read("busnum"))
		number, e2 := strconv.Atoi(read("devnum"))
		if e1 != nil || e2 != nil {
			continue
		}
		node := fmt.Sprintf("/dev/bus/usb/%03d/%03d", bus, number)
		d := Device{ID: filepath.Base(path), Name: read("product"), Serial: read("serial"), Node: node, Vendor: vendor, Product: product}
		driver, _ := filepath.EvalSymlinks(path + ":1.0/driver")
		if driver != "" {
			d.Driver = filepath.Base(driver)
		}
		if st, err := os.Stat(node); err == nil {
			if sys, ok := st.Sys().(*syscall.Stat_t); ok {
				d.Group = int(sys.Gid)
				d.GroupWritable = st.Mode().Perm()&0020 != 0
			}
		}
		result = append(result, d)
	}
	return result
}
