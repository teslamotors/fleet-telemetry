package metrics

import (
	"os"
	"runtime/trace"
	"sync"
)

var (
	profilerMutex sync.Mutex
	profilerOn    bool

	// ProfilerFile file to write profile info out to
	ProfilerFile *os.File
)

// EnableProfiler used to enable the profiler
func EnableProfiler(mode string) (changed bool) {
	profilerMutex.Lock()
	defer profilerMutex.Unlock()

	newProfileMode := mode == "on"
	if newProfileMode == profilerOn {
		return false
	}

	if mode == "off" {
		trace.Stop()
	} else {
		_ = trace.Start(ProfilerFile)
	}

	profilerOn = newProfileMode
	return true
}
