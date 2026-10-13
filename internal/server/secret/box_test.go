package secret

import (
	"bytes"
	"errors"
	"testing"
)

func TestBoxRoundTripAndTamper(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	b, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	aad := KeyAAD("user-a", "key-1")
	ct, nonce, err := b.Seal([]byte("sk-secret-value"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("sk-secret-value")) || len(nonce) != 12 {
		t.Fatalf("ciphertext lộ plaintext hoặc nonce sai độ dài")
	}
	got, err := b.Open(ct, nonce, aad)
	if err != nil || string(got) != "sk-secret-value" {
		t.Fatalf("giải mã sai: %q %v", got, err)
	}

	// AAD của user/key khác: ciphertext chép sang dòng khác không mở được.
	if _, err := b.Open(ct, nonce, KeyAAD("user-b", "key-1")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("sai AAD phải lỗi: %v", err)
	}
	other, _ := New(bytes.Repeat([]byte{8}, 32))
	if _, err := other.Open(ct, nonce, aad); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("sai master key phải lỗi: %v", err)
	}
	ct[0] ^= 1
	if _, err := b.Open(ct, nonce, aad); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("ciphertext bị sửa phải lỗi: %v", err)
	}
	if _, err := New([]byte("short")); err == nil {
		t.Fatal("master key ngắn phải lỗi")
	}
}

func TestBoxNonceUnique(t *testing.T) {
	b, _ := New(bytes.Repeat([]byte{1}, 32))
	_, n1, _ := b.Seal([]byte("x"), nil)
	_, n2, _ := b.Seal([]byte("x"), nil)
	if bytes.Equal(n1, n2) {
		t.Fatal("nonce lặp lại")
	}
}
