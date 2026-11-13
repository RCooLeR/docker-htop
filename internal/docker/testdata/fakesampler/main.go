// Command fakesampler stands in for docker-htop-sampler in the integration test. "stream" prints
// two sample lines and exits when stdin reaches EOF; "signal" refuses with exit code 3.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fakesampler stream|signal")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "stream":
		fmt.Println(`{"time_unix_ms":1,"cpus":[{"total_ms":10,"idle_ms":5}],"tasks":1,"threads":1}`)
		fmt.Println(`{"time_unix_ms":2,"cpus":[{"total_ms":20,"idle_ms":9}],"tasks":2,"threads":2}`)
		_, _ = io.Copy(io.Discard, os.Stdin)
		fmt.Fprintln(os.Stderr, "stdin closed")
	case "signal":
		fmt.Fprintln(os.Stderr, "fake signal helper refuses", os.Args[2:])
		os.Exit(3)
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(2)
	}
}
