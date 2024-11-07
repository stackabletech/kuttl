package test

import (
	"fmt"
	"strconv"

	"github.com/spf13/pflag"
	"sigs.k8s.io/kind/pkg/log"

	testutils "github.com/stackabletech/kuttl/pkg/test/utils"
)

type level int32

var verbosity level

func SetFlags(flags *pflag.FlagSet) {
	flags.VarP(&verbosity, "v", "v", "Logging verbosity level. 0=normal, 1=verbose, 2=detailed, 3+=trace.")
}

func (l *level) Get() interface{} {
	return *l
}

func (l *level) String() string {
	return strconv.FormatInt(int64(*l), 10)
}

func (l *level) Set(value string) error {
	v, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*l = level(v)
	return nil
}

func (l *level) Type() string {
	return string(*l)
}

// kindLogger lets KIND log to the kuttl logger.
// KIND log level N corresponds to kuttl log level N+1, such that
// using the default 0 kuttl log level produces no KIND output.
type kindLogger struct {
	l testutils.Logger
}

func (k kindLogger) V(level log.Level) log.InfoLogger {
	if int(level) >= int(verbosity) {
		return &nopLogger{}
	}
	return k
}

func (k kindLogger) Warn(message string) {
	// TODO (@NickLarsenNZ): Replace Logger.Log with a method for the correct level (eg: Warn)
	k.l.Log(message)
}

func (k kindLogger) Warnf(format string, args ...interface{}) {
	// TODO (@NickLarsenNZ): Replace Logger.Log with a method for the correct level (eg: Warn)
	k.l.Log(fmt.Sprintf(format, args...))
}

func (k kindLogger) Error(message string) {
	// TODO (@NickLarsenNZ): Replace Logger.Log with a method for the correct level (eg: Error)
	k.l.Log(message)
}

func (k kindLogger) Errorf(format string, args ...interface{}) {
	// TODO (@NickLarsenNZ): Replace Logger.Log with a method for the correct level (eg: Error)
	k.l.Log(fmt.Sprintf(format, args...))
}

func (k kindLogger) Info(message string) {
	// TODO (@NickLarsenNZ): Replace Logger.Log with a method for the correct level (eg: Info)
	k.l.Log(message)
}

func (k kindLogger) Infof(format string, args ...interface{}) {
	// TODO (@NickLarsenNZ): Replace Logger.Log with a method for the correct level (eg: Info)
	k.l.Log(fmt.Sprintf(format, args...))
}

func (k kindLogger) Enabled() bool {
	return true
}

type nopLogger struct{}

func (n *nopLogger) Enabled() bool {
	return false
}

func (n *nopLogger) Info(_ string) {}

func (n *nopLogger) Infof(_ string, _ ...interface{}) {}
