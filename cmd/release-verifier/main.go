// release-verifier is a dependency-free, offline verifier for native releases.
package main

import (
	"flag"
	"fmt"
	"os"

	"aurago/internal/releaseverify"
)

func main() {
	manifest := flag.String("manifest", "", "checksum manifest path")
	signature := flag.String("signature", "", "detached Cosign signature path")
	key := flag.String("key", "", "explicit operator public key; defaults to the embedded AuraGo root")
	keyring := flag.String("keyring", "", "root-signed release keyring path")
	keyringSignature := flag.String("keyring-signature", "", "detached keyring signature path")
	flag.Parse()
	if *manifest == "" || *signature == "" || flag.NArg() != 0 || (*keyring == "") != (*keyringSignature == "") || (*key != "" && *keyring != "") {
		fmt.Fprintln(os.Stderr, "manifest and signature are required; key and signed keyring are mutually exclusive")
		os.Exit(2)
	}
	read := func(path string, limit int64) []byte {
		data, err := releaseverify.ReadFile(path, limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read verification input: %v\n", err)
			os.Exit(1)
		}
		return data
	}
	data := read(*manifest, releaseverify.ManifestLimit)
	sig := read(*signature, 4096)
	publicKey := releaseverify.PublicKey
	if *key != "" {
		publicKey = read(*key, 16<<10)
	}
	var err error
	if *keyring != "" {
		err = releaseverify.VerifyKeyring(data, sig, read(*keyring, 64<<10), read(*keyringSignature, 4096))
	} else {
		err = releaseverify.Verify(data, sig, publicKey)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Release checksum signature verified.")
}
