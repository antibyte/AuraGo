package decisions

import (
	"fmt"
	"sort"
	"strings"
)

func orderedDevices(devices []runtimeDevice, goos string, cpuAllowed bool) ([]runtimeDevice, error) {
	preferences := runtimeBackends(goos)
	priority := make(map[string]int, len(preferences))
	for index, backend := range preferences {
		priority[backend] = index
	}
	accelerators := make([]runtimeDevice, 0, len(devices))
	var cpuDevices []runtimeDevice
	for _, device := range devices {
		switch strings.ToUpper(device.Type) {
		case "GPU", "IGPU", "ACCEL":
			if strings.EqualFold(device.Backend, "cpu") {
				continue
			}
			accelerators = append(accelerators, device)
		case "CPU":
			if strings.EqualFold(device.Backend, "cpu") {
				cpuDevices = append(cpuDevices, device)
			}
		}
	}
	sort.SliceStable(accelerators, func(i, j int) bool {
		left, right := accelerators[i], accelerators[j]
		leftClass, rightClass := deviceClassPriority(left.Integrated), deviceClassPriority(right.Integrated)
		if leftClass != rightClass {
			return leftClass < rightClass
		}
		leftPriority, leftOK := priority[strings.ToLower(left.Backend)]
		rightPriority, rightOK := priority[strings.ToLower(right.Backend)]
		if leftOK != rightOK {
			return leftOK
		}
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if left.MemoryFree != right.MemoryFree {
			return left.MemoryFree > right.MemoryFree
		}
		return left.ID < right.ID
	})
	if len(accelerators) > 0 {
		if cpuAllowed {
			return append(accelerators, cpuDevices...), nil
		}
		return accelerators, nil
	}
	if cpuAllowed && len(cpuDevices) > 0 {
		return cpuDevices, nil
	}
	if len(cpuDevices) > 0 && !cpuAllowed {
		return nil, NewError(CodeCPUConsentRequired, fmt.Errorf("accelerator unavailable; persistent CPU consent is required"))
	}
	if len(devices) > 0 {
		return nil, NewError(CodeHardwareUnavailable, fmt.Errorf("runtime did not discover a supported GPU or CPU device"))
	}
	return nil, NewError(CodeHardwareUnavailable, fmt.Errorf("runtime discovered no supported devices"))
}

func deviceClassPriority(integrated *bool) int {
	if integrated == nil {
		return 1
	}
	if *integrated {
		return 2
	}
	return 0
}
