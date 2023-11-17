package api

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/config"

	"github.com/go-playground/assert/v2"
	"github.com/gorilla/mux"
)

func TestTokenGeneratorEndpoints(t *testing.T) {
	tests := []struct {
		endpoint       string
		httpMethod     string
		expectedStatus int
		verifyResponse func(*testing.T, crypto.PublicKey, *bytes.Buffer)
	}{
		{
			endpoint:       "/token",
			httpMethod:     "GET",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			endpoint:       "/token",
			httpMethod:     "POST",
			expectedStatus: http.StatusOK,
			verifyResponse: verifyToken,
		},
	}

	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		t.Fatal("Error generating keypair: ", err)
	}

	//    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://localhost:9999/api/cloud-connector/v1/token/token | jq

	fmt.Println("public key:", publicKey)

	for _, tc := range tests {
		t.Run(tc.httpMethod+" "+tc.endpoint, func(t *testing.T) {
			fmt.Println("method: ", tc.httpMethod)
			fmt.Println("endpoint: ", tc.endpoint)

			postBody := createTokenGeneratorPostBody()

			req, err := http.NewRequest(tc.httpMethod, tc.endpoint, postBody)
			assert.Equal(t, err, nil)

			req.Header.Add(IDENTITY_HEADER_NAME, "eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMDAwMiIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJiYXNpYyIsICJhdXRoX3R5cGUiOiAiY2VydC1hdXRoIn19")

			rr := httptest.NewRecorder()

			cfg := config.GetConfig()
			apiMux := mux.NewRouter()
			tokenGeneratorServer := NewTokenGeneratorServer(apiMux, "ugh", privateKey, cfg)
			tokenGeneratorServer.Routes()

			tokenGeneratorServer.router.ServeHTTP(rr, req)

			assert.Equal(t, rr.Code, tc.expectedStatus)

			if tc.verifyResponse != nil {
				tc.verifyResponse(t, publicKey, rr.Body)
			}
		})
	}
}

func generateKeyPair() (*rsa.PrivateKey, crypto.PublicKey, error) {

	bitSize := 4096

	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, nil, err
	}

	publicKey := privateKey.Public()

	return privateKey, publicKey, nil
}

func createTokenGeneratorPostBody() io.Reader {
	return strings.NewReader("{\"placeholder\": \"imaplaceholdervalue\"}")
}

func verifyToken(t *testing.T, publicKey crypto.PublicKey, body *bytes.Buffer) {

	var tokenResp tokenResponse
	err := json.Unmarshal(body.Bytes(), &tokenResp)
	if err != nil {
		t.Fatal("Failed to unmarshal token response:", err)
	}
}
