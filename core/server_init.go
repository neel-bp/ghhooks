package core

import (
	"context"
	"io/ioutil"
	"log"
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
// map key in resultSyncMap is projectID, the reason behind this is to have independent project build results

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
var LiveResultUpdates map[string]chan JobState

// function that will be enqued by project specific queue
// DONE: make this func fit into queue job function prototype
// TODO: configurable shell
// DONE: configurable per command timeout
// TODO: multiserver install scripts (like ansible playbook) using golang ssh client
// DONE: along with error object add error description too (err.Error())
// TODO: cancel running build

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

func ServerInit(configlocation string, l *log.Logger, wg *sync.WaitGroup) error {
	conf, err := ConfigParser(configlocation)
	if err != nil {
		return err
	}
	ServerConf = conf
	Queues = make(jobqueue.QueueMap, 0)
	LiveResultUpdates = make(map[string]chan JobState)
	for projectName, project := range ServerConf.Project {
		jg := jobqueue.NewJobQueue(projectName, make(chan jobqueue.Job, 25), 1, l, wg)
		err = Queues.Register(jg)
		if err != nil {
			return err
		}
		LiveResultUpdates[projectName] = make(chan JobState, len(project.Steps)+1)

	}
	Queues.StartAll()
	ResultMap = &ResultSyncMap{
		Map: make(map[string]JobState),
	}

	Ctx = context.Background()
	return nil
}
