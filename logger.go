package logger

import (
	"github.com/sirupsen/logrus"
)

// NewLogger 创建新的日志对象
func NewLogger(service, env string) (*logrus.Logger, error) {
	f := NewFormatter(service, env)

	l := logrus.New()
	l.SetFormatter(f)
	return l, nil
}
