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
		w.Write([]byte("{\"error\":\"unauthorized\",\"error_description\":\"No client with requested id: wrongUser\"}"))
		return
	}

	token := r.FormValue("token")

	if token == "iAmAnAdmin" {
		authData := map[string]interface{}{
			"scope": []string{
				"doppler.firehose",
			},
		}

		marshaled, _ := json.Marshal(authData)
		w.Write(marshaled)
	} else if token == "iAmNotAnAdmin" {
		authData := map[string]interface{}{
			"scope": []string{
				"uaa.not-admin",
			},
		}

		marshaled, _ := json.Marshal(authData)
		w.Write(marshaled)
	} else if token == "expiredToken" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"error\":\"invalid_token\",\"error_description\":\"Token has expired\"}"))
	} else if token == "invalidToken" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"invalidToken\":\"invalid_token\",\"error_description\":\"Invalid token (could not decode): invalidToken\"}"))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

}
