package pn532

import "log"

type Logger interface {
	Infof(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}

var DefaultLogger = &defaultLogger{}

type defaultLogger struct{}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *defaultLogger) Debugf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

type SilentLogger struct{}

func (l *SilentLogger) Infof(_ string, _ ...interface{}) {}

func (l *SilentLogger) Errorf(_ string, _ ...interface{}) {}

func (l *SilentLogger) Debugf(_ string, _ ...interface{}) {}
