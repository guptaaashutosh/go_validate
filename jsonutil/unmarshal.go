// Package jsonutil provides common utilities for properly handling JSON payloads in HTTP body.
package jsonutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"strings"
)

// Unmarshal provides a common implementation of JSON unmarshalling
// with well defined error handling.
// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an InvalidUnmarshalError.
// It can unmarshal data available in request/data param.
func Unmarshal(r *http.Request, data []byte, v interface{}) (int, error) {
	// ensure that some data is provided for unmarshalling
	if r == nil && data == nil {
		return http.StatusUnsupportedMediaType, fmt.Errorf("no data provided")
	} else if r != nil && data != nil {
		// if someone sends multiple data for unmarshalling then it gives below error
		return http.StatusUnsupportedMediaType, fmt.Errorf("multiple data provided for unmarshalling not supported")
	}

	var d = &json.Decoder{}
	// if "r" request is not empty then it will read data from request body to unmarshal that data into object provided in "v".
	if r != nil && data == nil {
		// read request body as []byte.
		bodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("error reading request body")
		}
		// as we are only allowed to read request body once so we need to set request body after reading for further use of request data
		// it will set request body which we have got in "bodyBytes" above.
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		// check if content type is valid or not.
		if !HasContentType(r, "application/json") {
			return http.StatusUnsupportedMediaType, fmt.Errorf("content-type is not application/json")
		}

		d = json.NewDecoder(bytes.NewReader(bodyBytes))
	} else if data != nil && r == nil {
		d = json.NewDecoder(bytes.NewReader(data))
	}

	// DisallowUnknownFields causes the Decoder to return an error when the destination
	// is a struct and the input contains object keys which do not match any
	// non-ignored, exported fields in the destination.
	// d.DisallowUnknownFields()

	// handle errors returned while decoding data into object.
	if err := d.Decode(&v); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return http.StatusBadRequest, fmt.Errorf("malformed json at position %v", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return http.StatusBadRequest, fmt.Errorf("malformed json")
		case errors.As(err, &unmarshalError):
			return http.StatusBadRequest, fmt.Errorf("invalid value %v at position %v", unmarshalError.Field, unmarshalError.Offset)
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return http.StatusBadRequest, fmt.Errorf("unknown field %s", fieldName)
		case errors.Is(err, io.EOF):
			return http.StatusBadRequest, fmt.Errorf("body must not be empty")
		case err.Error() == "http: request body too large":
			return http.StatusRequestEntityTooLarge, err
		default:
			return http.StatusInternalServerError, fmt.Errorf("failed to decode json %v", err)
		}
	}
	if d.More() {
		return http.StatusBadRequest, fmt.Errorf("body must contain only one JSON object")
	}
	return http.StatusOK, nil
}

// check for valid content type.
func HasContentType(r *http.Request, mimetype string) bool {
	contentType := r.Header.Get("Content-type")
	if contentType == "" {
		return mimetype == "application/octet-stream"
	}

	for _, v := range strings.Split(contentType, ",") {
		t, _, err := mime.ParseMediaType(v)
		if err != nil {
			break
		}
		if t == mimetype {
			return true
		}
	}
	return false
}