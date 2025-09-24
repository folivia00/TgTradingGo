package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// ValidateInitData проверяет подпись Telegram WebApp initData.
// Документация: https://core.telegram.org/bots/webapps#validating-data-received-via-the-web-app
func ValidateInitData(initData, botToken string) bool {
	if initData == "" || botToken == "" {
		return false
	}
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return false
	}
	hash := vals.Get("hash")
	vals.Del("hash")
	// собрать data_check_string: key=value по алфавиту ключей, через \n
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+vals.Get(k))
	}
	dcs := strings.Join(parts, "\n")
	// secret = SHA256(botToken)
	secret := sha256.Sum256([]byte(botToken))
	h := hmac.New(sha256.New, secret[:])
	h.Write([]byte(dcs))
	digest := h.Sum(nil)
	expected := hex.EncodeToString(digest)
	return strings.EqualFold(expected, hash)
}
