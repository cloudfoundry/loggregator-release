package web

import (
	"bytes"
	"encoding/json"
	"net/http"
)

var (
	errMethodNotAllowed           = newJSONError(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	errEmptyQuery                 = newJSONError(http.StatusBadRequest, "empty_query", "query cannot be empty")
	errMissingType                = newJSONError(http.StatusBadRequest, "missing_envelope_type", "query must provide at least one envelope type")
	errCounterNamePresentButEmpty = newJSONError(http.StatusBadRequest, "missing_counter_name", "counter.name is invalid without value")
	errGaugeNamePresentButEmpty   = newJSONError(http.StatusBadRequest, "missing_gauge_name", "gauge.name is invalid without value")
	errStreamingUnsupported       = newJSONError(http.StatusInternalServerError, "streaming_unsupported", "request does not support streaming")
	errNotFound                   = newJSONError(http.StatusNotFound, "not_found", "resource not found")
)

type jsonError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Name    string `json:"error"`
}

func newJSONError(code int, name, msg string) jsonError {
	return jsonError{
		Code:    code,
		Message: msg,
		Name:    name,
	}
}

func (e jsonError) Error() string {
	buf := bytes.NewBuffer(nil)

	// We control this struct and it should never fail to encode.
	_ = json.NewEncoder(buf).Encode(&e)

	return buf.String()
}

func (e jsonError) Write(w http.ResponseWriter) {
	http.Error(w, e.Error(), e.Code)
}
