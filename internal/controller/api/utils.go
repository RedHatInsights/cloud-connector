package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type errorResponse struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func writeJSONResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Unable to encode payload!", http.StatusUnprocessableEntity)
		log.Println("Unable to encode payload!")
	}
}

func decodeJSON(body io.ReadCloser, data interface{}) error {
	dec := json.NewDecoder(body)
	if err := dec.Decode(&data); err != nil {
		// FIXME: More specific error handling needed.. case statement for different scenarios?
		return errors.New("Request body includes malformed json")
	}

	v := validator.New()
	if err := v.Struct(data); err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			log.Println(e)
		}
		return errors.New("Request body is missing required fields")
	} else if dec.More() {
		return errors.New("Request body must only contain one json object")
	}

	return nil
}
