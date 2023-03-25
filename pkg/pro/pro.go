package pro

// PRO this value indicates if goreplay is running in PRO mode..
// it must not be modified explicitly in production
var PRO = false

// Enable enables PRO mode. Can be used ony in tests.
func Enable() {
	PRO = true
}
