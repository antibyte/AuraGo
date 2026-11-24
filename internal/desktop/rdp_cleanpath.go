package desktop

import (
	"encoding/asn1"
	"errors"
	"fmt"
)

// RDCleanPath is the small ASN.1/DER envelope IronRDP's web client uses to talk
// to a gateway before the raw RDP byte stream starts. Field layout follows
// crates/ironrdp-rdcleanpath: every field is an EXPLICIT context tag and every
// string is a UTF8String.

const (
	rdCleanPathVersion = 3390

	// RDCleanPath error codes understood by the IronRDP client.
	rdCleanPathErrGeneral       = 1
	rdCleanPathErrNegotiation   = 2
	rdCleanPathErrTargetRefused = 103
	rdCleanPathErrDNS           = 104
	rdCleanPathErrTLS           = 105
	// rdCleanPathErrCertChanged is an AuraGo-specific code (implementation range)
	// sent with HTTP status 409 when the server certificate no longer matches
	// the pinned one.
	rdCleanPathErrCertChanged    = 16384
	rdCleanPathMaxDecodeSizeByte = 64 << 10
)

// rdCleanPathError is the RDCleanPathErr SEQUENCE. Optional integers equal to
// zero are omitted from the encoding.
type rdCleanPathError struct {
	ErrorCode      int `asn1:"explicit,tag:0"`
	HTTPStatusCode int `asn1:"explicit,tag:1,optional"`
	WSALastError   int `asn1:"explicit,tag:2,optional"`
	TLSAlertCode   int `asn1:"explicit,tag:3,optional"`
}

// rdCleanPathPDU is the RDCleanPathPdu SEQUENCE. Absent optional fields are
// the zero value (empty string / nil slice) and are not encoded.
type rdCleanPathPDU struct {
	Version           int              `asn1:"explicit,tag:0"`
	Error             rdCleanPathError `asn1:"explicit,tag:1,optional"`
	Destination       string           `asn1:"utf8,explicit,tag:2,optional"`
	ProxyAuth         string           `asn1:"utf8,explicit,tag:3,optional"`
	ServerAuth        string           `asn1:"utf8,explicit,tag:4,optional"`
	PreconnectionBlob string           `asn1:"utf8,explicit,tag:5,optional"`
	X224ConnectionPDU []byte           `asn1:"explicit,tag:6,optional"`
	ServerCertChain   [][]byte         `asn1:"explicit,tag:7,optional"`
	ServerAddr        string           `asn1:"utf8,explicit,tag:9,optional"`
}

func (p rdCleanPathPDU) marshal() ([]byte, error) {
	return asn1.Marshal(p)
}

func parseRDCleanPathPDU(data []byte) (rdCleanPathPDU, error) {
	if len(data) == 0 || len(data) > rdCleanPathMaxDecodeSizeByte {
		return rdCleanPathPDU{}, errors.New("invalid RDCleanPath PDU size")
	}
	var pdu rdCleanPathPDU
	rest, err := asn1.Unmarshal(data, &pdu)
	if err != nil {
		return rdCleanPathPDU{}, fmt.Errorf("decode RDCleanPath PDU: %w", err)
	}
	if len(rest) != 0 {
		return rdCleanPathPDU{}, errors.New("trailing data after RDCleanPath PDU")
	}
	if pdu.Version != rdCleanPathVersion {
		return rdCleanPathPDU{}, fmt.Errorf("unsupported RDCleanPath version %d", pdu.Version)
	}
	return pdu, nil
}

// buildRDCleanPathErrorPDU encodes an error response. httpStatus, wsaError and
// tlsAlert are omitted when zero; x224 (optional) carries the server's
// negotiation-failure confirm for error code 2.
func buildRDCleanPathErrorPDU(code, httpStatus, wsaError, tlsAlert int, x224 []byte) []byte {
	pdu := rdCleanPathPDU{
		Version: rdCleanPathVersion,
		Error: rdCleanPathError{
			ErrorCode:      code,
			HTTPStatusCode: httpStatus,
			WSALastError:   wsaError,
			TLSAlertCode:   tlsAlert,
		},
		X224ConnectionPDU: x224,
	}
	data, err := pdu.marshal()
	if err != nil {
		// Unreachable for these plain field types; fall back to a minimal PDU.
		data, _ = rdCleanPathPDU{Version: rdCleanPathVersion, Error: rdCleanPathError{ErrorCode: rdCleanPathErrGeneral}}.marshal()
	}
	return data
}
