package ws

import (
	"log"
	"net/http"
	"sync"
	"testing"

	"github.com/buger/goreplay/pkg/emitter"
	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/test"

	"github.com/gorilla/websocket"
)

func TestWebSocketOutput(t *testing.T) {
	wg := new(sync.WaitGroup)

	wsAddr := startWebsocket(func(data []byte) {
		wg.Done()
	})
	input := test.NewTestInput()
	output := NewWebSocketOutput(wsAddr, &WebSocketOutputConfig{Workers: 1})

	plugins := &plugin.InOutPlugins{
		Inputs:  []plugin.Reader{input},
		Outputs: []plugin.Writer{output},
	}

	emitter := emitter.New()
	go emitter.Start(plugins)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		input.EmitGET()
	}

	wg.Wait()
	emitter.Close()
}

func startWebsocket(cb func([]byte)) string {
	upgrader := websocket.Upgrader{}

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}

		go func(conn *websocket.Conn) {
			defer conn.Close()
			for {
				_, msg, _ := conn.ReadMessage()
				cb(msg)
			}
		}(c)
	})

	go func() {
		err := http.ListenAndServe("localhost:8081", nil)
		if err != nil {
			log.Fatal("Can't start:", err)
		}
	}()

	return "ws://localhost:8081/test"
}
