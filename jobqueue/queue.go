package jobqueue

import (
	"errors"
	"log"
	"sync"
)

// TODO: write closing of queues with graceful shutdown

// errors ===============================

var ErrQueueAlreadyRegistered = errors.New("queue is already registered")

// errors ===============================

// ============ types ==================

type Job struct {
	Name   string
	Action func(...any) error
	Args   []any
}

type JobQueue struct {
	name              string
	buffer            chan Job
	concurrentWorkers uint64
	l                 *log.Logger
	wg                *sync.WaitGroup
}

type QueueMap map[string]*JobQueue

// ============ types ==================

func NewJobQueue(pname string, pbuffer chan Job, pconcurrentWorkers uint64, l *log.Logger, wg *sync.WaitGroup) *JobQueue {
	return &JobQueue{
		name:              pname,
		buffer:            pbuffer,
		concurrentWorkers: pconcurrentWorkers,
		l:                 l,
		wg:                wg,
	}
}

func (jq *JobQueue) Enqueue(job Job) bool {
	select {
	case jq.buffer <- job:
		return true
	default:
		return false
	}
}

func (jq *JobQueue) startWorker() {
	for j := range jq.buffer {
		err := j.Action(j.Args...)
		if err != nil {
			jq.l.Println(err)
		}
		jq.l.Println("job done")
	}
	jq.wg.Done()
}

func (jq *JobQueue) StartWorkers() {
	if jq.concurrentWorkers == 0 {
		jq.concurrentWorkers = 1
	}
	for i := 0; i < int(jq.concurrentWorkers); i++ {
		jq.wg.Add(1)
		go jq.startWorker()
	}
}

func (jg *JobQueue) Drain() {
	close(jg.buffer)
	for len(jg.buffer) > 0 {
		<-jg.buffer
	}
}

func (q *QueueMap) Register(jq *JobQueue) error {
	if _, ok := (*q)[jq.name]; ok {
		return ErrQueueAlreadyRegistered
	}
	(*q)[jq.name] = jq
	return nil
}

func (q *QueueMap) StartAll() {
	for k := range *q {
		(*q)[k].StartWorkers()
	}
}

func (q *QueueMap) DrainAll() {
	for k := range *q {
		(*q)[k].Drain()
	}
}

func (q *QueueMap) Enqueue(queueName string, job Job) bool {
	return (*q)[queueName].Enqueue(job)
}
