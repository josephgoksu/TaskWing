package cmd

import (
    "crypto/rand"
    "encoding/hex"
    "os"
    "testing"
)

func TestArchiveEncryptDecrypt(t *testing.T) {
    // generate random 32-byte key
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil { t.Fatalf("rand: %v", err) }
    hexKey := hex.EncodeToString(key)

    // write plaintext
    in := t.TempDir() + "/in.json"
    out := t.TempDir() + "/out.enc"
    dec := t.TempDir() + "/dec.json"
    content := []byte(`{"hello":"world"}`)
    if err := os.WriteFile(in, content, 0o600); err != nil { t.Fatalf("write in: %v", err) }

    if err := encryptFile(in, out, hexKey); err != nil { t.Fatalf("encrypt: %v", err) }
    if err := decryptFile(out, dec, hexKey); err != nil { t.Fatalf("decrypt: %v", err) }

    got, err := os.ReadFile(dec)
    if err != nil { t.Fatalf("read dec: %v", err) }
    if string(got) != string(content) { t.Fatalf("roundtrip mismatch: got %q", string(got)) }
}

