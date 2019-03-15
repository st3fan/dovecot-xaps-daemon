package logger

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func init() {
	// optional use the json formatter
	log.SetFormatter(&log.TextFormatter{})

	log.SetOutput(os.Stdout)

	// by default use the warning loglevel
	log.SetLevel(log.WarnLevel)
}

func ParseLoglevel(argument string) {
	switch argument {
	case "debug":
		log.Warn("Logging set to debug")
		log.SetLevel(log.DebugLevel)
	case "error":
		log.Warn("Logging set to error")
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.Warn("Logging set to fatal")
		log.SetLevel(log.FatalLevel)
	case "info":
		log.Warn("Logging set to info")
		log.SetLevel(log.InfoLevel)
	case "panic":
		log.Warn("Logging set to panic")
		log.SetLevel(log.PanicLevel)
	case "warn":
		log.Warn("Logging set to warn")
		log.SetLevel(log.WarnLevel)
	default:
		log.Warn("The provided LogLevel is not of type debug, error, fatal, info, warn or panic - setting to warn instead")
		// we do not need to set the loglevel here since warn is set in init()
	}
}
