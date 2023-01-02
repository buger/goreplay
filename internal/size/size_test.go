package size

import "testing"

func TestParseDataUnit(t *testing.T) {
	var d = map[string]int{
		"42mb":                 42 << 20,
		"4_2":                  42,
		"00":                   0,
		"0":                    0,
		"0_600tb":              384 << 40,
		"0600Tb":               384 << 40,
		"0o12Mb":               10 << 20,
		"0b_10010001111_1kb":   2335 << 10,
		"1024":                 1 << 10,
		"0b111":                7,
		"0x12gB":               18 << 30,
		"0x_67_7a_2f_cc_40_c6": 113774485586118,
		"121562380192901":      121562380192901,
	}
	var buf Size
	var err error
	for k, v := range d {
		err = buf.Set(k)
		if err != nil || buf != Size(v) {
			t.Errorf("Error parsing %s: %v", k, err)
		}
	}
}
