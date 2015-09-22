package main

import (
	"io"
	"time"
)

// Start initialize loop for sending data from inputs to outputs
func Start(stop chan int) {
	var readers []io.Reader
	var writers []io.Writer

	for _, plugin := range Plugins {
		pluginWrapper := plugin

		if l, ok := plugin.(*Limiter); ok {
			plugin = l.plugin
		}

		if _, isR := plugin.(io.Reader); isR {
			readers = append(readers, pluginWrapper.(io.Reader))
		}

		if _, isW := plugin.(io.Writer); isW {
			writers = append(writers, pluginWrapper.(io.Writer))
		}
	}

	if len(Middleware) > 0 {
		// All readers report to first middleware in pipeline
		for _, reader := range readers {
			go CopyMulty(reader, Middleware[0])
		}

		// Middleware pipeline
		for i, mw := range Middleware {
			if i < len(Middleware)-1 {
				go CopyMulty(mw, Middleware[i+1])
			}
		}

		// Last middleware in pipeline report to writers
		go CopyMulty(Middleware[len(Middleware)-1], writers...)
	} else {
		for _, in := range readers {
			go CopyMulty(in, writers...)
		}
	}

	for {
		select {
		case <-stop:
			return
		case <-time.After(time.Second):
		}
	}
}

// CopyMulty copies from 1 reader to multiple writers
func CopyMulty(src io.Reader, writers ...io.Writer) (err error) {
	buf := make([]byte, 5*1024*1024)
	wIndex := 0

	for {
		nr, er := src.Read(buf)

		if nr > 0 && len(buf) > nr {
			payload := buf[:nr]

			if Settings.debug {
				Debug("[EMITTER] input:", stringLimit(payload), nr, "from:", src)
			}

			if Settings.splitOutput && len(writers) > 1 {
				// Simple round robin
				writers[wIndex].Write(payload)

				wIndex++

				if wIndex >= len(writers) {
					wIndex = 0
				}
			} else {
				for _, dst := range writers {
					dst.Write(payload)
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
