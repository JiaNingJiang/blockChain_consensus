package loglogrus

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// 自定义Hook, 负责将warnf级别以上的日志单独放到一个日志文件中
type WarnHook struct {
	Writer io.Writer
}

// 将logrus.WarnLevel级别以上的日志写入到单独的文件中
func (hook *WarnHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}
	hook.Writer.Write([]byte(line))
	return nil
}

// 范围只限logrus.WarnLevel级别以上的日志
func (hook *WarnHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.WarnLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}
