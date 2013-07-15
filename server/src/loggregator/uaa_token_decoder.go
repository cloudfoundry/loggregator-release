package loggregator

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

type TokenDecoder interface {
	Decode(token string) (map[string]interface{}, error)
}

func NewUaaTokenDecoder(uaaVerificationKey []byte) (TokenDecoder, error) {
	var block *pem.Block
	block, _ = pem.Decode(uaaVerificationKey)
	if block == nil {
		return nil, errors.New("Could not parse public key data from pem.")
	}

	parsedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaKey, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Key is not a valid RSA public key.")
	}
	return &UaaTokenDecoder{rsaKey}, nil
}

type UaaTokenDecoder struct {
	key *rsa.PublicKey
}

func (d UaaTokenDecoder) Decode(authToken string) (map[string]interface{}, error) {
	authTokenParts := strings.Split(authToken, " ")

	if len(authTokenParts) != 2 || authTokenParts[0] != "bearer" {
		return nil, errors.New(fmt.Sprintf("invalid authentication header: %s", authToken))
	}

	jwtTokenParts := strings.Split(authTokenParts[1], ".")

	if len(jwtTokenParts) != 3 {
		return nil, errors.New("Not enough or too many segments")
	}

	signature, err := base64.URLEncoding.DecodeString(restorePadding(jwtTokenParts[2]))
	if err != nil {
		return nil, errors.New("Trouble base64 decoding signature")
	}

	signingString := strings.Join(jwtTokenParts[0:2], ".")

	hasher := sha256.New()
	hasher.Write([]byte(signingString))

	err = rsa.VerifyPKCS1v15(d.key, crypto.SHA256, hasher.Sum(nil), signature)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Signature verification failed: %s", err))
	}

	var parsedPayload map[string]interface{}
	payload, err := base64.URLEncoding.DecodeString(restorePadding(jwtTokenParts[1]))

	err = json.Unmarshal(payload, &parsedPayload)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Can not unmarshall payload JSON %s: %s", payload, err))
	}

	return parsedPayload, nil
}

func restorePadding(seg string) string {
	switch len(seg) % 4 {
	case 2:
		seg = seg + "=="
	case 3:
		seg = seg + "==="
	}
	return seg
}
