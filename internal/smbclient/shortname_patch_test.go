package smbclient

import (
	"encoding/binary"
	"testing"

	"github.com/hirochachacha/go-smb2/v2/x/wire"
)

// TestSambaDotShortNameIsAccepted guards the AuraGo patch in third_party/go-smb2:
// Samba fills the short name of the "." entry with bytes the upstream character
// check rejects, which made every directory listing fail.
func TestSambaDotShortNameIsAccepted(t *testing.T) {
	entry := make([]byte, 104+2)
	binary.LittleEndian.PutUint32(entry[60:], 2) // FileNameLength
	entry[68] = 24                               // ShortNameLength
	entry[70+23] = 0x80
	entry[104] = '.'
	if !wire.IsInvalidShortName(entry[70 : 70+24]) {
		t.Skip("short name is no longer rejected by the character check; patch not needed")
	}
	if wire.FileIdBothDirectoryInformationDecoder(entry).IsInvalid() {
		t.Fatal("entry with a Samba style short name was rejected")
	}
}
