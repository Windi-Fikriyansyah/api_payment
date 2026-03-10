package services

import (
	"fmt"
	"sync"
)

type WorkerPool struct {
	tasks chan func()
	wg    sync.WaitGroup
}

func NewWorkerPool(size int) *WorkerPool {
	p := &WorkerPool{
		tasks: make(chan func(), size*10),
	}
	p.wg.Add(size)
	for i := 0; i < size; i++ {
		go func() {
			defer p.wg.Done()
			for task := range p.tasks {
				task()
			}
		}()
	}
	return p
}

func (p *WorkerPool) Submit(task func()) {
	p.tasks <- task
}

func (p *WorkerPool) Shutdown() {
	close(p.tasks)
	p.wg.Wait()
}

func (p *WorkerPool) LogTask(name string) {
	fmt.Printf("[Worker] Executing task: %s\n", name)
}
