package middleware

//func TestTokenMiddleware(t *testing.T) {
//	quit := make(chan struct{})
//	in := test.NewTestInput()
//	in.skipHeader = true
//	cmd, cancl := initCmd(tokenModifier, withDebug)
//	midd := initMiddleware(cmd, cancl, in, func(err error) {})
//	req := []byte("1 932079936fa4306fc308d67588178d17d823647c 1439818823587396305 200\nGET /token HTTP/1.1\r\nHost: example.org\r\n\r\n")
//	res := []byte("2 932079936fa4306fc308d67588178d17d823647c 1439818823587396305 200\nHTTP/1.1 200 OK\r\nContent-Length: 10\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n17d823647c")
//	rep := []byte("3 932079936fa4306fc308d67588178d17d823647c 1439818823587396305 200\nHTTP/1.1 200 OK\r\nContent-Length: 15\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n932079936fa4306")
//	count := uint32(0)
//	out := test.NewTestOutput(func(msg *Message) {
//		if msg.Meta[0] == '1' && !bytes.Equal(payloadID(msg.Meta), payloadID(req)) {
//			token, _, _ := proto.PathParam(msg.Data, []byte("token"))
//			if !bytes.Equal(token, proto.Body(rep)) {
//				t.Errorf("expected the token %s to be equal to the replayed response's token %s", token, proto.Body(rep))
//			}
//		}
//		atomic.AddUint32(&count, 1)
//		if atomic.LoadUint32(&count) == 2 {
//			quit <- struct{}{}
//		}
//	})
//	pl := &InOutPlugins{}
//	pl.Inputs = []PluginReader{midd, in}
//	pl.Outputs = []PluginWriter{out}
//	pl.All = []interface{}{midd, out, in}
//	e := NewEmitter()
//	go e.Start(pl, "")
//	in.EmitBytes(req) // emit original request
//	in.EmitBytes(res) // emit its response
//	in.EmitBytes(rep) // emit replayed response
//	// emit the request which should have modified token
//	token := []byte("1 8e091765ae902fef8a2b7d9dd96 14398188235873 100\nGET /?token=17d823647c HTTP/1.1\r\nHost: example.org\r\n\r\n")
//	in.EmitBytes(token)
//	<-quit
//	midd.Close()
//}

//func TestMiddlewareWithPrettify(t *testing.T) {
//	Settings.PrettifyHTTP = true
//	quit := make(chan struct{})
//	in := test.NewTestInput()
//	cmd, cancl := initCmd(echoSh, withDebug)
//	midd := initMiddleware(cmd, cancl, in, func(err error) {})
//	var b1 = []byte("POST / HTTP/1.1\r\nHost: example.org\r\nTransfer-Encoding: chunked\r\n\r\n4\r\nWiki\r\n5\r\npedia\r\nE\r\n in\r\n\r\nchunks.\r\n0\r\n\r\n")
//	var b2 = []byte("POST / HTTP/1.1\r\nHost: example.org\r\nContent-Length: 25\r\n\r\nWikipedia in\r\n\r\nchunks.")
//	out := test.NewTestOutput(func(msg *Message) {
//		if !bytes.Equal(proto.Body(b2), proto.Body(msg.Data)) {
//			t.Errorf("expected %q body to equal %q body", b2, msg.Data)
//		}
//		quit <- struct{}{}
//	})
//	pl := &InOutPlugins{}
//	pl.Inputs = []PluginReader{midd, in}
//	pl.Outputs = []PluginWriter{out}
//	pl.All = []interface{}{midd, out, in}
//	e := NewEmitter()
//	go e.Start(pl, "")
//	in.EmitBytes(b1)
//	<-quit
//	midd.Close()
//	Settings.PrettifyHTTP = false
//}
