package meshcore

import "fmt"

type TelemetryValue struct {
	Channel byte      `json:"channel"`
	Type    byte      `json:"type"`
	Name    string    `json:"name"`
	Unit    string    `json:"unit"`
	Values  []float64 `json:"values"`
}

// Cayenne LPP is big endian, unlike Companion framing. Unknown types cannot be
// skipped safely because they carry no generic length; fail the whole payload.
func decodeTelemetry(b []byte) ([]TelemetryValue, error) {
	result := []TelemetryValue{}
	for len(b) > 0 {
		if len(b) < 3 || len(result) >= 64 {
			return nil, fmt.Errorf("invalid_telemetry")
		}
		v := TelemetryValue{Channel: b[0], Type: b[1]}
		size, count, scale, signed := 1, 1, 1.0, false
		switch v.Type {
		case 0:
			v.Name = "digital_input"
		case 1:
			v.Name = "digital_output"
		case 2, 3:
			v.Name = "analog"
			size = 2
			scale = 100
			signed = true
		case 100:
			v.Name = "generic"
			size = 4
		case 101:
			v.Name = "illuminance"
			v.Unit = "lx"
			size = 2
		case 102:
			v.Name = "presence"
		case 103:
			v.Name = "temperature"
			v.Unit = "°C"
			size = 2
			scale = 10
			signed = true
		case 104:
			v.Name = "humidity"
			v.Unit = "%"
			scale = 2
		case 113:
			v.Name = "acceleration"
			v.Unit = "g"
			size = 2
			count = 3
			scale = 1000
			signed = true
		case 115:
			v.Name = "pressure"
			v.Unit = "hPa"
			size = 2
			scale = 10
		case 116:
			v.Name = "voltage"
			v.Unit = "V"
			size = 2
			scale = 100
		case 117:
			v.Name = "current"
			v.Unit = "A"
			size = 2
			scale = 1000
		case 118:
			v.Name = "frequency"
			v.Unit = "Hz"
			size = 4
		case 120:
			v.Name = "percentage"
			v.Unit = "%"
		case 121:
			v.Name = "altitude"
			v.Unit = "m"
			size = 2
			signed = true
		case 125:
			v.Name = "concentration"
			v.Unit = "ppm"
			size = 2
		case 128:
			v.Name = "power"
			v.Unit = "W"
			size = 2
		case 130:
			v.Name = "distance"
			v.Unit = "m"
			size = 4
			scale = 1000
		case 131:
			v.Name = "energy"
			v.Unit = "kWh"
			size = 4
			scale = 1000
		case 132:
			v.Name = "direction"
			v.Unit = "°"
			size = 2
		case 133:
			v.Name = "time"
			v.Unit = "s"
			size = 4
		case 134:
			v.Name = "rotation"
			v.Unit = "°/s"
			size = 2
			count = 3
			scale = 100
			signed = true
		case 135:
			v.Name = "colour"
			v.Unit = "RGB"
			count = 3
		case 136:
			v.Name = "position"
			v.Unit = "°, °, m"
			size = 3
			count = 3
			scale = 10000
			signed = true
		case 142:
			v.Name = "switch"
		default:
			return nil, fmt.Errorf("unsupported_telemetry")
		}
		b = b[2:]
		if len(b) < size*count {
			return nil, fmt.Errorf("invalid_telemetry")
		}
		for i := 0; i < count; i++ {
			n := int64(0)
			for _, x := range b[i*size : (i+1)*size] {
				n = n<<8 | int64(x)
			}
			if signed && b[i*size]&0x80 != 0 {
				n -= 1 << uint(size*8)
			}
			divisor := scale
			if v.Type == 136 && i == 2 {
				divisor = 100
			}
			v.Values = append(v.Values, float64(n)/divisor)
		}
		if (v.Type == 136 && (v.Values[0] < -90 || v.Values[0] > 90 || v.Values[1] < -180 || v.Values[1] > 180)) || ((v.Type == 104 || v.Type == 120) && v.Values[0] > 100) || ((v.Type == 102 || v.Type == 142) && v.Values[0] > 1) {
			return nil, fmt.Errorf("invalid_telemetry")
		}
		b = b[size*count:]
		result = append(result, v)
	}
	return result, nil
}
