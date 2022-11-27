package main

type Progress struct {
	Current float32
	Total   float32
	Info    interface{}
	End     bool
	Reset   bool
}

type Message string
