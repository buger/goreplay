package gor

import (
	"io"
)

func Start(stop chan int) {
	for _, in := range Plugins.Inputs {
		go CopyMulty(in, Plugins.Outputs...)
	}

	select {
	case <-stop:
		return
	}
}

// Copy from 1 reader to multiple writers
func CopyMulty(src Input, writers ...io.Writer) {
	wIndex := 0

	for {
		buf, ok := src.Read()
		if ok {
			Debug("Sending", src, ": ", string(buf))

			if Settings.splitOutput {
				// Simple round robin
				writers[wIndex].Write(buf)

				wIndex++

				if wIndex >= len(writers) {
					wIndex = 0
				}
			} else {
				for _, dst := range writers {
					dst.Write(buf)
				}
			}

		} else {
			break
		}
	}
}
