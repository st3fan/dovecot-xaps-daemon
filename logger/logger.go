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
		log.SetLevel(log.DebugLevel)
		log.Info("Logging set to debug")
	case "error":
		log.SetLevel(log.ErrorLevel)
		log.Info("Logging set to error")
	case "fatal":
		log.SetLevel(log.FatalLevel)
		log.Info("Logging set to fatal")
	case "info":
		log.SetLevel(log.InfoLevel)
		log.Info("Logging set to info")
	case "panic":
		log.SetLevel(log.PanicLevel)
		log.Info("Logging set to panic")
	default:
		log.Warn("The provided LogLevel is not of type debug, error, fatal, info or panic - setting to warn instead")
		// we do nit need to set the loglevel here since warn is set in init()
	}
}

