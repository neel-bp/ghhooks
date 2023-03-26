package httpinterface

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ghhooks.com/hook/core"
	"ghhooks.com/hook/jobqueue"
	"github.com/gorilla/mux"
)

// DONE: verify signature before accepting webhook
// DONE: verify if branch is correct
// TODO: to add or not to add logger
// TODO: log with levels and cli flags
// TODO: think about error stack traces probably
// TODO: brainstorm about proxy server that will send to all the agents running on different servers (woold need CORS)
// TODO: golang ssh client is also an option
// DONE: create global resultmap in core package for keeping track of build results
// DONE: status route
// TODO: github commit status
// TODO: blocking build run
// TODO: html page for status
// TODO: maybe put password on status page to prevent from builds being cancelled by just anyone

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

	bodyInBytes, err := StreamToByte(r.Body)
	if err != nil {
		Respond(w, 400, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	hash := r.Header.Get("X-Hub-Signature-256")
	if hash != "" {
		verified, err := VerifySignature(bodyInBytes, hash, project.Secret)
		if err != nil {
			Respond(w, 500, map[string]any{
				"error": err.Error(),
			})
			return
		}
		if !verified {
			Respond(w, 412, map[string]any{
				"error": "signauture could not be verified",
			})
			return
		}
	}

	var payload WebhookPayload
	err = json.Unmarshal(bodyInBytes, &payload)
	if err != nil {
		Respond(w, 400, map[string]any{
			"error": err.Error(),
		})
		return
	}

	if payload.Ref == "" {
		Respond(w, 400, map[string]interface{}{
			"error": "invalid payload: cannot find ref inside given payload",
		})
		return
	}

	branchStringArr := strings.Split(payload.Ref, "/")
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
		Respond(w, http.StatusBadRequest, map[string]interface{}{
			"error": "no build steps configured",
		})
		return
	}
	core.ResultMap.Mu.RLock()
	result, ok := core.ResultMap.Map[projectID]
	core.ResultMap.Mu.RUnlock()
	if !ok {
		Respond(w, http.StatusBadRequest, map[string]interface{}{
			"error": "no build have been run yet, or nothing to report on the project",
		})
		return
	}

	var successfullSteps int
	for _, v := range result.StepResults {
		if v.Error == nil {
			successfullSteps = successfullSteps + 1
		}
	}

	Respond(w, 200, map[string]interface{}{
		"buildResult": result,
		"coverage":    fmt.Sprintf("%v%%", (successfullSteps*100)/totalSteps),
	})

}

func RouterInit(r *mux.Router) {
	r.HandleFunc("/{project}", WebHookListener).Methods("POST")
	r.HandleFunc("/{project}/", WebHookListener).Methods("POST")
	r.HandleFunc("/{project}/status", BuildStatus).Methods("GET")
	r.HandleFunc("/{project}/status/", BuildStatus).Methods("GET")
}
