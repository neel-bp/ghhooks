package httpinterface

import (
	"encoding/json"
	"net/http"
	"strings"

	"ghhooks.com/hook/core"
	"ghhooks.com/hook/jobqueue"
	"github.com/gorilla/mux"
)

// TODO: verify signature before accepting webhook
// DONE: verify if branch is correct
// TODO: to add or not to add logger
// TODO: log with levels and cli flags
// TODO: think about error stack traces probably
// TODO: brainstorm about proxy server that will send to all the agents running on different servers (woold need CORS)
// TODO: golang ssh client is also an option
// DONE: create global resultmap in core package for keeping track of build results
// DONE: status route
// TODO: github commit status

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
	enqueueStatus := core.Queues.Enqueue(projectID, jobqueue.Job{
		Name:   "build",
		Action: core.Job,
		Args: []any{
			projectID,
			project,
		},
	})
	if !enqueueStatus {
		Respond(
			w, 429, map[string]interface{}{
				"error": "build queue is full",
			},
		)
		return
	}
	Respond(w, 201, map[string]interface{}{
		"message": "build queued successfully",
	})
}

func BuildStatus(w http.ResponseWriter, r *http.Request) {
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

	totalSteps := len(project.Steps)
	if totalSteps == 0 {
		Respond(w, http.StatusBadGateway, map[string]interface{}{
			"error": "no build steps configured",
		})
		return
	}
	core.ResultMap.Mu.RLock()
	result, ok := core.ResultMap.Map[projectID]
	core.ResultMap.Mu.Unlock()
	if !ok {
		Respond(w, http.StatusBadGateway, map[string]interface{}{
			"error": "no build have been run yet, or nothing to report on the project",
		})
		return
	}
	Respond(w, 200, map[string]interface{}{
		"buildResult":      result,
		"completionStatus": (len(result.StepResults) * 100) / totalSteps,
	})

}

func RouterInit(r *mux.Router) {
	r.HandleFunc("/{project}", WebHookListener).Methods("POST")
	r.HandleFunc("/{project}/", WebHookListener).Methods("POST")
	r.HandleFunc("/{project}/status", BuildStatus).Methods("GET")
	r.HandleFunc("/{project}/status/", BuildStatus).Methods("GET")
}
