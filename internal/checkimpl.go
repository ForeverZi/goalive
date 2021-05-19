package internal

import (
	"container/list"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrMonoChecker  = errors.New("monochecker error")
	ErrDuplicateRun = fmt.Errorf("[%w]:run仅允许调用一次", ErrMonoChecker)
	ErrTouchBlock   = fmt.Errorf("[%w]:touch管道阻塞了", ErrMonoChecker)
)

type Entry struct {
	K string
	T time.Time
}

type MonoChecker struct {
	stopChan     chan struct{}
	touchChan    chan Entry
	downLineChan chan Entry
	upLineChan   chan Entry
	list         *list.List
	m            map[string]*list.Element
	running      int32
	timeout      time.Duration
	batchSize    int
}

func NewMonoChecker(bufferCount int, timeout time.Duration, batchSize int) *MonoChecker {
	return &MonoChecker{
		stopChan:     make(chan struct{}),
		touchChan:    make(chan Entry, bufferCount),
		downLineChan: make(chan Entry, bufferCount),
		upLineChan:   make(chan Entry, bufferCount),
		list:         list.New(),
		m:            make(map[string]*list.Element),
		timeout:      timeout,
		batchSize:    batchSize,
	}
}

func (checker *MonoChecker) Touch(identity string) (err error) {
	select {
	case checker.touchChan <- Entry{K: identity, T: time.Now()}:
	default:
		err = ErrTouchBlock
	}
	return
}

func (checker *MonoChecker) UpChan() (upChan <-chan Entry) {
	upChan = checker.upLineChan
	return
}

func (checker *MonoChecker) DownChan() (downChan <-chan Entry) {
	downChan = checker.downLineChan
	return
}

func (checker *MonoChecker) Run() (err error) {
	if !atomic.CompareAndSwapInt32(&checker.running, 0, 1) {
		err = ErrDuplicateRun
		return
	}
	defer func() {
		// 不可以直接崩溃
		if err := recover(); err != nil {
			log.Printf("monochecker:%v\n", err)
		}
		atomic.CompareAndSwapInt32(&checker.running, 1, 0)
	}()
	var timerPool = sync.Pool{
		New: func() interface{} {
			return time.NewTimer(time.Second)
		},
	}
loop:
	for {
		select {
		case <-checker.stopChan:
			break loop
		default:
		}
		if checker.list.Len() < 1 {
			select {
			case entry := <-checker.touchChan:
				if elem, ok := checker.m[entry.K]; ok {
					checker.list.Remove(elem)
				} else {
					checker.upLineChan <- entry
				}
				checker.m[entry.K] = checker.list.PushBack(entry)
			case <-time.After(checker.timeout):
			}
		} else {
			now := time.Now()
			batchCount := 0
			for front := checker.list.Front(); front != nil && batchCount <= checker.batchSize; {
				fv := front.Value.(Entry)
				if now.Sub(fv.T) < checker.timeout {
					to := timerPool.Get().(*time.Timer)
					to.Stop()
					to.Reset(now.Sub(fv.T))
					select {
					case entry := <-checker.touchChan:
						if !to.Stop() {
							<-to.C
						}
						if elem, ok := checker.m[entry.K]; ok {
							checker.list.Remove(elem)
						} else {
							checker.upLineChan <- entry
						}
						checker.m[entry.K] = checker.list.PushBack(entry)
					case <-to.C:
					}
					timerPool.Put(to)
					break
				}
				// 移除最前面的
				checker.list.Remove(front)
				delete(checker.m, fv.K)
				// 这里不可以阻塞
				select {
				case checker.downLineChan <- fv:
				default:
				}
				front = checker.list.Front()
				batchCount++
			}
		}
	}
	return
}
