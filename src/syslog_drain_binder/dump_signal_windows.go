package main

import "os"

// Windows does not have a signal equivalent to SIGUSR1.
func registerGoRoutineDumpSignalChannel() chan os.Signal {
	return make(chan os.Signal)
}
