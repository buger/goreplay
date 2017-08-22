package rawSocket

import (
	"bytes"
	"log"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

func TestRawListenerInput(t *testing.T) {
	var req, resp *TCPMessage

	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket := buildPacket(true, 1, 1, []byte("GET / HTTP/1.1\r\n\r\n"), time.Now())

	respAck := reqPacket.Seq + uint32(len(reqPacket.Data))
	respPacket := buildPacket(false, respAck, reqPacket.Seq+1, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	listener.packetsChan <- reqPacket.dump()
	listener.packetsChan <- respPacket.dump()

	select {
	case req = <-listener.messagesChan:
	case <-time.After(time.Millisecond):
		t.Error("Should return request immediately")
		return
	}

	if !req.IsIncoming {
		t.Error("Should be request")
	}

	select {
	case resp = <-listener.messagesChan:
	case <-time.After(20 * time.Millisecond):
		t.Error("Should return response immediately")
		return
	}

	if resp.IsIncoming {
		t.Error("Should be response")
	}
}

func TestRawListenerInputResponseByClose(t *testing.T) {
	var req, resp *TCPMessage

	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket := buildPacket(true, 1, 1, []byte("GET / HTTP/1.1\r\n\r\n"), time.Now())

	respAck := reqPacket.Seq + uint32(len(reqPacket.Data))
	respPacket := buildPacket(false, respAck, reqPacket.Seq+1, []byte("HTTP/1.1 200 OK\r\nConnection: close\r\n\r\nasd"), time.Now())
	finPacket := buildPacket(false, respAck, reqPacket.Seq+2, []byte(""), time.Now())
	finPacket.IsFIN = true

	listener.packetsChan <- reqPacket.dump()
	listener.packetsChan <- respPacket.dump()
	listener.packetsChan <- finPacket.dump()

	select {
	case req = <-listener.messagesChan:
	case <-time.After(time.Millisecond):
		t.Error("Should return request immediately")
		return
	}

	if !req.IsIncoming {
		t.Error("Should be request")
	}

	select {
	case resp = <-listener.messagesChan:
	case <-time.After(20 * time.Millisecond):
		t.Error("Should return response immediately")
		return
	}

	if resp.IsIncoming {
		t.Error("Should be response")
	}
}

func TestRawListenerInputWithoutResponse(t *testing.T) {
	var req *TCPMessage

	listener := NewListener("", "0", EnginePcap, false, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket := buildPacket(true, 1, 1, []byte("GET / HTTP/1.1\r\n\r\n"), time.Now())

	listener.packetsChan <- reqPacket.dump()

	select {
	case req = <-listener.messagesChan:
	case <-time.After(time.Millisecond):
		t.Error("Should return request immediately")
		return
	}

	if !req.IsIncoming {
		t.Error("Should be request")
	}
}

func TestRawListenerResponse(t *testing.T) {
	var req, resp *TCPMessage

	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket := buildPacket(true, 1, 1, []byte("GET / HTTP/1.1\r\n\r\n"), time.Now())
	respPacket := buildPacket(false, 1+uint32(len(reqPacket.Data)), 2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	// If response packet comes before request
	listener.packetsChan <- respPacket.dump()
	listener.packetsChan <- reqPacket.dump()

	select {
	case req = <-listener.messagesChan:
	case <-time.After(time.Millisecond):
		t.Error("Should return respose immediately")
		return
	}

	if !req.IsIncoming {
		t.Error("Should be request")
	}

	select {
	case resp = <-listener.messagesChan:
	case <-time.After(time.Millisecond):
		t.Error("Should return response immediately")
		return
	}

	if resp.IsIncoming {
		t.Error("Should be response")
	}

	if !bytes.Equal(resp.UUID(), req.UUID()) {
		t.Error("Resp and Req UUID should be equal")
	}
}

func TestShort100Continue(t *testing.T) {
	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket1 := buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\nExpect: 100-continue\r\n\r\n"), time.Now())
	// Packet with data have different Seq
	reqPacket2 := buildPacket(true, 2, reqPacket1.Seq+uint32(len(reqPacket1.Data)), []byte("a"), time.Now())
	reqPacket3 := buildPacket(true, 2, reqPacket2.Seq+1, []byte("b"), time.Now())

	respPacket1 := buildPacket(false, 10, 3, []byte("HTTP/1.1 100 Continue\r\n\r\n"), time.Now())

	// panic(int(uint32(len(reqPacket1.Data)) + uint32(len(reqPacket2.Data)) + uint32(len(reqPacket3.Data))))
	respPacket2 := buildPacket(false, reqPacket3.Seq+1 /* len of data */, 2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	result := []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\nab")

	testRawListener100Continue(t, listener, result, reqPacket1, reqPacket2, reqPacket3, respPacket1, respPacket2)
}

// Response comes before Request
func Test100ContinueWrongOrder(t *testing.T) {
	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket1 := buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\nExpect: 100-continue\r\n\r\n"), time.Now())
	// Packet with data have different Seq
	reqPacket2 := buildPacket(true, 2, reqPacket1.Seq+uint32(len(reqPacket1.Data)), []byte("a"), time.Now())
	reqPacket3 := buildPacket(true, 2, reqPacket2.Seq+1, []byte("b"), time.Now())

	respPacket1 := buildPacket(false, 10, 3, []byte("HTTP/1.1 100 Continue\r\n"), time.Now())

	// panic(int(uint32(len(reqPacket1.Data)) + uint32(len(reqPacket2.Data)) + uint32(len(reqPacket3.Data))))
	respPacket2 := buildPacket(false, reqPacket3.Seq+1 /* len of data */, 2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	result := []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\nab")

	testRawListener100Continue(t, listener, result, respPacket1, respPacket2, reqPacket1, reqPacket2, reqPacket3)
}

func TestAlt100ContinueHeaderOrder(t *testing.T) {
	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket1 := buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 2\r\n\r\n"), time.Now())
	// Packet with data have different Seq
	reqPacket2 := buildPacket(true, 2, reqPacket1.Seq+uint32(len(reqPacket1.Data)), []byte("a"), time.Now())
	reqPacket3 := buildPacket(true, 2, reqPacket2.Seq+1, []byte("b"), time.Now())

	respPacket1 := buildPacket(false, 10, 3, []byte("HTTP/1.1 100 Continue\r\n"), time.Now())

	// panic(int(uint32(len(reqPacket1.Data)) + uint32(len(reqPacket2.Data)) + uint32(len(reqPacket3.Data))))
	respPacket2 := buildPacket(false, reqPacket3.Seq+1 /* len of data */, 2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	result := []byte("POST / HTTP/1.1\r\nContent-Length: 2\r\n\r\nab")

	testRawListener100Continue(t, listener, result, reqPacket1, reqPacket2, reqPacket3, respPacket1, respPacket2)
}

func testRawListener100Continue(t *testing.T, listener *Listener, result []byte, packets ...*TCPPacket) {
	var req, resp *TCPMessage
	for _, p := range packets {
		listener.packetsChan <- p.dump()
	}

	select {
	case req = <-listener.messagesChan:
		break
	case <-time.After(11 * time.Millisecond):
		t.Error("Should return response after expire time")
		return
	}

	if !bytes.Equal(req.Bytes(), result) {
		t.Error("Should receive full message", string(req.Bytes()))
	}

	if !req.IsIncoming {
		t.Error("Should be request")
	}

	select {
	case resp = <-listener.messagesChan:
		break
	case <-time.After(21 * time.Millisecond):
		t.Error("Should return response after expire time")
		return
	}

	if resp.IsIncoming {
		t.Error("Should be response")
	}

	if !bytes.Equal(resp.UUID(), req.UUID()) {
		t.Error("Resp and Req UUID should be equal")
	}
}

func testChunkedSequence(t *testing.T, listener *Listener, packets ...*TCPPacket) {
	var r, req, resp *TCPMessage

	for _, p := range packets {
		listener.packetsChan <- p.dump()
	}

	select {
	case r = <-listener.messagesChan:
		if r.IsIncoming {
			req = r
		} else {
			resp = r
		}
		break
	case <-time.After(25 * time.Millisecond):
		t.Error("Should return request after expire time")
		return
	}
	select {
	case r = <-listener.messagesChan:
		if r.IsIncoming {
			if req != nil {
				t.Error("Request already received", r)
				return
			}
			req = r
		} else {
			if resp != nil {
				t.Error("Response already received", r)
				return
			}
			resp = r
		}
		break
	case <-time.After(25 * time.Millisecond):
		t.Error("Should return request after expire time")
		return
	}

	if !bytes.Equal(req.Bytes(), []byte("POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n1\r\na\r\n1\r\nb\r\n0\r\n\r\n")) {
		t.Error("Should receive full message", string(req.Bytes()))
	}

	if !req.IsIncoming {
		t.Error("Should be request")
	}

	if resp.IsIncoming {
		t.Error("Should be response")
	}

	if !bytes.Equal(resp.UUID(), req.UUID()) {
		t.Error("Resp and Req UUID should be equal", string(resp.UUID()), string(req.UUID()))
	}

	time.Sleep(20 * time.Millisecond)

	if len(listener.packetsChan) != 0 {
		t.Fatal("packetsChan non empty:", listener.packetsChan)
	}

	if len(listener.messagesChan) != 0 {
		t.Fatal("messagesChan non empty:", <-listener.messagesChan)
	}

	if len(listener.messages) != 0 {
		t.Fatal("Messages non empty:", listener.messages)
	}

	if len(listener.ackAliases) != 0 {
		t.Fatal("ackAliases non empty:", listener.ackAliases)
	}

	if len(listener.seqWithData) != 0 {
		t.Fatal("seqWithData non empty:", listener.seqWithData)
	}

	if len(listener.respAliases) != 0 {
		t.Fatal("respAliases non empty:", listener.respAliases)
	}

	if len(listener.respWithoutReq) != 0 {
		t.Fatal("respWithoutReq non empty:", listener.respWithoutReq)
	}
}

func permutation(n int, list []*TCPPacket) []*TCPPacket {
	if len(list) == 1 {
		return list
	}

	k := n % len(list)

	first := []*TCPPacket{list[k]}
	next := make([]*TCPPacket, len(list)-1)

	copy(next, append(list[:k], list[k+1:]...))

	return append(first, permutation(n/len(list), next)...)
}

// Response comes before Request
func TestRawListenerChunkedWrongOrder(t *testing.T) {
	listener := NewListener("", "0", EnginePcap, true, 10*time.Millisecond, "")
	defer listener.Close()

	reqPacket1 := buildPacket(true, 1, 1, []byte("POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\nExpect: 100-continue\r\n\r\n"), time.Now())
	// Packet with data have different Seq
	reqPacket2 := buildPacket(true, 2, reqPacket1.Seq+uint32(len(reqPacket1.Data)), []byte("1\r\na\r\n"), time.Now())
	reqPacket3 := buildPacket(true, 2, reqPacket2.Seq+uint32(len(reqPacket2.Data)), []byte("1\r\nb\r\n"), time.Now())
	reqPacket4 := buildPacket(true, 2, reqPacket3.Seq+uint32(len(reqPacket3.Data)), []byte("0\r\n\r\n"), time.Now())

	respPacket1 := buildPacket(false, 10, 3, []byte("HTTP/1.1 100 Continue\r\n\r\n"), time.Now())

	// panic(int(uint32(len(reqPacket1.Data)) + uint32(len(reqPacket2.Data)) + uint32(len(reqPacket3.Data))))
	respPacket2 := buildPacket(false, reqPacket4.Seq+5 /* len of data */, 2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	// Should re-construct message from all possible combinations
	for i := 0; i < 6*5*4*3*2*1; i++ {

		// if i < 54 || i > 57 {
		// 	continue
		// }

		packets := permutation(i, []*TCPPacket{reqPacket1, reqPacket2, reqPacket3, reqPacket4, respPacket1, respPacket2})

		t.Log("permutation:", i)
		testChunkedSequence(t, listener, packets...)
	}
}

func chunkedPostMessage() []*TCPPacket {
	ack := uint32(rand.Int63())
	seq := uint32(rand.Int63())

	reqPacket1 := buildPacket(true, ack, seq, []byte("POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n"), time.Now())
	// Packet with data have different Seq
	reqPacket2 := buildPacket(true, ack, seq+47, []byte("1\r\na\r\n"), time.Now())
	reqPacket3 := buildPacket(true, ack, reqPacket2.Seq+5, []byte("1\r\nb\r\n"), time.Now())
	reqPacket4 := buildPacket(true, ack, reqPacket3.Seq+5, []byte("0\r\n\r\n"), time.Now())

	respPacket := buildPacket(false, reqPacket4.Seq+5 /* len of data */, ack, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now())

	return []*TCPPacket{
		reqPacket1, reqPacket2, reqPacket3, reqPacket4, respPacket,
	}
}

func postMessage() []*TCPPacket {
	ack := uint32(rand.Int63())
	seq2 := uint32(rand.Int63())
	seq := uint32(rand.Int63())

	c := 10000
	data := make([]byte, c)
	rand.Read(data)

	head := []byte("POST / HTTP/1.1\r\nContent-Length: 9958\r\n\r\n")
	for i := range head {
		data[i] = head[i]
	}

	return []*TCPPacket{
		buildPacket(true, ack, seq, data, time.Now()),
		buildPacket(false, seq+uint32(len(data)), seq2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now()),
	}
}

func getMessage() []*TCPPacket {
	ack := uint32(rand.Int63())
	seq2 := uint32(rand.Int63())
	seq := uint32(rand.Int63())

	return []*TCPPacket{
		buildPacket(true, ack, seq, []byte("GET / HTTP/1.1\r\n\r\n"), time.Now()),
		buildPacket(false, seq+18, seq2, []byte("HTTP/1.1 200 OK\r\n\r\n"), time.Now()),
	}
}

// Response comes before Request
func TestRawListenerBench(t *testing.T) {
	l := NewListener("", "0", EnginePcap, true, 200*time.Millisecond, "")
	defer l.Close()

	// Should re-construct message from all possible combinations
	for i := 0; i < 1000; i++ {
		go func(i int) {
			for j := 0; j < 100; j++ {
				var packets []*TCPPacket

				if j%5 == 0 {
					packets = chunkedPostMessage()
				} else if j%3 == 0 {
					packets = postMessage()
				} else {
					packets = getMessage()
				}

				for _, p := range packets {
					// Randomly drop packets
					if (i+j)%5 == 0 {
						if rand.Int63()%3 == 0 {
							continue
						}
					}

					l.packetsChan <- p.dump()
					time.Sleep(time.Millisecond)
				}

				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	ch := l.Receiver()

	var count int32

	for {
		select {
		case <-ch:
			atomic.AddInt32(&count, 1)
		case <-time.After(2000 * time.Millisecond):
			log.Println("Emitted 200000 messages, captured: ", count, len(l.ackAliases), len(l.seqWithData), len(l.respAliases), len(l.respWithoutReq), len(l.packetsChan))
			return
		}
	}
}
