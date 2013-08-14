package sink

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

/*
Parts:

header: {"typ":"JWT","alg":"RS256"}
payload: {"user_id":"abc1234","exp":137452386}

eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJ1c2VyX2lkIjoiYWJjMTIzNCIsImV4cCI6MTM3NDUyMzg2MH0.JF8dTUJp3NaZhfIhYgesKh-HmV9isnJc51eFaqeFuIhJQ73wiyekfgu-5jSoquVRITSL3cIRjD42F8WabCMYHA

Private Key:
-----BEGIN RSA PRIVATE KEY-----
      MIIBOwIBAAJBAN+5O6n85LSs/fj46Ht1jNbc5e+3QX+suxVPJqICvuV6sIukJXXE
      zfblneN2GeEVqgeNvglAU9tnm3OIKzlwM5UCAwEAAQJAEhJ2fV7OYsHuqiQBM6fl
      Pp4NfPXCtruPSUNhjYjHPuYpnqo6cpuUNAzRvqAdDkJJsPCPt1E5AWOYUYOmLE+d
      AQIhAO/XxMb9GrTDyqJDvS8T1EcJpLCaUIReae0jSg1RnBrhAiEA7st6WLmOyTxX
      JgLcO6LUfW6RsE3pgi9NGL25P3eOAzUCIQDUFKi1CJR36XWh/GIqYc9grX9KhnnS
      QqZKAd12X4a5IQIhAMTOJKaNP/Xwai7kupfX6mL6Rs5UWDg4PcU/UDbTlNJlAiBv
      2yrlT5h164jGCxqe7++1kIl4ollFCgz6QJ8lcmb/2Q==
      -----END RSA PRIVATE KEY-----

Public Key:
-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAN+5O6n85LSs/fj46Ht1jNbc5e+3QX+s
uxVPJqICvuV6sIukJXXEzfblneN2GeEVqgeNvglAU9tnm3OIKzlwM5UCAwEAAQ==
-----END PUBLIC KEY-----
*/

func TestItChecksForValidTokenFormat(t *testing.T) {
	publicKey := `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAN+5O6n85LSs/fj46Ht1jNbc5e+3QX+s
uxVPJqICvuV6sIukJXXEzfblneN2GeEVqgeNvglAU9tnm3OIKzlwM5UCAwEAAQ==
-----END PUBLIC KEY-----
`

	decoder, _ := NewUaaTokenDecoder([]byte(publicKey))

	tokenWithoutBearerString := "token"

	_, err := decoder.Decode(tokenWithoutBearerString)

	assert.Error(t, err, "invalid authentication header: token")

	tokenWithInvalidBearerString := "notBearer token"

	_, err = decoder.Decode(tokenWithInvalidBearerString)

	assert.Error(t, err, "invalid authentication header: token")
}

func TestThatTokenHasProperNumberOfSegments(t *testing.T) {
	publicKey := `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAN+5O6n85LSs/fj46Ht1jNbc5e+3QX+s
uxVPJqICvuV6sIukJXXEzfblneN2GeEVqgeNvglAU9tnm3OIKzlwM5UCAwEAAQ==
-----END PUBLIC KEY-----
`

	decoder, _ := NewUaaTokenDecoder([]byte(publicKey))

	tokenWithNotEnough := "bearer header.payload"

	_, err := decoder.Decode(tokenWithNotEnough)

	assert.Error(t, err, "Not enough or too many segments")

	tokenWithTooMuch := "bearer header.payload.crypto.tooMuchStuff"

	_, err = decoder.Decode(tokenWithTooMuch)

	assert.Error(t, err, "Not enough or too many segments")
}

func TestThatItVerifiesPayloadSignature(t *testing.T) {
	publicKey := `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAN+5O6n85LSs/fj46Ht1jNbc5e+3QX+s
uxVPJqICvuV6sIukJXXEzfblneN2GeEVqgeNvglAU9tnm3OIKzlwM5UCAwEAAQ==
-----END PUBLIC KEY-----
`

	decoder, _ := NewUaaTokenDecoder([]byte(publicKey))

	tokenWithInvalidSignature := `bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJ1c2VyX2lkIjoiYWJjMTIzNCIsImV4cCI6MTM3NDUyMzg2MH0.JF8dTUJp3NaZhfIhYgesKh-HmV9isnJc51eFaqeFuIhJQ73wiyekfgu-5jSoquVRITSL3cIRjD42F8acbCMYHA`
	_, err := decoder.Decode(tokenWithInvalidSignature)

	assert.Error(t, err, "Signature verification failed")

	tokenWithValidSignature := `bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJ1c2VyX2lkIjoiYWJjMTIzNCIsImV4cCI6MTM3NDUyMzg2MH0.JF8dTUJp3NaZhfIhYgesKh-HmV9isnJc51eFaqeFuIhJQ73wiyekfgu-5jSoquVRITSL3cIRjD42F8WabCMYHA`

	results, err := decoder.Decode(tokenWithValidSignature)

	assert.NoError(t, err)

	assert.Equal(t, results.UserId, "abc1234")
}
