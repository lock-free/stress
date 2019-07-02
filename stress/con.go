package stress

import "sync"

// repeat a task with max limitation

type Coner struct {
	maxRunning int
	count      int
	countMutex *sync.Mutex
}

func GetConer(maxRunning int) Coner {
	var mutex = &sync.Mutex{}
	return Coner{
		maxRunning: maxRunning,
		count:      0,
		countMutex: mutex,
	}
}

// @return, run or reject
func (coner *Coner) Run(taskFun func()) bool {
	if coner.occupy() {
		go (func() {
			taskFun()
			coner.decc()
		})()
		return true
	}
	return false
}

// occupy resource
func (coner *Coner) occupy() bool {
	coner.countMutex.Lock()
	defer coner.countMutex.Unlock()

	// still have resources
	if coner.count < coner.maxRunning {
		coner.count++
		return true
	}

	return false
}

// release resource
func (coner *Coner) decc() {
	coner.countMutex.Lock()
	defer coner.countMutex.Unlock()

	coner.count--
}
