package api

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type ApiSpecServer struct {
	router       *mux.Router
	specFileName string
}

func NewApiSpecServer(r *mux.Router, f string) *ApiSpecServer {
	return &ApiSpecServer{
		router:       r,
		specFileName: f,
	}
}

func (s *ApiSpecServer) Routes() {
	s.router.HandleFunc("/openapi.json", s.handleApiSpec()).Methods(http.MethodGet)
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
