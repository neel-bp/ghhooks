package core

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
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
var ResultMap *ResultSyncMap
var Ctx context.Context

// function that will be enqued by project specific queue
// DONE: make this func fit into queue job function prototype
// TODO: configurable shell
// TODO: configurable per command timeout
func Job(args ...any) error {
	projectName := args[0].(string)
	project := args[1].(Project)

	ResultMap.Mu.Lock()
	ResultMap.Map[projectName] = BuildStruct{
		LastBuildStart: time.Now(),
		StepResults:    make([]Result, 0),
	}
	ResultMap.Mu.Unlock()

	for _, step := range project.Steps {
		commandsWithArgs := strings.Split(step, " ")
		command := commandsWithArgs[0]
		var args []string
		if len(commandsWithArgs) > 1 {
			args = commandsWithArgs[1:]
		}
		//DONE: create context with deadline from global context
		ctx, cancel := context.WithTimeout(Ctx, time.Minute*10)
		cmd := exec.CommandContext(ctx, command, args...)
		cmd.Dir = project.Cwd
		out, err := cmd.Output()
		cancel()

		//reporting results
		ResultMap.Mu.RLock()
		project, ok := ResultMap.Map[projectName]
		if ok {
			buildTime := project.LastBuildStart
			steps := project.StepResults
			if len(steps) > 0 {
				steps = append(steps, Result{
					Error:  err,
					Output: string(out),
				})
			}
			ResultMap.Mu.RUnlock()
			ResultMap.Mu.Lock()
			ResultMap.Map[projectName] = BuildStruct{
				LastBuildStart: buildTime,
				StepResults:    steps,
			}
			ResultMap.Mu.Unlock()
		} else {
			ResultMap.Mu.RUnlock()
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			break
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
	ResultMap = &ResultSyncMap{
		Map: make(map[string]BuildStruct),
	}
	Ctx = context.Background()
	return nil
}
