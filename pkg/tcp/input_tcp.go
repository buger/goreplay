package tcp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net"

	"github.com/buger/goreplay/pkg/plugin"
	"github.com/buger/goreplay/pkg/proto"

	"github.com/rs/zerolog/log"
)

// TCPInput used for internal communication
type TCPInput struct {
	data     chan *plugin.Message
	listener net.Listener
	address  string
	config   *TCPInputConfig
	stop     chan bool // Channel used only to indicate goroutine should shutdown
}

// TCPInputConfig represents configuration of a TCP input plugin
type TCPInputConfig struct {
	Secure          bool   `json:"input-tcp-secure"`
	CertificatePath string `json:"input-tcp-certificate"`
	KeyPath         string `json:"input-tcp-certificate-key"`
}

// NewTCPInput constructor for TCPInput, accepts address with port
func NewTCPInput(address string, config *TCPInputConfig) (i *TCPInput) {
	i = new(TCPInput)
	i.data = make(chan *plugin.Message, 1000)
	i.address = address
	i.config = config
	i.stop = make(chan bool)

	i.listen(address)

	return
}

// PluginRead returns data and details read from plugin
func (i *TCPInput) PluginRead() (msg *plugin.Message, err error) {
	select {
	case <-i.stop:
		return nil, plugin.ErrorStopped
	case msg = <-i.data:
		return msg, nil
	}

}

// Close closes the plugin
func (i *TCPInput) Close() error {
	close(i.stop)
	i.listener.Close()
	return nil
}

func (i *TCPInput) listen(address string) {
	if i.config.Secure {
		cer, err := tls.LoadX509KeyPair(i.config.CertificatePath, i.config.KeyPath)
		if err != nil {
			log.Fatal().Err(err).Msg("error while loading --input-tcp TLS certificate")
		}

		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		listener, err := tls.Listen("tcp", address, config)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start INPUT-TCP listener")
		}
		i.listener = listener
	} else {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start INPUT-TCP listener")
		}
		i.listener = listener
	}
	go func() {
		for {
			conn, err := i.listener.Accept()
			if err == nil {
				go i.handleConnection(conn)
				continue
			}
			if isTemporaryNetworkError(err) {
				continue
			}
			if operr, ok := err.(*net.OpError); ok && operr.Err.Error() != "use of closed network connection" {
				log.Error().Err(err).Msg("failed to accept connection")
			}
			break
		}
	}()
}

func (i *TCPInput) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	var buffer bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if isTemporaryNetworkError(err) {
				continue
			}
			if err != io.EOF {
				log.Err(err).Msg("failed to read from connection")
			}
			break
		}

		if bytes.Equal(proto.PayloadSeparatorAsBytes[1:], line) {
			// unread the '\n' before monkeys
			buffer.UnreadByte()
			var msg plugin.Message
			msg.Meta, msg.Data = proto.PayloadMetaWithBody(buffer.Bytes())
			i.data <- &msg
			buffer.Reset()
		} else {
			buffer.Write(line)
		}
	}
}

func (i *TCPInput) String() string {
	return "TCP input: " + i.address
}

func isTemporaryNetworkError(err error) bool {
	if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
		return true
	}
	if operr, ok := err.(*net.OpError); ok && operr.Temporary() {
		return true
	}
	return false
}
