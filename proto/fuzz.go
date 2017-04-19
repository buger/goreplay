// +build gofuzz

package proto

func Fuzz(data []byte) int {

	ParseHeaders([][]byte{data}, func(header []byte, value []byte) bool {
		return true
	})

	return 1
}
