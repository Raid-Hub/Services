package utils

import "sync"

type ReadOnlyWaitGroup struct {
	wg *sync.WaitGroup
}

func NewReadOnlyWaitGroup(wg *sync.WaitGroup) ReadOnlyWaitGroup {
	return ReadOnlyWaitGroup{wg: wg}
}

func (rw *ReadOnlyWaitGroup) Wait() {
	rw.wg.Wait()
}
