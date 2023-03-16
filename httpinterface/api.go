package httpinterface

import (
	"encoding/json"
	"net/http"
	"strings"

	"ghhooks.com/hook/core"
	"github.com/gorilla/mux"
)

// TODO: verify signature before accepting webhook
// DONE: verify if branch is correct
// TODO: to add or not to add logger
// TODO: log with levels and cli flags
// TODO: think about error stack traces probably
// TODO: brainstorm about proxy server that will send to all the agents running on different servers
// TODO: golang ssh client is also an option
// DONE: create global resultmap in core package for keeping track of build results

func Respond(w http.ResponseWriter, statusCode int, v interface{}) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func WebHookListener(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	projectID, ok := vars["project"]
	if !ok {
		Respond(w, 400, map[string]interface{}{
			"error": "no vars found",
		})
		return
	}
	project, ok := core.ServerConf.Project[projectID]
	if !ok {
		Respond(w, 400, map[string]interface{}{
			"error": "no project found with given project name",
		})
		return
	}

	payload := make(map[string]interface{}, 0)
	decoder := json.NewDecoder(r.Body)
	decoder.Decode(&payload)

	// check out sample github webhook for details
	ref, ok := payload["ref"]
	if !ok {
		Respond(w, 400, map[string]interface{}{
			"error": "invalid payload: cannot find ref inside given payload",
		})
		return
	}
	refString, ok := ref.(string)
	if !ok {
		Respond(w, 400, map[string]interface{}{
			"error": "type of ref is not string",
		})
		return
	}

	branchStringArr := strings.Split(refString, "/")
	branchString := branchStringArr[len(branchStringArr)-1]

	if project.Branch != branchString {
		Respond(w, 200, map[string]interface{}{
			"message": "request recieved but the push event is not for the configured branch",
		})
		return
	}

	// enqueing job

}
