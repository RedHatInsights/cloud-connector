package api

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type ApiSpecServer struct {
	router       *mux.Router
	urlPrefix    string
	specFileName string
}

func NewApiSpecServer(r *mux.Router, urlPrefix string, f string) *ApiSpecServer {
	return &ApiSpecServer{
		router:       r,
		urlPrefix:    urlPrefix,
		specFileName: f,
	}
}

func (s *ApiSpecServer) Routes() {
	subRouter := s.router.PathPrefix(s.urlPrefix).Subrouter()
	subRouter.HandleFunc("/openapi.json", s.handleApiSpec()).Methods(http.MethodGet)
}

func (s *ApiSpecServer) handleApiSpec() http.HandlerFunc {

	return func(w http.ResponseWriter, req *http.Request) {
		file, err := ioutil.ReadFile(s.specFileName)
		if err != nil {
			log.Printf("Unable to read API spec file (%s): %s", s.specFileName, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(file)
	}
}
