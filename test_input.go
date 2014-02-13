package gor

type TestInput struct {
	data chan []byte
}

func NewTestInput() (i *TestInput) {
	i = new(TestInput)
	i.data = make(chan []byte, 100)

	return
}

func (i *TestInput) Read() ([]byte, bool) {
	buf, ok := <-i.data

	return buf, ok
}

func (i *TestInput) EmitGET() {
	i.data <- []byte("GET / HTTP/1.1\r\n\r\n")
}

func (i *TestInput) EmitPOST() {
	i.data <- []byte("POST /pub/WWW/ HTTP/1.1\nHost: www.w3.org\r\n\r\na=1&b=2\r\n\r\n")
}

func (i *TestInput) EmitOPTIONS() {
	i.data <- []byte("OPTIONS / HTTP/1.1\nHost: www.w3.org\r\n\r\n")
}

func (i *TestInput) String() string {
	return "Test Input"
}
