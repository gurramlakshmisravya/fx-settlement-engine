package worker

import (
	"context"
	"log"
	"sync"
)

type Task func(ctx context.Context) error

type WorkerPool struct {
	numWorkers int
	taskQueue  chan Task
	wg         sync.WaitGroup
}

func NewWorkerPool(numWorkers int, queueSize int) *WorkerPool {
	return &WorkerPool{
		numWorkers: numWorkers,
		taskQueue:  make(chan Task, queueSize),
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 1; i <= wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
	log.Printf("[Worker Pool] Initialized %d concurrent workers", wp.numWorkers)
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-wp.taskQueue:
			if !ok {
				return
			}
			if err := task(ctx); err != nil {
				log.Printf("[Worker %d] Task execution error: %v", id, err)
			}
		}
	}
}

func (wp *WorkerPool) Submit(task Task) bool {
	select {
	case wp.taskQueue <- task:
		return true
	default:
		log.Println("[Worker Pool] Queue full, task dropped or running synchronously")
		return false
	}
}

func (wp *WorkerPool) Stop() {
	close(wp.taskQueue)
	wp.wg.Wait()
	log.Println("[Worker Pool] Stopped all workers")
}
