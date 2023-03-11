package log

import (
	"log"
	"os"
)

var (
	errorLog = log.New(os.Stdout, "\033[31m[ERROR]\033[0m", log.LstdFlags|log.Lshortfile)
	infoLog  = log.New(os.Stdout, "\033[34m[INRO ]\033[0m", log.LstdFlags|log.Lshortfile)
)

var (
	info_  = true
	error_ = true
)

func Info(any ...interface{}) {
	if !info_ {
		return
	}
	infoLog.Println(any...)
}
func Infof(format string, any ...interface{}) {
	if !info_ {
		return
	}
	infoLog.Printf(format, any...)
}
func InfoSQL(desc string, vars []interface{}) {
	if !info_ {
		return
	}
	vars = quoteString(vars)
	infoLog.Println(desc, vars)
}
func Error(any ...interface{}) {
	if !error_ {
		return
	}
	errorLog.Println(any...)
}
func Errorf(format string, any ...interface{}) {
	if !error_ {
		return
	}
	errorLog.Printf(format, any...)
}

func quoteString(vars []interface{}) []interface{} {
	temp := make([]interface{}, len(vars))
	for i, v := range vars {
		if vv, ok := v.(string); ok {
			temp[i] = "\"" + vv + "\""
		} else {
			temp[i] = v
		}
	}
	return temp
}

const (
	InfoLevel = iota
	ErrorLevel
	Disabled
)

func SetLevel(level int) {
	// default
	if level < InfoLevel || level > Disabled {
		level = ErrorLevel
	}
	if ErrorLevel < level {
		error_ = false
	}
	if InfoLevel < level {
		info_ = false
	}
}
