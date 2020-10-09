package goalive

import (
	"fmt"
	"math/rand"
	"time"
)

func Example() {
	checker := NewChecker(TimeoutOption(10 * time.Millisecond))
	go checker.Run()
	go func() {
		upChan := checker.UpChan()
		downChan := checker.DownChan()
		for {
			select {
			case entry := <-upChan:
				fmt.Printf("[%v]上线了", entry.K)
			case entry := <-downChan:
				fmt.Printf("[%v]下线了\n", entry.K)
			}
		}
	}()
	for i := 0; i < 10000; i++ {
		go func(i int) {
			for k := 0; k < 1000; k++ {
				checker.Touch(fmt.Sprint(i))
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(20)))
			}
		}(i)
	}
	time.Sleep(100 * time.Second)
}
