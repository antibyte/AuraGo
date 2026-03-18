// Package tools – dns_lookup: DNS record lookups using the standard library.
package tools

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// dnsRecord represents a single DNS record in the response.
type dnsRecord struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Priority int    `json:"priority,omitempty"` // MX only
}

// dnsResult is the JSON payload returned by DNSLookup.
type dnsResult struct {
	Status     string      `json:"status"`
	Host       string      `json:"host"`
	RecordType string      `json:"record_type"`
	Records    []dnsRecord `json:"records,omitempty"`
	Message    string      `json:"message,omitempty"`
}

// DNSLookup performs DNS record lookups for the given host.
// recordType can be: "all", "A", "AAAA", "MX", "NS", "TXT", "CNAME", "PTR".
func DNSLookup(host, recordType string) string {
	encode := func(r dnsResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if host == "" {
		return encode(dnsResult{Status: "error", Message: "host is required"})
	}
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	if recordType == "" {
		recordType = "ALL"
	}

	result := dnsResult{
		Status:     "success",
		Host:       host,
		RecordType: recordType,
	}

	var records []dnsRecord
	var errs []string

	lookups := map[string]bool{
		"A":     recordType == "ALL" || recordType == "A",
		"AAAA":  recordType == "ALL" || recordType == "AAAA",
		"MX":    recordType == "ALL" || recordType == "MX",
		"NS":    recordType == "ALL" || recordType == "NS",
		"TXT":   recordType == "ALL" || recordType == "TXT",
		"CNAME": recordType == "ALL" || recordType == "CNAME",
		"PTR":   recordType == "PTR",
	}

	if lookups["A"] || lookups["AAAA"] {
		ips, err := net.LookupIP(host)
		if err != nil {
			errs = append(errs, fmt.Sprintf("IP lookup: %v", err))
		} else {
			for _, ip := range ips {
				if ip.To4() != nil && lookups["A"] {
					records = append(records, dnsRecord{Type: "A", Value: ip.String()})
				} else if ip.To4() == nil && lookups["AAAA"] {
					records = append(records, dnsRecord{Type: "AAAA", Value: ip.String()})
				}
			}
		}
	}

	if lookups["MX"] {
		mxRecords, err := net.LookupMX(host)
		if err != nil {
			errs = append(errs, fmt.Sprintf("MX lookup: %v", err))
		} else {
			for _, mx := range mxRecords {
				records = append(records, dnsRecord{
					Type:     "MX",
					Value:    strings.TrimSuffix(mx.Host, "."),
					Priority: int(mx.Pref),
				})
			}
		}
	}

	if lookups["NS"] {
		nsRecords, err := net.LookupNS(host)
		if err != nil {
			errs = append(errs, fmt.Sprintf("NS lookup: %v", err))
		} else {
			for _, ns := range nsRecords {
				records = append(records, dnsRecord{Type: "NS", Value: strings.TrimSuffix(ns.Host, ".")})
			}
		}
	}

	if lookups["TXT"] {
		txtRecords, err := net.LookupTXT(host)
		if err != nil {
			errs = append(errs, fmt.Sprintf("TXT lookup: %v", err))
		} else {
			for _, txt := range txtRecords {
				records = append(records, dnsRecord{Type: "TXT", Value: txt})
			}
		}
	}

	if lookups["CNAME"] {
		cname, err := net.LookupCNAME(host)
		if err != nil {
			errs = append(errs, fmt.Sprintf("CNAME lookup: %v", err))
		} else {
			cleaned := strings.TrimSuffix(cname, ".")
			if cleaned != host {
				records = append(records, dnsRecord{Type: "CNAME", Value: cleaned})
			}
		}
	}

	if lookups["PTR"] {
		names, err := net.LookupAddr(host)
		if err != nil {
			errs = append(errs, fmt.Sprintf("PTR lookup: %v", err))
		} else {
			for _, name := range names {
				records = append(records, dnsRecord{Type: "PTR", Value: strings.TrimSuffix(name, ".")})
			}
		}
	}

	result.Records = records
	if len(records) == 0 && len(errs) > 0 {
		result.Status = "error"
		result.Message = strings.Join(errs, "; ")
	} else if len(errs) > 0 {
		result.Message = "partial results: " + strings.Join(errs, "; ")
	}

	return encode(result)
}
