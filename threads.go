package main

import "sync"

type Threads struct {
	size      int
	allocated int
	wg        *sync.WaitGroup
	mutex     *sync.Mutex
	rw_lock   *sync.RWMutex
}

func NewThreadPool(size int) *Threads {
	return &Threads{
		size:  size,
		wg:    &sync.WaitGroup{},
		mutex: &sync.Mutex{},
	}
}

func (t *Threads) Add() {
	t.rw_lock.RLock()
	if t.allocated == t.size {
		t.mutex.Lock()
	}

	t.rw_lock.RUnlock()

	t.rw_lock.Lock()
	t.allocated++
	t.rw_lock.Unlock()

	t.wg.Add(1)

}

func (t *Threads) Increase(new_size int) {

}

func (t *Threads) AddWithReserve(reserve_size int) {
	t.rw_lock.Lock()
	t.wg.Add(reserve_size)

	t.allocated += reserve_size
	t.rw_lock.Unlock()
}

func (t *Threads) Done() {
	t.rw_lock.Lock()
	if t.allocated == t.size {
		t.mutex.Unlock()
	}

	t.allocated--
	t.rw_lock.Unlock()
}
