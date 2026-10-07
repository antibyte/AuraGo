package desktop

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateSerialProfiles(t *testing.T) {
	validHost := `{"version":1,"profiles":[{"id":"host-1","name":"Console","source":"host","port":"COM1","options":{}}]}`
	validBrowser := `{"version":1,"profiles":[{"id":"browser_1","name":"USB adapter","source":"browser","port":"","usb_vendor_id":4292,"usb_product_id":60000,"options":{"flow_control":"hardware"}}]}`
	if err := validateSerialProfiles(validHost); err != nil {
		t.Fatalf("valid host profile: %v", err)
	}
	if err := validateSerialProfiles(validBrowser); err != nil {
		t.Fatalf("valid browser profile: %v", err)
	}
	for name, value := range map[string]string{
		"zero product ID": `{"version":1,"profiles":[{"id":"pid-zero","name":"USB adapter","source":"browser","port":"","usb_vendor_id":4292,"usb_product_id":0,"options":{}}]}`,
		"zero vendor ID":  `{"version":1,"profiles":[{"id":"vid-zero","name":"USB adapter","source":"browser","port":"","usb_vendor_id":0,"usb_product_id":60000,"options":{}}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateSerialProfiles(value); err != nil {
				t.Fatalf("valid USB ID filter rejected: %v", err)
			}
		})
	}

	invalid := []struct {
		name  string
		value string
	}{
		{"duplicate ids", `{"version":1,"profiles":[{"id":"same","name":"One","source":"host","port":"COM1","options":{}},{"id":"same","name":"Two","source":"host","port":"COM2","options":{}}]}`},
		{"browser host path", `{"version":1,"profiles":[{"id":"browser","name":"Browser","source":"browser","port":"COM1","options":{}}]}`},
		{"host without path", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"","options":{}}]}`},
		{"host USB filter", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"COM1","usb_vendor_id":1,"options":{}}]}`},
		{"product without vendor", `{"version":1,"profiles":[{"id":"browser","name":"Browser","source":"browser","port":"","usb_product_id":1,"options":{}}]}`},
		{"null baud", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"COM1","options":{"baud_rate":null}}]}`},
		{"null RTS", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"COM1","options":{"rts":null}}]}`},
		{"invalid option type", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"COM1","options":{"baud_rate":"fast"}}]}`},
		{"unknown option", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"COM1","options":{"flow":"hardware"}}]}`},
		{"unknown profile field", `{"version":1,"profiles":[{"id":"host","name":"Host","source":"host","port":"COM1","extra":true,"options":{}}]}`},
		{"trailing JSON", validHost + `{}`},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSerialProfiles(tc.value); err == nil {
				t.Fatal("expected invalid profile document")
			}
		})
	}
}

func TestSerialProfilesEnforceCountAndSettingRoute(t *testing.T) {
	profiles := func(count int) string {
		var value strings.Builder
		value.WriteString(`{"version":1,"profiles":[`)
		for i := 0; i < count; i++ {
			if i > 0 {
				value.WriteByte(',')
			}
			fmt.Fprintf(&value, `{"id":"p%d","name":"Device","source":"host","port":"COM1","options":{}}`, i)
		}
		value.WriteString(`]}`)
		return value.String()
	}
	if err := validateSerialProfiles(profiles(64)); err != nil {
		t.Fatalf("64 profiles rejected: %v", err)
	}
	if err := validateSerialProfiles(profiles(65)); err == nil {
		t.Fatal("expected more than 64 profiles to be rejected")
	}
	if err := validateFreeformDesktopSetting(SerialProfilesSetting, validSerialProfileFixture()); err != nil {
		t.Fatalf("setting validation did not route to serial profile validator: %v", err)
	}
}

func validSerialProfileFixture() string {
	return `{"version":1,"profiles":[{"id":"one","name":"Device","source":"host","port":"COM1","options":{}}]}`
}
