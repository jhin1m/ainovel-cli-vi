// Package secret mã hoá API key của người dùng bằng AES-256-GCM với một master key.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// KeyVersion là phiên bản master key hiện hành, lưu cạnh mỗi ciphertext để sau
// này xoay vòng key mà không cần migration.
const KeyVersion = 1

// ErrDecrypt là lỗi chung khi giải mã thất bại (sai key, sai AAD, dữ liệu hỏng).
var ErrDecrypt = errors.New("không giải mã được dữ liệu bí mật")

// Box niêm phong / mở dữ liệu với AAD gắn ngữ cảnh (vd. user_id|key_id) để một
// ciphertext chép sang dòng khác không giải mã được.
type Box struct {
	aead cipher.AEAD
}

// New tạo Box từ master key 32 byte.
func New(masterKey []byte) (*Box, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key phải dài đúng 32 byte")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal mã hoá plaintext, trả về ciphertext và nonce 12 byte ngẫu nhiên.
func (b *Box) Seal(plaintext, aad []byte) (ciphertext, nonce []byte, err error) {
	nonce = make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return b.aead.Seal(nil, nonce, plaintext, aad), nonce, nil
}

// Open giải mã; sai key, sai AAD hay dữ liệu bị sửa đều trả ErrDecrypt.
func (b *Box) Open(ciphertext, nonce, aad []byte) ([]byte, error) {
	if len(nonce) != b.aead.NonceSize() {
		return nil, ErrDecrypt
	}
	out, err := b.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return out, nil
}

// KeyAAD là AAD cho API key: ràng buộc ciphertext với đúng user và đúng key id.
func KeyAAD(userID, keyID string) []byte {
	return []byte(userID + "|" + keyID)
}
