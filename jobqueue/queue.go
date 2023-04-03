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
}

type QueueMap map[string]*JobQueue

// ============ types ==================

func NewJobQueue(pname string, pbuffer chan Job, pconcurrentWorkers uint64) *JobQueue {
	return &JobQueue{
		name:              pname,
		buffer:            pbuffer,
		concurrentWorkers: pconcurrentWorkers,
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

func (jq *JobQueue) startWorker(l *log.Logger, wg *sync.WaitGroup) {
	for j := range jq.buffer {
		err := j.Action(j.Args...)
		if err != nil {
			l.Println(err)
		}
		l.Println("job done")
	}
	wg.Done()
}

func (jq *JobQueue) StartWorkers(l *log.Logger, wg *sync.WaitGroup) {
	if jq.concurrentWorkers == 0 {
		jq.concurrentWorkers = 1
	}
	for i := 0; i < int(jq.concurrentWorkers); i++ {
		wg.Add(1)
		go jq.startWorker(l, wg)
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

func (q *QueueMap) StartAll(l *log.Logger, wg *sync.WaitGroup) {
	for k := range *q {
		(*q)[k].StartWorkers(l, wg)
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
