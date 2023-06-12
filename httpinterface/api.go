package httpinterface

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"ghhooks.com/hook/core"
	"ghhooks.com/hook/jobqueue"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// DONE: verify signature before accepting webhook
// DONE: verify if branch is correct
// TODO: log with levels and cli flags
// TODO: think about error stack traces probably
// TODO: brainstorm about proxy server that will send to all the agents running on different servers (woold need CORS)
// TODO: golang ssh client is also an option
// DONE: create global resultmap in core package for keeping track of build results
// DONE: status route
// TODO: github commit status
// TODO: blocking build run
// DONE: html page for status
// TODO: maybe put password on status page to prevent from builds being cancelled by just anyone
// DONE: update progressbar using websockets,
// DONE: individual step results on statuspage
// TODO: investigate what happens if websocket events come faster than the time it takes for event to process

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	SUCCESS_MARK = "✓"
	PENDING_MARK = "❍"
	FAILED_MARK  = "✗"
)

type Step struct {
	Command       string `json:"command"`
	Status        string `json:"status"`
	CommandOutput string `json:"commandOutput"`
	Description   string `json:"description"`
}

type StatusResponse struct {
	core.JobState
	ProjectName    string       `json:"projectName"`
	DateTimeString string       `json:"dateTimeString"`
	Coverage       float64      `json:"coverage"`
	WebSocketRoute template.URL `json:"websocketRoute"`
	Steps          []Step       `json:"steps"`
}

type WebsocketResponse struct {
	core.JobState
	Coverage    float64 `json:"coverage"`
	ProjectName string  `json:"projectName"`
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

	eventType := r.Header.Get("X-GitHub-Event")
	err = VerifyEvent(eventType, bodyInBytes, project.Branch)

	if err != nil {
		Respond(w, 400, map[string]interface{}{
			"error": fmt.Sprintf("error: %v.", err),
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
	//NOTE: to debug script on statuspage add //# sourceURL=statuspage at end of script above closing tag

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

	coverage := float64(successfullSteps * 100 / totalSteps)

	format := r.URL.Query().Get("format")

	if format == "json" {
		Respond(w, 200, result)
		return
	}

	projectSteps := make([]Step, 0)
	for i, step := range project.Steps {
		step_ := Step{
			Command: strings.Join(step, " "),
			Status:  PENDING_MARK,
		}
		if i <= len(result.StepResults)-1 {
			res := result.StepResults[i]
			step_.CommandOutput = res.Output
			step_.Description = res.Description
			if res.Error != nil {
				step_.Status = FAILED_MARK
			} else {
				step_.Status = SUCCESS_MARK
			}
		}

		projectSteps = append(projectSteps, step_)

	}

	templateResponse := StatusResponse{
		JobState:       result,
		ProjectName:    projectID,
		DateTimeString: result.LastBuildStart.Format(time.RFC3339),
		Coverage:       coverage,
		WebSocketRoute: template.URL(fmt.Sprintf("ws://%s/%s/livestatus", r.Host, projectID)),
		Steps:          projectSteps,
	}

	tmpl := template.Must(template.ParseFiles("statuspage.html"))
	tmpl.Execute(w, templateResponse)

}

func LiveStatusUpdate(w http.ResponseWriter, r *http.Request) {
	//NOTE: to debug script on statuspage add //# sourceURL=statuspage at end of script above closing tag

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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		Respond(w, 500, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	defer conn.Close()

	for jobState := range core.LiveResultUpdates[projectID] {

		var successfullSteps int
		for _, v := range jobState.StepResults {
			if v.Error == nil {
				successfullSteps = successfullSteps + 1
			}
		}

		coverage := float64(successfullSteps * 100 / totalSteps)

		res := WebsocketResponse{
			JobState:    jobState,
			ProjectName: projectID,
			Coverage:    coverage,
		}
		err := conn.WriteJSON(res)
		if err != nil {
			break
		}
	}

}

func RouterInit(r *mux.Router) {
	r.HandleFunc("/{project}", WebHookListener).Methods("POST")
	r.HandleFunc("/{project}/", WebHookListener).Methods("POST")
	r.HandleFunc("/{project}/status", BuildStatus).Methods("GET")
	r.HandleFunc("/{project}/status/", BuildStatus).Methods("GET")
	r.HandleFunc("/{project}/livestatus", LiveStatusUpdate)
	r.HandleFunc("/{project}/livestatus/", LiveStatusUpdate)
}
