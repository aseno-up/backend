package crypto

import (
	"encoding/hex"
	"testing"
)

func testKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return hex.EncodeToString(key)
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := testKey()
	plaintext := "sk_test_abc123_my_secret_api_key"

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("Encrypted text should not equal plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("Expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	key := testKey()

	encrypted, err := Encrypt("", key)
	if err != nil {
		t.Fatalf("Encrypt empty failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt empty failed: %v", err)
	}

	if decrypted != "" {
		t.Fatalf("Expected empty string, got %q", decrypted)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := testKey()
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 1)
	}
	key2Hex := hex.EncodeToString(key2)

	encrypted, err := Encrypt("secret", key1)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, key2Hex)
	if err == nil {
		t.Fatal("Decrypt with wrong key should fail")
	}
}

func TestEncrypt_InvalidKey(t *testing.T) {
	_, err := Encrypt("test", "short")
	if err == nil {
		t.Fatal("Encrypt with short key should fail")
	}
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	key := testKey()
	plaintext := "same-text"

	enc1, _ := Encrypt(plaintext, key)
	enc2, _ := Encrypt(plaintext, key)

	if enc1 == enc2 {
		t.Fatal("Two encryptions of the same text should produce different ciphertexts (random nonce)")
	}

	// But both should decrypt to the same plaintext
	dec1, _ := Decrypt(enc1, key)
	dec2, _ := Decrypt(enc2, key)
	if dec1 != dec2 || dec1 != plaintext {
		t.Fatal("Both should decrypt to same plaintext")
	}
}
