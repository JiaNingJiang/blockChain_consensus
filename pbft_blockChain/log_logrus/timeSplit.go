package loglogrus

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// 自定义的io.Writer对象，负责对日志文件进行分割
type logFileWriter struct {
	file    *os.File // 真正的日志文件的io.Writer
	logPath string   // 日志文件路径
	logFile string   // 日志文件路径+文件名

	logKind string // 普通日志 or 错误日志

	fileDate string //判断日期切换目录
}

// 自定义write方法
func (p *logFileWriter) Write(data []byte) (n int, err error) {
	if p == nil {
		return 0, errors.New("logFileWriter is nil")
	}
	if p.file == nil {
		return 0, errors.New("file not opened")
	}

	//判断日期是否变更，如果变更需要重新生成新的日志文件，达到分割效果
	fileDate := time.Now().Format("2006_01_02_15_04") // 按分钟进行分割
	if p.fileDate != fileDate {
		p.file.Close() //关闭旧的日志文件

		p.logFile = p.logPath + fmt.Sprintf("%s%s", p.logKind, time.Now().Format("2006_01_02_15_04"))

		err = os.MkdirAll(fmt.Sprintf("%s", p.logPath), os.ModePerm)
		if err != nil {
			return 0, err
		}
		filename := fmt.Sprintf("%s", p.logFile)

		p.file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660) //创建新的日志文件
		if err != nil {
			return 0, err
		}
	}

	n, e := p.file.Write(data) //将日志写入日志文件
	return n, e

}
