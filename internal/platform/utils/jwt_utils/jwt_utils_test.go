package jwt_utils

import (
	"context"
	"crypto/rsa"
	"io/ioutil"
	"log"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/go-playground/assert/v2"
	"github.com/golang-jwt/jwt"
)

var verifyKey *rsa.PublicKey

func init() {
	logger.InitLogger()
	verifyBytes, err := ioutil.ReadFile("../../../../test/jwt/jwtpubkey.rsa.pub")
	fatal(err)
	verifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
	fatal(err)
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func verifyToken(tokenString string, t *testing.T) *jwt.Token {

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			t.Fatalf("Unexpected signing method: %v", token.Header["alg"])
		}
		return verifyKey, nil
	})
	if err != nil {
		t.Fatalf("Invalid token format")
	}
	return token

}

func TestRsaTokenGeneration(t *testing.T) {
	testClientID := "test-id"
	jwtPrivateKeyFile := "../../../../test/jwt/jwtprivkey.rsa"
	gen, error := NewRSABasedJwtGenerator(jwtPrivateKeyFile, testClientID, 10)
	assert.Equal(t, error, nil)
	context := context.Background()
	tok, err := gen(context)
	assert.Equal(t, err, nil)
	assert.NotEqual(t, tok, nil)
	token := verifyToken(tok, t)
	claims := (token.Claims).(jwt.MapClaims)
	assert.Equal(t, claims["auth-group"], "admin")
	assert.Equal(t, claims["client-id"], testClientID)

}

func TestFileGeneration(t *testing.T) {
	expectedJwt := "eyJhbGciOiJSUzI1NiIsImtpZCI6InJoY2xvdWQiLCJ0eXAiOiJKV1QifQ.eyJleHAiOjE2MTUyNzUwMzYsImNsaWVudC1pZCI6ImxpbmRhbmktdGVzdCIsImF1dGgtZ3JvdXAiOiJhZG1pbiJ9.JJboKz6hWkRaImIEuGCcKbxeTzDeJdGM3_uqHbGxLRIjJSZFELQrQdZM40m_MYGsropnBLOybtMv8xwoKOr8on1KEqPTTwLTl4LC3EngOk7YwoOSDlZDCvI8mkFBKAzmsLQ8e9T4MHIHss7mjkWR66ylA7ayvSbmOk1GuC8osWMIRdNkfm1dyzWD5zjb10ZQiPCK5mjUJU217d5SYFUH9pJB3yV71vYtAUncV2x4RsqgZRM958oesdugO99EoY9Pjd_gPV0ip1_O3QiGgqKdVGycWMHDvRwipR_w1M6D6QuDXXrJolIpWxlSDSTDCXrzHyv8X1OD3gS214_Iyor2g\n"
	mqttBrokerJwtFile := "../../../../test/jwt/testjwt.txt"
	gen, error := NewFileBasedJwtGenerator(mqttBrokerJwtFile)
	assert.Equal(t, error, nil)
	context := context.Background()
	tok, err := gen(context)
	assert.Equal(t, err, nil)
	assert.Equal(t, tok, expectedJwt)
}
