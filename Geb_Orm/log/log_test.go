package log

import "testing"

func TestSetLevel(t *testing.T) {
	SetLevel(ErrorLevel)
	Error(1)
	Info(2)
	SetLevel(InfoLevel)
	Error(3)
	Info(4)
	SetLevel(Disabled)
	Error(5)
	Info(6)
}
