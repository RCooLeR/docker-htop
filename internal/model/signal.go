package model

// Signal is a POSIX signal by its full name, the form the Engine API accepts.
type Signal string

// The signals the plugin can send.
const (
	SignalTerm Signal = "SIGTERM"
	SignalKill Signal = "SIGKILL"
	SignalInt  Signal = "SIGINT"
	SignalHup  Signal = "SIGHUP"
	SignalQuit Signal = "SIGQUIT"
	SignalUsr1 Signal = "SIGUSR1"
	SignalUsr2 Signal = "SIGUSR2"
	SignalStop Signal = "SIGSTOP"
	SignalCont Signal = "SIGCONT"
)

// Signals returns the signals the plugin can send, in the order the UI lists them. The first
// one is the default choice.
func Signals() []Signal {
	return []Signal{
		SignalTerm, SignalKill, SignalInt, SignalHup, SignalQuit,
		SignalUsr1, SignalUsr2, SignalStop, SignalCont,
	}
}
