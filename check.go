package goalive

import (
	"time"

	"github.com/ForeverZi/goalive/internal"
)

type Entry = internal.Entry

type AliveChecker interface {
	Run() (err error)
	Touch(identity string) (err error)
	// UpChan 获取上线消息通道，需要尽快消耗，touch会以非阻塞的方式向此管道发送
	UpChan() (upChan <-chan Entry)
	// DownChan 获取下线消息通道，需要尽快消耗，超过缓冲区后将丢失事件
	DownChan() (toChan <-chan Entry)
}

type checkerConf struct {
	timeout    time.Duration
	chanbuffer int
	batchSize  int
}

var defaultConf = checkerConf{
	timeout:    300 * time.Second,
	chanbuffer: 1000,
	batchSize:  61,
}

type Option func(conf *checkerConf)

func TimeoutOption(duration time.Duration) Option {
	return func(conf *checkerConf) {
		conf.timeout = duration
	}
}

func NewChecker(options ...Option) AliveChecker {
	var conf = defaultConf
	for _, opt := range options {
		opt(&conf)
	}
	return internal.NewMonoChecker(conf.chanbuffer, conf.timeout, conf.batchSize)
}
