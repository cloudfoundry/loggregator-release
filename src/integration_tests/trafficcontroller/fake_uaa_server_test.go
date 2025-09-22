package trafficcontroller_test

import (
	"encoding/json"
	"net/http"
)

type FakeUaaHandler struct {
}

func (h *FakeUaaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/check_token" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Header.Get("Authorization") != "Basic Ym9iOnlvdXJVbmNsZQ==" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("{\"error\":\"unauthorized\",\"error_description\":\"No client with requested id: wrongUser\"}"))
		return
	}

	token := r.FormValue("token")

	switch token {
	case "iAmAnAdmin":
		authData := map[string]interface{}{
			"scope": []string{
				"doppler.firehose",
			},
		}

		marshaled, _ := json.Marshal(authData)
		_, _ = w.Write(marshaled)
	case "iAmNotAnAdmin":
		authData := map[string]interface{}{
			"scope": []string{
				"uaa.not-admin",
			},
		}

		marshaled, _ := json.Marshal(authData)
		_, _ = w.Write(marshaled)
	case "expiredToken":
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{\"error\":\"invalid_token\",\"error_description\":\"Token has expired\"}"))
	case "invalidToken":
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{\"invalidToken\":\"invalid_token\",\"error_description\":\"Invalid token (could not decode): invalidToken\"}"))
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}

}
