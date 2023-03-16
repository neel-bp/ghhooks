package core

import (
	"context"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"ghhooks.com/hook/jobqueue"
	"github.com/BurntSushi/toml"
)

type Doc struct {
	Project map[string]Project
}

type Project struct {
	Branch string
	Secret string
	Cwd    string
	Steps  []string
}

// result processing is local to individual job
// result processing is only started after a job has been started

type Result struct {
	Error  error
	Output string
}

type BuildStruct struct {
	LastBuildStart time.Time
	StepResults    []Result
}

type ResultSyncMap struct {
	Mu  sync.RWMutex
	Map map[string]BuildStruct
}

// GLobals
var Queues jobqueue.QueueMap
var ServerConf Doc

// function that will be enqued by project specific queue
// make this func fit into queue job function prototype
func Job(projectName string, project Project, ctx context.Context, resultMap *ResultSyncMap) error {

	resultMap.Mu.Lock()
	resultMap.Map[projectName] = BuildStruct{
		LastBuildStart: time.Now(),
		StepResults:    make([]Result, 0),
	}
	resultMap.Mu.Unlock()

	for _, step := range project.Steps {
		commandsWithArgs := strings.Split(step, " ")
		command := commandsWithArgs[0]
		var args []string
		if len(commandsWithArgs) > 1 {
			args = commandsWithArgs[1:]
		}
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Dir = project.Cwd
		out, err := cmd.Output()

		//reporting results
		resultMap.Mu.RLock()
		project, ok := resultMap.Map[projectName]
		if ok {
			buildTime := project.LastBuildStart
			steps := project.StepResults
			if len(steps) > 0 {
				steps = append(steps, Result{
					Error:  err,
					Output: string(out),
				})
			}
			resultMap.Mu.RUnlock()
			resultMap.Mu.Lock()
			resultMap.Map[projectName] = BuildStruct{
				LastBuildStart: buildTime,
				StepResults:    steps,
			}
			resultMap.Mu.Unlock()
		} else {
			resultMap.Mu.RUnlock()
		}

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
	return nil
}
