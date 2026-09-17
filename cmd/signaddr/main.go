// Command signaddr produces the signed DNS TXT record value that lets a
// ShuzaGram client trust ipshuzaqq.sgq.me's answer as genuinely coming from
// this server, not a spoofed/poisoned DNS response.
//
// The client already ships this server's RSA identity public key (the same
// key used for the MTProto handshake). This tool signs the exact string
// "<ip>:<port>" with the matching private key (SHA-256 + RSASSA-PKCS1-v1_5),
// and prints the TXT record value:
//
//	v1:<ip>:<port>:<base64 signature>
//
// Publish that exact string as a TXT record on ipshuzaqq.sgq.me. The client
// re-derives "<ip>:<port>" from the record itself and verifies the signature
// against its embedded public key before ever dialing that address -- an
// attacker who controls DNS but not server_rsa.pem cannot produce a TXT
// value the client will accept, so the advertised IP can't be swapped out
// from under it.
//
// There is deliberately no expiry/timestamp in the signed payload: this
// attestation is meant to be pasted into DNS once and left there until the
// server's IP actually changes, not refreshed on a schedule.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
)

func main() {
	ip := flag.String("ip", "", "server IP address to attest (must match TELESRV_ADVERTISE_IP)")
	port := flag.Int("port", 2398, "server MTProto port")
	keyPath := flag.String("key", "", "path to the server's RSA private key PEM (server_rsa.pem)")
	flag.Parse()

	if *ip == "" || *keyPath == "" {
		fmt.Fprintln(os.Stderr, "usage: signaddr -ip <ip> -port <port> -key <server_rsa.pem>")
		os.Exit(2)
	}

	keyBytes, err := os.ReadFile(*keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read key: %v\n", err)
		os.Exit(1)
	}
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		fmt.Fprintln(os.Stderr, "read key: no PEM block found")
		os.Exit(1)
	}
	priv, err := parseRSAPrivateKey(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse key: %v\n", err)
		os.Exit(1)
	}

	payload := fmt.Sprintf("%s:%d", *ip, *port)
	digest := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign: %v\n", err)
		os.Exit(1)
	}

	txt := fmt.Sprintf("v1:%s:%s", payload, base64.StdEncoding.EncodeToString(sig))
	fmt.Printf("TXT record for ipshuzaqq.sgq.me:\n\n%s\n\n(length: %d bytes -- fine for a single TXT string, DNS providers allow up to 255 per string / several KB total)\n", txt, len(txt))
}

// parseRSAPrivateKey accepts both PKCS#1 ("RSA PRIVATE KEY") and PKCS#8
// ("PRIVATE KEY") encodings, matching whatever gramsrv itself wrote.
func parseRSAPrivateKey(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("PEM key is not an RSA private key")
	}
	return rsaKey, nil
}
