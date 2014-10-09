package main

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
func CopyMulty(src io.Reader, writers ...io.Writer) (err error) {
	buf := make([]byte, 32*1024)
	wIndex := 0

	for {
		nr, er := src.Read(buf)
		if nr > 0 && len(buf) > nr{
			Debug("Sending", src, ": ", string(buf[0:nr]))

			if Settings.splitOutput {
				// Simple round robin
				writers[wIndex].Write(buf[0:nr])

				wIndex++

				if wIndex >= len(writers) {
					wIndex = 0
				}
			} else {
				for _, dst := range writers {
					dst.Write(buf[0:nr])
				}
			}

		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return err
}
