package findit

import "sort"

// Catalog returns the stable built-in categories, units, package families and templates.
func Catalog() CatalogData {
	text := func(key, label string, required bool) PropertyDefinition {
		return PropertyDefinition{Key: key, Label: label, Type: "text", Required: required}
	}
	date := func(key, label string) PropertyDefinition {
		return PropertyDefinition{Key: key, Label: label, Type: "date"}
	}
	number := func(key, label, unit string) PropertyDefinition {
		return PropertyDefinition{Key: key, Label: label, Type: "number", Unit: unit}
	}
	enum := func(key, label string, options ...string) PropertyDefinition {
		return PropertyDefinition{Key: key, Label: label, Type: "enum", Options: options}
	}
	boolean := func(key, label string) PropertyDefinition {
		return PropertyDefinition{Key: key, Label: label, Type: "boolean"}
	}
	template := func(id, name, description, category string, fields ...PropertyDefinition) Template {
		return Template{ID: id, IconID: id, Name: name, Description: description, Category: category, BuiltIn: true, Version: 1, Fields: fields}
	}

	data := CatalogData{
		Version: 1,
		Categories: []CategoryDefinition{
			{ID: "electronic_component", Name: "Electronic components", Group: "electronics"},
			{ID: "pc_component", Name: "PC components", Group: "computers"},
			{ID: "networking", Name: "Networking", Group: "homelab"},
			{ID: "server", Name: "Servers and storage", Group: "homelab"},
			{ID: "cable", Name: "Cables and adapters", Group: "supplies"},
			{ID: "tool", Name: "Tools", Group: "supplies"},
			{ID: "consumable", Name: "Consumables", Group: "supplies"},
			{ID: "development", Name: "Development boards and radios", Group: "electronics"},
			{ID: "instruments", Name: "Meters and instruments", Group: "tools"},
			{ID: "battery", Name: "Batteries and power cells", Group: "supplies"},
			{ID: "fastener", Name: "Fasteners and hardware", Group: "supplies"},
			{ID: "other", Name: "Other", Group: "general"},
		},
		Units: []UnitDefinition{
			{ID: "piece", Name: "Piece", Symbol: "pcs"},
			{ID: "m", Name: "Metre", Symbol: "m"},
			{ID: "cm", Name: "Centimetre", Symbol: "cm"},
			{ID: "mm", Name: "Millimetre", Symbol: "mm"},
			{ID: "kg", Name: "Kilogram", Symbol: "kg"},
			{ID: "g", Name: "Gram", Symbol: "g"},
			{ID: "l", Name: "Litre", Symbol: "L"},
			{ID: "ml", Name: "Millilitre", Symbol: "mL"},
		},
		PackageFamilies: []PackageFamily{
			{ID: "chip_resistor", Name: "Chip resistor", Description: "Surface-mount resistor size family"},
			{ID: "chip_capacitor", Name: "Chip capacitor", Description: "Surface-mount capacitor size family"},
			{ID: "01005", Name: "01005 / 0402 metric"}, {ID: "0201", Name: "0201 / 0603 metric"},
			{ID: "0402", Name: "0402 / 1005 metric"}, {ID: "0603", Name: "0603 / 1608 metric"},
			{ID: "0805", Name: "0805 / 2012 metric"}, {ID: "1206", Name: "1206 / 3216 metric"},
			{ID: "1210", Name: "1210 / 3225 metric"}, {ID: "1812", Name: "1812 / 4532 metric"}, {ID: "2512", Name: "2512 / 6432 metric"},
			{ID: "dip", Name: "DIP"}, {ID: "sip", Name: "SIP"}, {ID: "soic", Name: "SOIC"}, {ID: "sop", Name: "SOP"},
			{ID: "ssop", Name: "SSOP"}, {ID: "tssop", Name: "TSSOP"}, {ID: "msop", Name: "MSOP"}, {ID: "qfp", Name: "QFP"},
			{ID: "lqfp", Name: "LQFP"}, {ID: "tqfp", Name: "TQFP"}, {ID: "qfn", Name: "QFN"}, {ID: "dfn", Name: "DFN"},
			{ID: "bga", Name: "BGA"}, {ID: "lga", Name: "LGA"}, {ID: "plcc", Name: "PLCC"},
			{ID: "sot23", Name: "SOT-23"}, {ID: "sc70", Name: "SC-70"}, {ID: "sot323", Name: "SOT-323 / SC-70"}, {ID: "sot223", Name: "SOT-223"},
			{ID: "sod323", Name: "SOD-323"}, {ID: "sod523", Name: "SOD-523"}, {ID: "sod123", Name: "SOD-123"},
			{ID: "to92", Name: "TO-92"}, {ID: "to220", Name: "TO-220"}, {ID: "to252_dpak", Name: "TO-252 / DPAK"},
			{ID: "to263_d2pak", Name: "TO-263 / D2PAK"}, {ID: "axial", Name: "Axial lead"}, {ID: "radial", Name: "Radial lead"},
			{ID: "through_hole", Name: "Through-hole"},
			{ID: "smd", Name: "Surface mount"}, {ID: "m2", Name: "M.2"}, {ID: "2_5_inch", Name: "2.5-inch"},
			{ID: "3_5_inch", Name: "3.5-inch"}, {ID: "dimm", Name: "DIMM"}, {ID: "so_dimm", Name: "SO-DIMM"},
			{ID: "atx", Name: "ATX"}, {ID: "micro_atx", Name: "Micro-ATX"}, {ID: "mini_itx", Name: "Mini-ITX"},
			{ID: "pcie", Name: "PCI Express"}, {ID: "rj45", Name: "RJ45"}, {ID: "sfp", Name: "SFP / SFP+"},
			{ID: "usb_c", Name: "USB Type-C"}, {ID: "jst", Name: "JST"}, {ID: "dupont", Name: "Dupont"},
		},
		Templates: []Template{
			template("generic", "Generic item", "General homelab item with custom properties", "other", text("reference", "Reference", false), text("revision", "Revision", false)),
			template("electronic_component", "Electronic component", "General electronic part", "electronic_component", text("value", "Value", false), number("voltage_v", "Voltage", "V"), number("current_a", "Current", "A"), number("power_w", "Power", "W"), number("tolerance_pct", "Tolerance", "%"), text("technology", "Technology", false)),
			template("resistor", "Resistor", "Fixed and variable resistors", "electronic_component", number("resistance_ohms", "Resistance", "Ω"), number("tolerance_pct", "Tolerance", "%"), number("power_w", "Power rating", "W"), number("voltage_v", "Voltage rating", "V"), enum("technology", "Technology", "thick_film", "thin_film", "wirewound", "carbon", "metal_foil", "potentiometer")),
			template("capacitor", "Capacitor", "Ceramic, film, electrolytic and other capacitors", "electronic_component", number("capacitance_f", "Capacitance", "F"), number("voltage_v", "Voltage rating", "V"), number("tolerance_pct", "Tolerance", "%"), enum("dielectric", "Dielectric", "ceramic", "film", "aluminium_electrolytic", "tantalum", "polymer", "mica"), boolean("polarized", "Polarized")),
			template("inductor", "Inductor", "Inductors and chokes", "electronic_component", number("inductance_h", "Inductance", "H"), number("current_a", "Rated current", "A"), number("resistance_ohms", "DC resistance", "Ω"), number("tolerance_pct", "Tolerance", "%")),
			template("diode", "Diode", "Rectifier, signal, Zener and LED diodes", "electronic_component", enum("diode_type", "Diode type", "rectifier", "signal", "zener", "schottky", "led", "photodiode", "tvS"), number("voltage_v", "Reverse voltage", "V"), number("current_a", "Forward current", "A"), number("forward_voltage_v", "Forward voltage", "V")),
			template("transistor", "Transistor", "BJT, MOSFET, JFET and IGBT devices", "electronic_component", enum("transistor_type", "Type", "bjt_npn", "bjt_pnp", "mosfet_n", "mosfet_p", "jfet_n", "jfet_p", "igbt"), number("voltage_v", "Voltage rating", "V"), number("current_a", "Current rating", "A"), number("power_w", "Power rating", "W")),
			template("integrated_circuit", "Integrated circuit", "Logic, analog, MCU and interface ICs", "electronic_component", text("function", "Function", false), number("supply_voltage_v", "Supply voltage", "V"), number("clock_mhz", "Clock speed", "MHz"), enum("ic_family", "Family", "logic", "analog", "microcontroller", "memory", "interface", "power", "radio", "fpga")),
			template("connector", "Connector", "Board and cable connectors", "electronic_component", text("series", "Series", false), number("positions", "Positions", ""), number("rows", "Rows", ""), number("pitch_mm", "Pitch", "mm"), enum("mounting", "Mounting", "through_hole", "smd", "panel", "edge")),
			template("pcb", "PCB", "Printed circuit boards", "electronic_component", number("layers", "Layers", ""), number("length_mm", "Length", "mm"), number("width_mm", "Width", "mm"), enum("finish", "Finish", "hasl", "enig", "immersion_silver", "bare_copper")),
			template("cable", "Cable", "Power, data and RF cables", "cable", enum("cable_type", "Cable type", "ethernet", "usb", "power", "sata", "hdmi", "displayport", "coax", "fiber", "other"), text("connector_a", "Connector A", false), text("connector_b", "Connector B", false), number("length_m", "Length", "m"), text("rating", "Rating", false)),
			template("cpu", "CPU", "Desktop, workstation and server processors", "pc_component", text("socket", "Socket", false), number("cores", "Cores", ""), number("threads", "Threads", ""), number("base_clock_ghz", "Base clock", "GHz"), number("tdp_w", "TDP", "W"), text("architecture", "Architecture", false)),
			template("motherboard", "Motherboard", "Desktop and server motherboards", "pc_component", text("socket", "Socket", false), enum("form_factor", "Form factor", "ATX", "Micro-ATX", "Mini-ITX", "E-ATX", "SSI-CEB", "SSI-EEB"), text("memory_type", "Memory type", false), number("memory_slots", "Memory slots", ""), number("pcie_slots", "PCIe slots", "")),
			template("memory", "Memory", "DIMM and SO-DIMM memory modules", "pc_component", number("capacity_gb", "Capacity", "GB"), text("memory_type", "Memory type", false), text("form_factor", "Form factor", false), number("speed_mts", "Transfer rate", "MT/s"), boolean("ecc", "ECC"), number("modules", "Module count", "")),
			template("storage", "Storage device", "HDD, SSD, NVMe and removable storage", "pc_component", number("capacity_gb", "Capacity", "GB"), enum("storage_type", "Storage type", "hdd", "ssd", "nvme", "usb", "sd"), text("interface", "Interface", false), text("form_factor", "Form factor", false), number("endurance_tbw", "Endurance", "TBW")),
			template("gpu", "GPU", "Graphics and compute accelerators", "pc_component", number("memory_gb", "Memory", "GB"), text("interface", "Interface", false), number("length_mm", "Length", "mm"), number("tdp_w", "Board power", "W"), text("architecture", "Architecture", false)),
			template("power_supply", "Power supply", "ATX, SFX and server power supplies", "pc_component", number("power_w", "Rated power", "W"), text("form_factor", "Form factor", false), text("efficiency", "Efficiency rating", false), text("modularity", "Modularity", false)),
			template("pc_case", "PC case", "Desktop and rack-mount enclosures", "pc_component", text("form_factor", "Form factor", false), number("rack_units", "Rack units", "U"), number("length_mm", "Length", "mm"), number("width_mm", "Width", "mm"), number("height_mm", "Height", "mm")),
			template("cpu_cooler", "CPU cooler", "Air and liquid CPU coolers", "pc_component", number("height_mm", "Height", "mm"), number("radiator_mm", "Radiator size", "mm"), text("socket_support", "Socket support", false), number("tdp_w", "Cooling capacity", "W")),
			template("network_card", "Network adapter", "Ethernet, Wi-Fi and fibre adapters", "pc_component", text("interface", "Interface", false), number("ports", "Ports", ""), number("speed_gbps", "Link speed", "Gb/s"), text("chipset", "Chipset", false)),
			template("server", "Server", "Rack and tower servers", "server", number("rack_units", "Rack units", "U"), text("generation", "Generation", false), number("cpu_sockets", "CPU sockets", ""), number("memory_slots", "Memory slots", ""), text("management", "Management interface", false)),
			template("network_switch", "Network switch", "Managed and unmanaged network switches", "networking", number("ports", "Ports", ""), number("speed_gbps", "Port speed", "Gb/s"), boolean("poe", "PoE"), number("poe_budget_w", "PoE budget", "W"), number("rack_units", "Rack units", "U")),
			template("router", "Router", "Routers, firewalls and gateways", "networking", number("ports", "Ports", ""), text("wan_interface", "WAN interface", false), text("wireless_standards", "Wireless standards", false), boolean("poe", "PoE")),
			template("access_point", "Wireless access point", "Wi-Fi access points", "networking", text("wireless_standards", "Wireless standards", false), number("spatial_streams", "Spatial streams", ""), boolean("poe", "PoE"), text("mounting", "Mounting", false)),
			template("sfp_module", "SFP module", "SFP, SFP+ and QSFP optical/copper modules", "networking", text("media", "Media", false), number("speed_gbps", "Speed", "Gb/s"), number("wavelength_nm", "Wavelength", "nm"), number("reach_km", "Reach", "km")),
			template("ups", "UPS", "Uninterruptible power supplies", "server", number("capacity_va", "Capacity", "VA"), number("capacity_w", "Power", "W"), number("battery_count", "Battery count", ""), text("battery_model", "Battery model", false)),
			template("printer", "3D printer", "3D printers and related hardware", "other", text("process", "Process", false), number("build_volume_l", "Build volume", "L"), text("firmware", "Firmware", false)),
			template("tool", "Tool", "Hand tools and test equipment", "tool", text("tool_type", "Tool type", false), text("range", "Range", false), date("calibration_due", "Calibration due")),
			template("consumable", "Consumable", "Filament, solder, fasteners and general supplies", "consumable", text("material", "Material", false), text("color", "Color", false), number("diameter_mm", "Diameter", "mm"), date("expiry_date", "Expiry date")),
			template("potentiometer", "Potentiometer", "Variable and trimmer resistors", "electronic_component", number("resistance_ohms", "Resistance", "Ω"), number("tolerance_pct", "Tolerance", "%"), number("power_w", "Power rating", "W"), number("turns", "Turns", ""), enum("mounting", "Mounting", "through_hole", "smd", "panel")),
			template("led", "LED", "Indicator, lighting and infrared LEDs", "electronic_component", enum("color", "Color", "red", "green", "blue", "white", "amber", "infrared", "uv", "rgb"), number("forward_voltage_v", "Forward voltage", "V"), number("current_a", "Rated current", "A"), number("wavelength_nm", "Wavelength", "nm")),
			template("mosfet", "MOSFET", "Power and signal MOSFETs", "electronic_component", enum("channel", "Channel", "n", "p"), number("voltage_v", "Drain voltage", "V"), number("current_a", "Drain current", "A"), number("rds_on_ohms", "On resistance", "Ω"), number("power_w", "Power rating", "W")),
			template("op_amp", "Operational amplifier", "Op-amps and instrumentation amplifiers", "electronic_component", number("supply_voltage_v", "Supply voltage", "V"), number("gain_bandwidth_mhz", "Gain bandwidth", "MHz"), number("input_offset_uv", "Input offset", "µV"), enum("input_type", "Input type", "bipolar", "cmos", "jfet"), number("channels", "Channels", ""), number("slew_rate_v_us", "Slew rate", "V/µs"), boolean("rail_to_rail_input", "Rail-to-rail input"), boolean("rail_to_rail_output", "Rail-to-rail output")),
			template("comparator", "Comparator", "Analog comparators", "electronic_component", number("supply_voltage_v", "Supply voltage", "V"), number("propagation_delay_ns", "Propagation delay", "ns"), boolean("open_collector", "Open collector")),
			template("voltage_regulator", "Voltage regulator", "Linear and switching regulators", "electronic_component", enum("regulator_type", "Type", "linear", "buck", "boost", "buck_boost", "ldo"), number("input_voltage_v", "Input voltage", "V"), number("output_voltage_v", "Output voltage", "V"), number("current_a", "Output current", "A")),
			template("logic", "Logic IC", "Logic gates, buffers, counters and shift registers", "electronic_component", text("logic_family", "Logic family", false), number("supply_voltage_v", "Supply voltage", "V"), number("channels", "Channels", "")),
			template("microcontroller", "Microcontroller", "Microcontrollers and embedded processors", "electronic_component", text("architecture", "Architecture", false), number("flash_kb", "Flash", "KB"), number("ram_kb", "RAM", "KB"), number("clock_mhz", "Clock speed", "MHz"), number("gpio_count", "GPIO count", "")),
			template("memory_ic", "Memory IC", "EEPROM, flash, SRAM and DRAM ICs", "electronic_component", enum("memory_type", "Memory type", "eeprom", "flash", "sram", "dram", "fram"), number("capacity_mbit", "Capacity", "Mbit"), number("supply_voltage_v", "Supply voltage", "V"), number("interface_mhz", "Interface speed", "MHz")),
			template("adc", "ADC", "Analog-to-digital converters", "electronic_component", number("resolution_bits", "Resolution", "bits"), number("sample_rate_ksps", "Sample rate", "ksps"), number("input_channels", "Input channels", ""), text("interface", "Interface", false)),
			template("dac", "DAC", "Digital-to-analog converters", "electronic_component", number("resolution_bits", "Resolution", "bits"), number("settling_time_us", "Settling time", "µs"), number("output_channels", "Output channels", ""), text("interface", "Interface", false)),
			template("interface_ic", "Interface IC", "USB, UART, CAN, Ethernet and level-shifting ICs", "electronic_component", text("interface", "Interface", false), number("data_rate_mbps", "Data rate", "Mb/s"), number("channels", "Channels", ""), number("supply_voltage_v", "Supply voltage", "V")),
			template("sensor", "Sensor", "Environmental, motion, current and position sensors", "electronic_component", text("measurement", "Measurement", false), text("interface", "Interface", false), number("range_min", "Range minimum", ""), number("range_max", "Range maximum", ""), text("accuracy", "Accuracy", false)),
			template("optoisolator", "Optocoupler", "Optoisolators and optorelays", "electronic_component", number("isolation_voltage_v", "Isolation voltage", "V"), number("ctr_pct", "Current transfer ratio", "%"), number("channels", "Channels", "")),
			template("relay", "Relay", "Electromechanical and solid-state relays", "electronic_component", enum("relay_type", "Type", "electromechanical", "reed", "solid_state"), number("coil_voltage_v", "Coil voltage", "V"), number("contact_current_a", "Contact current", "A"), number("contact_voltage_v", "Contact voltage", "V"), boolean("latching", "Latching")),
			template("switch", "Switch", "Panel, board and tactile switches", "electronic_component", enum("switch_type", "Type", "tactile", "toggle", "rocker", "slide", "rotary", "limit"), number("poles", "Poles", ""), number("throws", "Throws", ""), number("rated_current_a", "Rated current", "A")),
			template("fuse", "Fuse", "Fuses, resettable fuses and fuse holders", "electronic_component", number("current_a", "Rated current", "A"), number("voltage_v", "Rated voltage", "V"), enum("fuse_type", "Type", "fast", "slow", "resettable"), number("breaking_capacity_a", "Breaking capacity", "A")),
			template("crystal", "Crystal", "Quartz crystals and resonators", "electronic_component", number("frequency_mhz", "Frequency", "MHz"), number("tolerance_ppm", "Tolerance", "ppm"), number("load_capacitance_pf", "Load capacitance", "pF"), number("esr_ohms", "ESR", "Ω")),
			template("oscillator", "Oscillator", "Clock oscillators and timing modules", "electronic_component", number("frequency_mhz", "Frequency", "MHz"), number("supply_voltage_v", "Supply voltage", "V"), number("tolerance_ppm", "Tolerance", "ppm"), text("output_type", "Output type", false)),
			template("display", "Display", "LCD, OLED, LED and e-paper modules", "electronic_component", text("display_type", "Display type", false), number("diagonal_in", "Diagonal", "in"), number("width_px", "Width", "px"), number("height_px", "Height", "px"), text("interface", "Interface", false)),
			template("motor", "Motor", "DC, stepper, servo and brushless motors", "other", enum("motor_type", "Type", "dc", "stepper", "servo", "brushless", "ac"), number("rated_voltage_v", "Rated voltage", "V"), number("rated_current_a", "Rated current", "A"), number("rated_speed_rpm", "Rated speed", "rpm")),
			template("fan", "Fan", "Axial and blower fans", "pc_component", number("size_mm", "Fan size", "mm"), number("voltage_v", "Voltage", "V"), number("airflow_cfm", "Airflow", "CFM"), number("noise_dba", "Noise", "dBA"), number("speed_rpm", "Speed", "rpm")),
			template("development_board", "Development board", "Microcontroller, FPGA and prototyping boards", "development", text("processor", "Processor", false), text("board_family", "Board family", false), text("wireless", "Wireless", false), text("interface", "Programming interface", false)),
			template("radio_module", "Radio module", "Wi-Fi, Bluetooth, LoRa and cellular modules", "development", text("protocol", "Protocol", false), number("frequency_mhz", "Frequency", "MHz"), number("tx_power_dbm", "Transmit power", "dBm"), text("antenna", "Antenna", false)),
			template("single_board_computer", "Single-board computer", "SBCs and compute modules", "development", text("processor", "Processor", false), number("memory_gb", "Memory", "GB"), number("storage_gb", "Storage", "GB"), text("wireless", "Wireless", false), text("io", "I/O", false)),
			template("expansion_card", "Expansion card", "PCIe and system expansion cards", "pc_component", text("interface", "Interface", false), number("ports", "Ports", ""), number("length_mm", "Length", "mm"), text("chipset", "Chipset", false)),
			template("ssd", "SSD", "Solid-state drives and NVMe devices", "pc_component", number("capacity_gb", "Capacity", "GB"), text("interface", "Interface", false), text("form_factor", "Form factor", false), number("endurance_tbw", "Endurance", "TBW"), boolean("nvme", "NVMe")),
			template("hdd", "Hard disk drive", "Mechanical hard drives", "pc_component", number("capacity_gb", "Capacity", "GB"), number("spin_rpm", "Rotation speed", "rpm"), text("interface", "Interface", false), text("form_factor", "Form factor", false), number("cache_mb", "Cache", "MB")),
			template("meter", "Meter", "Multimeters, test meters and probes", "instruments", text("meter_type", "Type", false), number("voltage_max_v", "Maximum voltage", "V"), number("current_max_a", "Maximum current", "A"), text("accuracy", "Accuracy", false), date("calibration_due", "Calibration due")),
			template("battery", "Battery", "Primary and rechargeable batteries", "battery", text("chemistry", "Chemistry", false), number("nominal_voltage_v", "Nominal voltage", "V"), number("capacity_ah", "Capacity", "Ah"), text("form_factor", "Form factor", false), date("expiry_date", "Expiry date")),
			template("fastener", "Fastener", "Screws, nuts, bolts, spacers and clips", "fastener", text("fastener_type", "Type", false), text("thread", "Thread", false), number("length_mm", "Length", "mm"), text("material", "Material", false)),
			template("adapter", "Adapter", "Power, data and mechanical adapters", "cable", text("input_connector", "Input connector", false), text("output_connector", "Output connector", false), text("protocol", "Protocol", false), number("length_m", "Length", "m")),
		},
	}
	for i := range data.Templates {
		sort.Slice(data.Templates[i].Fields, func(a, b int) bool { return data.Templates[i].Fields[a].Key < data.Templates[i].Fields[b].Key })
	}
	sort.Slice(data.PackageFamilies, func(i, j int) bool { return data.PackageFamilies[i].ID < data.PackageFamilies[j].ID })
	return data
}
