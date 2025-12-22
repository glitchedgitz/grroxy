package logflags

import "log"

// init ensures all standard library log output includes timestamps with
// microsecond precision. Import this package with a blank identifier (`_`)
// in binaries to apply the setting globally.
func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

