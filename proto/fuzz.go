// +build gofuzz

package proto

func Fuzz(data []byte) int {

	ParseHeaders(data)

	return 1
}
