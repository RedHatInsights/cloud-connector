package api

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gorilla/mux"
)

var _ = Describe("OpenAPI", func() {

	Describe("Serve openapi.json", func() {
		Context("With a valid spec file", func() {
			It("Should return the openapi.json file", func() {

				req, err := http.NewRequest("GET", "/openapi.json", nil)
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				apiMux := mux.NewRouter()
				apiSpecServer := NewApiSpecServer(apiMux, "api.spec.json")
				apiSpecServer.Routes()

				apiSpecServer.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))

				expectedBytes, err := ioutil.ReadFile("api.spec.json")
				Expect(err).NotTo(HaveOccurred())
				Expect(rr.Body.Bytes()).To(Equal(expectedBytes))
			})
		})

		Context("With a missing invalid path to the api spec file", func() {
			It("Should return a 404", func() {
				req, err := http.NewRequest("GET", "/openapi.json", nil)
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				apiMux := mux.NewRouter()
				apiSpecServer := NewApiSpecServer(apiMux, "invalid-file-name")
				apiSpecServer.Routes()

				apiSpecServer.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusNotFound))
			})
		})
	})
})
