package core

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"ghhooks.com/hook/jobqueue"
	"github.com/BurntSushi/toml"
)

type Doc struct {
	Project map[string]Project `toml:"project"`
}

type Project struct {
	Branch      string     `toml:"branch"`
	Secret      string     `toml:"secret"`
	Cwd         string     `toml:"cwd"`
	Steps       [][]string `toml:"steps"`
	StepTimeout int        `toml:"stepTimeout"`
}

// result processing is local to individual job
// result processing is only started after a job has been started

type Result struct {
	Error       error  `json:"error"`
	Output      string `json:"output"`
	Description string `json:"description"`
}

// DONE: build status
type JobState struct {
	LastBuildStart time.Time `json:"lastBuildStart"`
	StepResults    []Result  `json:"stepResults"`
	BuildStatus    string    `json:"buildStatus"`
	Coverage       float64   `json:"coverage"`
}

type StatusResponse struct {
	JobState
	ProjectName    string `json:"projectName"`
	DateTimeString string `json:"dateTimeString"`
}

type ResultSyncMap struct {
	Mu  sync.RWMutex
	Map map[string]JobState
}

const (
	PENDING string = "pending"
	FAILED  string = "failed"
	SUCCESS string = "success"
)

// GLobals
var Queues jobqueue.QueueMap
var ServerConf Doc
var ResultMap *ResultSyncMap
var Ctx context.Context

// function that will be enqued by project specific queue
// DONE: make this func fit into queue job function prototype
// TODO: configurable shell
// DONE: configurable per command timeout
// TODO: multiserver install scripts (like ansible playbook) using golang ssh client
// DONE: along with error object add error description too (err.Error())
// TODO: cancel running build
func Job(args ...any) error {
	projectName := args[0].(string)
	project := args[1].(Project)

	ResultMap.Mu.Lock()
	ResultMap.Map[projectName] = JobState{
		LastBuildStart: time.Now().UTC(),
		StepResults:    make([]Result, 0),
		BuildStatus:    PENDING,
	}
	ResultMap.Mu.Unlock()

	for _, step := range project.Steps {

		if len(step) == 0 {
			fmt.Fprintln(os.Stderr, "empty step")
			continue
		}

		command := step[0]
		var args []string
		if len(step) > 1 {
			args = step[1:]
		}

		//DONE: create context with deadline from global context
		duration := 10 * time.Minute
		if project.StepTimeout != 0 {
			duration = time.Duration(project.StepTimeout) * time.Second
		}
		ctx, cancel := context.WithTimeout(Ctx, duration)
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Dir = project.Cwd
		out, err := cmd.Output()
		cancel()

		//reporting results
		ResultMap.Mu.RLock()
		project, ok := ResultMap.Map[projectName]
		ResultMap.Mu.RUnlock()
		if ok {
			buildTime := project.LastBuildStart
			steps := project.StepResults
			description := "step ran successfully"
			if err != nil {
				description = err.Error()
			}
			steps = append(steps, Result{
				Error:       err,
				Output:      string(out),
				Description: description,
			})

			ResultMap.Mu.Lock()
			ResultMap.Map[projectName] = JobState{
				LastBuildStart: buildTime,
				StepResults:    steps,
			}
			ResultMap.Mu.Unlock()
		} else {
			ResultMap.Mu.RUnlock()
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			ResultMap.Mu.RLock()
			obj := ResultMap.Map[projectName]
			ResultMap.Mu.RUnlock()
			obj.BuildStatus = FAILED
			ResultMap.Mu.Lock()
			ResultMap.Map[projectName] = obj
			ResultMap.Mu.Unlock()
			break
		}
	}
	ResultMap.Mu.RLock()
	obj := ResultMap.Map[projectName]
	ResultMap.Mu.RUnlock()
	if obj.BuildStatus != FAILED {
		obj.BuildStatus = SUCCESS
		ResultMap.Mu.Lock()
		ResultMap.Map[projectName] = obj
		ResultMap.Mu.Unlock()
	}
	return nil
}

func ConfigParser(fileLocation string) (Doc, error) {
	var doc Doc
	b, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return doc, err
	}
	_, err = toml.Decode(string(b), &doc)
	if err != nil {
		return doc, err
	}
	return doc, nil

}

func ServerInit(configlocation string, l *log.Logger) error {
	conf, err := ConfigParser(configlocation)
	if err != nil {
		return err
	}
	ServerConf = conf
	Queues = make(jobqueue.QueueMap, 0)
	for projectName, _ := range ServerConf.Project {
		jg := jobqueue.NewJobQueue(projectName, make(chan jobqueue.Job, 25), 1)
		err = Queues.Register(jg)
		if err != nil {
			return err
		}

	}
	Queues.StartAll(l)
	ResultMap = &ResultSyncMap{
		Map: make(map[string]JobState),
	}
	Ctx = context.Background()
	return nil
}
