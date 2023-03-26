package httpinterface

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func Respond(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v)
}

func VerifySignature(payload []byte, hash, key string) (bool, error) {
	hashNew := strings.Replace(hash, "sha256=", "", 1)
	sig, err := hex.DecodeString(hashNew)
	if err != nil {
		return false, err
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, err = mac.Write(payload)
	if err != nil {
		return false, err
	}
	return hmac.Equal(sig, mac.Sum(nil)), nil

}

func StreamToByte(stream io.Reader) ([]byte, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(stream)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
