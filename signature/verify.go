package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Verify 验证 MWS365 事件/回调签名
// 算法: sha256(timestamp + nonce + encryptKey + body)
func Verify(encryptKey, timestamp, nonce, body, signature string) bool {
	if encryptKey == "" || signature == "" {
		return false
	}
	expected := Sign(encryptKey, timestamp, nonce, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Sign 生成签名
func Sign(encryptKey, timestamp, nonce, body string) string {
	content := timestamp + nonce + encryptKey + body
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
