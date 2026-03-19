// Package fritzbox – Smart Home service calls via AHA-HTTP API.
// Covers: device list, switches, heating thermostats, blinds/rollershutters, lamps/DECT-ULE lights.
// AHA-HTTP uses SID-based GET requests to /webservices/homeautoswitch.lua.
// Device names and states are external data – must be wrapped in <external_data> before LLM use.
package fritzbox

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// SmartHomeDevice represents a Fritz!Box smart home device (DECT-ULE or ZigBee).
type SmartHomeDevice struct {
	AIN             string `json:"ain"` // actor identification number (unique ID)
	Name            string `json:"name"` // device name (external – wrap in <external_data>)
	ProductName     string `json:"product_name"`
	Manufacturer    string `json:"manufacturer"`
	FirmwareVersion string `json:"firmware_version"`
	Present         bool   `json:"present"`

	// Power/switch state (nil if device has no switch)
	SwitchState *bool `json:"switch_state,omitempty"`

	// Temperature sensor (nil if not available)
	Temperature *float64 `json:"temperature,omitempty"` // in °C (Fritz!Box reports in units of 0.1°C)

	// Heating (DECT 301 etc.)
	SetTemp    *float64 `json:"set_temp,omitempty"`    // target temperature in °C
	ActualTemp *float64 `json:"actual_temp,omitempty"` // current temperature in °C

	// Blind/roller shutter (nil if not available)
	BlindPosition *int `json:"blind_position,omitempty"` // 0–100 %

	// DECT-ULE lamp (nil if not a lamp)
	LampBrightness *int    `json:"lamp_brightness,omitempty"`
	LampColor      *string `json:"lamp_color,omitempty"`
}

// deviceListXML is the top-level XML from AHA getdevicelistinfos.
type deviceListXML struct {
	XMLName xml.Name    `xml:"devicelist"`
	Devices []deviceXML `xml:"device"`
}

type deviceXML struct {
	AIN             string `xml:"identifier,attr"`
	ProductName     string `xml:"productname,attr"`
	Manufacturer    string `xml:"manufacturer,attr"`
	FirmwareVersion string `xml:"fwversion,attr"`
	Present         string `xml:"present,attr"`
	Name            string `xml:"name"`

	Switch *struct {
		State string `xml:"state"`
	} `xml:"switch"`

	Temperature *struct {
		Celsius string `xml:"celsius"`
	} `xml:"temperature"`

	Hkr *struct {
		Tsoll string `xml:"tsoll"` // target temp in 0.5°C steps starting from 8°C
		Tist  string `xml:"tist"`  // actual temp in 0.1°C
	} `xml:"hkr"`

	Blind *struct {
		Position string `xml:"endpositionsset"` // not standard – position varies by model
	} `xml:"blind"`

	SimpleonOff *struct {
		State string `xml:"state"`
	} `xml:"simpleonoff"`

	LevelControl *struct {
		Level string `xml:"level"` // 0–255
	} `xml:"levelcontrol"`

	ColorControl *struct {
		ColorTemperature string `xml:"colortemperature"`
	} `xml:"colorcontrol"`
}

// GetSmartHomeDevices returns all smart home devices from the Fritz!Box.
func (c *Client) GetSmartHomeDevices() ([]SmartHomeDevice, error) {
	raw, err := c.AHA("", "getdevicelistinfos", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox smarthome: getdevicelistinfos: %w", err)
	}

	var doc deviceListXML
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("fritzbox smarthome: parse device list: %w", err)
	}

	devices := make([]SmartHomeDevice, 0, len(doc.Devices))
	for _, d := range doc.Devices {
		dev := SmartHomeDevice{
			AIN:             strings.TrimSpace(d.AIN),
			Name:            d.Name,
			ProductName:     d.ProductName,
			Manufacturer:    d.Manufacturer,
			FirmwareVersion: d.FirmwareVersion,
			Present:         d.Present == "1",
		}

		if d.Switch != nil {
			state := d.Switch.State == "1"
			dev.SwitchState = &state
		}
		if d.SimpleonOff != nil {
			state := d.SimpleonOff.State == "1"
			dev.SwitchState = &state
		}
		if d.Temperature != nil {
			if v, err := strconv.ParseFloat(d.Temperature.Celsius, 64); err == nil {
				t := v / 10.0
				dev.Temperature = &t
			}
		}
		if d.Hkr != nil {
			if v, err := strconv.ParseFloat(d.Hkr.Tist, 64); err == nil {
				t := v / 10.0
				dev.ActualTemp = &t
			}
			if v, err := strconv.ParseFloat(d.Hkr.Tsoll, 64); err == nil {
				// Fritz!Box HKR target: 16=8°C, 56=28°C, 253=OFF, 254=ON
				var setTemp float64
				switch d.Hkr.Tsoll {
				case "253":
					setTemp = 0 // OFF
				case "254":
					setTemp = 30 // ON = max heating
				default:
					setTemp = (v / 2.0) + 8.0
				}
				dev.SetTemp = &setTemp
			}
		}
		if d.LevelControl != nil {
			if v, err := strconv.Atoi(d.LevelControl.Level); err == nil {
				pct := (v * 100) / 255
				dev.LampBrightness = &pct
			}
		}
		if d.ColorControl != nil && d.ColorControl.ColorTemperature != "" {
			dev.LampColor = &d.ColorControl.ColorTemperature
		}

		devices = append(devices, dev)
	}
	return devices, nil
}

// SetSwitch turns a switch or smart plug on or off.
// Blocked when ReadOnly is true.
func (c *Client) SetSwitch(ain string, on bool) error {
	if c.SmartHomeReadOnly() {
		return fmt.Errorf("fritzbox smarthome: switch toggle blocked (readonly mode)")
	}
	cmd := "setswitchoff"
	if on {
		cmd = "setswitchon"
	}
	_, err := c.AHA(ain, cmd, nil)
	if err != nil {
		return fmt.Errorf("fritzbox smarthome: %s %s: %w", cmd, ain, err)
	}
	return nil
}

// SetHeatingTarget sets the target temperature for a DECT 301/302 heating actuator.
// temp: 8.0–28.0 °C (0 = OFF, 30 = ON/boost).
// Blocked when ReadOnly is true.
func (c *Client) SetHeatingTarget(ain string, tempC float64) error {
	if c.SmartHomeReadOnly() {
		return fmt.Errorf("fritzbox smarthome: heating control blocked (readonly mode)")
	}
	var ntempkz string
	switch {
	case tempC <= 0:
		ntempkz = "253" // OFF
	case tempC >= 30:
		ntempkz = "254" // ON
	default:
		// Fritz!Box: (temp - 8) * 2 + 16, step 0.5°C
		v := int((tempC-8)*2) + 16
		if v < 16 {
			v = 16
		}
		if v > 56 {
			v = 56
		}
		ntempkz = strconv.Itoa(v)
	}
	_, err := c.AHA(ain, "sethkrtsoll", map[string]string{"param": ntempkz})
	if err != nil {
		return fmt.Errorf("fritzbox smarthome: sethkrtsoll %s: %w", ain, err)
	}
	return nil
}

// SetLampBrightness sets the brightness of a DECT-ULE lamp (0–100 %).
// Blocked when ReadOnly is true.
func (c *Client) SetLampBrightness(ain string, pct int) error {
	if c.SmartHomeReadOnly() {
		return fmt.Errorf("fritzbox smarthome: lamp control blocked (readonly mode)")
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	level := (pct * 255) / 100
	_, err := c.AHA(ain, "setlevelpercentage", map[string]string{"param": strconv.Itoa(pct)})
	_ = level // strconv used to validate range, level not passed (AHA uses percentage directly)
	if err != nil {
		return fmt.Errorf("fritzbox smarthome: setlevelpercentage %s: %w", ain, err)
	}
	return nil
}

// GetSmartHomeTemplates returns the names of all Fritz!Box smart home templates.
func (c *Client) GetSmartHomeTemplates() ([]string, error) {
	raw, err := c.AHA("", "gettemplatelistinfos", nil)
	if err != nil {
		return nil, fmt.Errorf("fritzbox smarthome: gettemplatelistinfos: %w", err)
	}
	// Parse template names from XML <templatelist><template name="...">
	type templateList struct {
		XMLName   xml.Name `xml:"templatelist"`
		Templates []struct {
			Name string `xml:"name,attr"`
		} `xml:"template"`
	}
	var list templateList
	if err := xml.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("fritzbox smarthome: parse templates: %w", err)
	}
	names := make([]string, 0, len(list.Templates))
	for _, t := range list.Templates {
		names = append(names, t.Name)
	}
	return names, nil
}

// ApplySmartHomeTemplate applies a named template.
// Blocked when ReadOnly is true.
func (c *Client) ApplySmartHomeTemplate(templateID string) error {
	if c.SmartHomeReadOnly() {
		return fmt.Errorf("fritzbox smarthome: template apply blocked (readonly mode)")
	}
	_, err := c.AHA(templateID, "applytemplate", nil)
	if err != nil {
		return fmt.Errorf("fritzbox smarthome: applytemplate %s: %w", templateID, err)
	}
	return nil
}
