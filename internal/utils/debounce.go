package utils

import (
	"sync"
	"time"
)

func CreateDebouncedCallback(delay time.Duration, callback func(string)) func(string) {
	var timer *time.Timer
	var mu sync.Mutex

	return func(value string) {
		mu.Lock()
		defer mu.Unlock()

		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(delay, func() {
			callback(value)
		})
	}
}
