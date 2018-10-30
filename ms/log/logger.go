package log

// This package contains definitions for the logger with log levels - something that is missing in the golang core packages
// The use of the packages allows to avoid importing third-party libs, like glog etc

import (
	"io"
	"io/ioutil"
	"log"
	"os"
)

// LevelLogger is a struct containing different logging levels' pointers to the log.Logger object
// Can be used as is, then the user is to take care of initialization, configuration if needed, defaults to be used otherwise
type LevelLogger struct {
	// Trace is the Lowest (includes all) logging level
	Trace *log.Logger
	// Info allows to display info messages and above but not the tracing / debugging info
	Info *log.Logger
	// Warning only shows warnings and errors
	Warning *log.Logger
	// Error level only used to print out the errors
	Error *log.Logger
}

// InitLogger allows to create/initialize the loggers with the specific Handle (stdout/err, file etc) and a custom name
// the call will convert string name of the io.Writer into the actual object and use it for logger initialization
// so the call InitLogger("ioutil.Discard", "os.Stdout", "os.Stderr", "os.Stderr") will result in the creation of four loggers
// Trace   := log.New(ioutil.Discard, "TRACE: ", log.LstdFlags|Lshortfile)
// Info    := log.New(os.Stdout, "INFO : ", log.LstdFlags|log.Lshortfile)
// Warning := log.New(os.Stderr, "WARNING : ", log.LstdFlags|log.Lshortfile)
// Error   := log.New(os.Stderr, "ERROR : ", log.LstdFlags|log.Lshortfile)
func InitLogger(traceHandle, infoHandle, warningHandle, errorHandle string) LevelLogger {
	mapping := map[string]io.Writer{
		"os.Stdout":      os.Stdout,
		"os.Stderr":      os.Stderr,
		"ioutil.Discard": ioutil.Discard,
	}
	logger := LevelLogger{
		Trace:   log.New(mapping[traceHandle], "TRACE : ", log.LstdFlags|log.Lshortfile),
		Info:    log.New(mapping[infoHandle], "INFO : ", log.LstdFlags|log.Lshortfile),
		Warning: log.New(mapping[warningHandle], "WARNING : ", log.LstdFlags|log.Lshortfile),
		Error:   log.New(mapping[errorHandle], "ERROR : ", log.LstdFlags|log.Lshortfile),
	}

	return logger
}
