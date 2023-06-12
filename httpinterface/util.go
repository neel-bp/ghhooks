package httpinterface

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// VirifyType - check if a given event  type is supported
func VerifyEvent(eventType string, bodyInBytes []byte, configBranchName string) error {
	switch eventType {

	case "push":

		var payload WebhookPayload
		err := json.Unmarshal(bodyInBytes, &payload)
		if err != nil {
			return fmt.Errorf("Couldnt unmarshal push event  - %v", err)
		}

		if payload.Ref == "" {
			return fmt.Errorf("invalid payload: cannot find ref inside given payload")
		}

		branchStringArr := strings.Split(payload.Ref, "/")
		branchString := branchStringArr[len(branchStringArr)-1]

		if configBranchName != branchString {
			return fmt.Errorf("request recieved but the push event is not for the configured branch")
		}

		return nil

	case "release":
		supportedReleaseActions := map[string]bool{
			"published": true,
			"created":   false,
			"released":  false,
		}

		var payload ReleaseWebhookPayload
		err := json.Unmarshal(bodyInBytes, &payload)

		if err != nil {
			return fmt.Errorf("couldnt unmarshal release event  - %v", err)
		}
		supported, exists := supportedReleaseActions[payload.Action]
		if exists && supported {
			return nil
		}
		return fmt.Errorf("release event  action %s is not enabled", payload.Action)

	default:
		return fmt.Errorf("event type %s: is not supported", eventType)
	}
}

func StreamToByte(stream io.Reader) ([]byte, error) {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(stream)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
