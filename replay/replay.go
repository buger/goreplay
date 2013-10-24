// Replay server receive requests objects from Listeners and forward it to given address.
// Basic usage:
//
//     gor replay -f http://staging.server
//
//
// Rate limiting
//
// It can be useful if you want forward only part of production traffic, not to overload staging environment. You can specify desired request per second using "|" operator after server address:
//
//     # staging.server not get more than 10 requests per second
//     gor replay -f "http://staging.server|10"
//
// Load testing
//
// For load testing reasons you can set limit to be higher than your actual requests count. Gor will increase load based on existing requests. It creates circullar buffer of variable length with existing requests, and use them to increase load.
//
// Forward to multiple addresses
//
// Just separate addresses by coma:
//    gor replay -f "http://staging.server|10,http://dev.server|20"
//
//
//  For more help run:
//
//     gor replay -h
//
package replay

import (
	"log"
)

// Debug enables logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.Verbose {
		log.Print("\033[33mReplay:")
		log.Print(v...)
		log.Println("\033[0m")
	}
}

// Run acts as `main` function of replay
// Replay server listen to UDP traffic from Listeners
// Each request processed by RequestFactory
func Run() {
	Settings.Parse()

	factory := NewRequestFactory()

	if Settings.FileToReplayPath != "" {
		RunReplayFromFile(factory)
	} else {
		RunReplayFromNetwork(factory)
	}
}
