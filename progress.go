package main

import (
	"fmt"
	"sync"
)

type Progress struct {
	Current float32
}

type ProgressWatcher struct {
	mutex   sync.Mutex
	Total   float32
	Current float32
	Title   string
}

func NewProgressWatcher(title string) *ProgressWatcher {
	return &ProgressWatcher{
		Title: title,
	}
}

func (pw *ProgressWatcher) SetTotal(total float32) {
	pw.mutex.Lock()
	pw.Total = total
	pw.mutex.Unlock()
}

func (pw *ProgressWatcher) SetTotalFunc(function func(current float32) float32) {
	pw.mutex.Lock()
	pw.Total = function(pw.Total)
	fmt.Printf("\nProgress of %s - %f/%f\n\n", pw.Title, pw.Current, pw.Total)
	pw.mutex.Unlock()
}

func (pw *ProgressWatcher) SetCurrent(value float32) {
	pw.mutex.Lock()
	pw.Current = value
	fmt.Printf("\nProgress of %s - %f/%f\n\n", pw.Title, pw.Current, pw.Total)
	pw.mutex.Unlock()
}

func (pw *ProgressWatcher) SetCurrentFunc(function func(current float32) float32) {
	pw.mutex.Lock()
	pw.Current = function(pw.Current)
	fmt.Printf("\nProgress of %s - %f/%f\n\n", pw.Title, pw.Current, pw.Total)
	pw.mutex.Unlock()
}

type Message string
