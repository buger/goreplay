//go:build gofuzz

package http_proto

func Fuzz(data []byte) int {

	ParseHeaders(data)

	return 1
}
