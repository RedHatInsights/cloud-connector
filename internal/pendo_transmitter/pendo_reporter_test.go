package pendo_transmitter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMakeRequest(t *testing.T) {
	cases := []struct {
		apiKey       string
		responseCode int
		responseBody string
		expectError  bool
	}{
		{
			apiKey:       "issasecret",
			responseCode: 200,
			responseBody: "success",
			expectError:  false,
		},
		{
			apiKey:       "sush",
			responseCode: 500,
			responseBody: "",
			expectError:  true,
		},
	}
	for _, c := range cases {
		receivedKey := ""

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedKey = r.Header.Get("x-pendo-integration-key")

			w.WriteHeader(c.responseCode)
			w.Write([]byte(c.responseBody))
		}))
		defer ts.Close()

		resp, err := makeRequest(ts.URL, 1*time.Second, c.apiKey)

		if c.expectError && err == nil {
			t.Fatalf("Expected error; got none.")
		}

		if !c.expectError && err != nil {
			t.Fatalf("Got error; expected none.")
		}

		if receivedKey != c.apiKey {
			t.Fatalf("API key not getting set properly.")
		}

		if resp != c.responseBody {
			t.Fatalf("Issue with parsing response.")
		}

	}
}
