package simpletime

import (
	"time"
)

var Now time.Time

func init() {
	go func() {
		for {
			// Accurate enough
			Now = time.Now()
			time.Sleep(100 * time.Millisecond)
		}
	}()
}
