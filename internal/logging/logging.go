package logging

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Logger is a thread-safe logger whose properties persist and can be modified.
type Logger struct {
	logger *logrus.Entry
	ctx    string
	fields map[string]interface{}
	mtx    *sync.Mutex
}

// NewLogger will instantiate a logger with the given context.
func NewLogger(ctx string, kvpairs ...interface{}) *Logger {
	return &Logger{
		logger: logrus.WithField("ctx", ctx),
		ctx:    ctx,
		fields: serializeKVPairs(kvpairs),
		mtx:    &sync.Mutex{},
	}
}

func (l *Logger) withFields() *logrus.Entry {
	if len(l.fields) > 0 {
		return l.logger.WithFields(l.fields)
	}
	return l.logger
}

func serializeKVPairs(kvpairs ...interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	if (len(kvpairs) % 2) == 0 {
		for i := 0; i < len(kvpairs); i += 2 {
			res[kvpairs[i].(string)] = kvpairs[i+1]
		}
	}
	return res
}

func (l *Logger) withKVPairs(kvpairs ...interface{}) *logrus.Entry {
	fields := serializeKVPairs(kvpairs...)
	if len(fields) > 0 {
		return l.withFields().WithFields(fields)
	}
	return l.withFields()
}

func (l *Logger) Debug(msg string, kvpairs ...interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.withKVPairs(kvpairs...).Debugln(msg)
}

func (l *Logger) Info(msg string, kvpairs ...interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.withKVPairs(kvpairs...).Infoln(msg)
}

func (l *Logger) Error(msg string, kvpairs ...interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.withKVPairs(kvpairs...).Errorln(msg)
}

func (l *Logger) SetField(key string, val interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.fields[key] = val
}
