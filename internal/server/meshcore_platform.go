package server

import "runtime"

func meshCoreBLESupported() bool { return runtime.GOOS == "linux" }
