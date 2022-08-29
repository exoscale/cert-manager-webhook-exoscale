package main

import "log"

// Logger is a thin wrapper around stdlib.log with just 2 levels (info & debug).
type Logger struct {
	Verbose bool
}

// Info simply runs log.Println with info tag.
func (l Logger) Info(args ...any) {
	log.Println(append([]any{"[INFO]"}, args...)...)
}

// Info simply runs log.Printf with info tag.
func (l Logger) Infof(format string, args ...any) {
	log.Printf("[INFO] "+format, args...)
}

// Debug runs log.Println with debug tag if Verbose is true.
func (l Logger) Debug(args ...any) {
	if l.Verbose {
		log.Println(append([]any{"[DEBUG]"}, args...)...)
	}
}

// Debug runs log.Printf with debug tag if Verbose is true.
func (l Logger) Debugf(format string, args ...any) {
	if l.Verbose {
		log.Printf("[DEBUG] "+format, args...)
	}
}
