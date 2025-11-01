package utils

import "sync"

type ReadOnlyWaitGroup struct {
	wg  *sync.WaitGroup // For single wait group
	wgs []*ReadOnlyWaitGroup // For multiple wait groups
}

func NewReadOnlyWaitGroup(wg *sync.WaitGroup) *ReadOnlyWaitGroup {
	return &ReadOnlyWaitGroup{wg: wg}
}

func NewReadOnlyWaitGroupMulti(wgs []*ReadOnlyWaitGroup) *ReadOnlyWaitGroup {
	return &ReadOnlyWaitGroup{wgs: wgs}
}

func (rw *ReadOnlyWaitGroup) Wait() {
	if rw.wg != nil {
		// Single wait group
		rw.wg.Wait()
	} else if len(rw.wgs) > 0 {
		// Multiple wait groups - wait for all
		for _, wg := range rw.wgs {
			wg.Wait()
		}
	}
}
