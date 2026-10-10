package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 4 * 1024

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	var extra any

	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}

	if err != nil {
		return err
	}

	return fmt.Errorf("request body must contain a single JSON value")
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var sizeErr *http.MaxBytesError

	if errors.As(err, &sizeErr) {
		http.Error(
			w,
			"request body too large",
			http.StatusRequestEntityTooLarge,
		)
		return
	}

	http.Error(w, "invalid request body", http.StatusBadRequest)
}
