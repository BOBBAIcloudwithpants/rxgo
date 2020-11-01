package rxgo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRange(t *testing.T) {
	res := []int{}
	Range(0, 5).Subscribe(func(x int) {
		res = append(res, x)
	})

	assert.Equal(t, []int{0, 1, 2, 3, 4}, res, "Ragne Test Error!")
}

func TestRangeWithCancel(t *testing.T) {

	res := []int{}
	var oberver = ObserverMonitor{}
	oberver.Next = func(y interface{}) {
		x := y.(int)
		res = append(res, x)
		if x >= 3 {
			oberver.Unsubscribe()
		}
	}

	oberver.Context = func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		oberver.CancelObservables = cancel
		//fmt.Println("ctx created ", ctx)
		return ctx
	}

	Range(0, 10).Subscribe(oberver)
	assert.False(t, len(res) > 5, "Range cancel failure!")
	//fmt.Println(res)
}

func TestStart(t *testing.T) {
	// generator function func() (x anytype, end bool)
	rangex := func(start, end int64) func(ctx context.Context) (int64, bool) {
		i := start - 1
		return func(ctx context.Context) (int64, bool) {
			if i < end-1 {
				i++
				if i == 3 {
					panic(FlowableError{Err: errors.New("any"), Elements: nil})
				}
				return i, false
			}
			return 0, true
		}
	}

	res := []int64{}
	Start(rangex(1, 5)).Subscribe(
		func(x int64) {
			res = append(res, x)
		})
	//fmt.Println(res)
	assert.Equal(t, []int64{1, 2, 4}, res, "Start Test Error!")
}

func TestAnySouce(t *testing.T) {
	res := []int{}
	source := func(ctx context.Context, send func(x interface{}) (endSignal bool)) {
		send(10)
		send(20)
		send(30)
	}
	Generator(source).Subscribe(
		func(x int) {
			res = append(res, x)
		})

	assert.Equal(t, []int{10, 20, 30}, res, "Any Test Error!")
}

func TestJust(t *testing.T) {
	res := []int{}
	Just(10, 20, 30).Subscribe(
		func(x int) {
			res = append(res, x)
		})

	assert.Equal(t, []int{10, 20, 30}, res, "Just Test Error!")
}

func TestFromSlice(t *testing.T) {
	res := []int{}
	From([]int{10, 20, 30}).Subscribe(
		func(x int) {
			res = append(res, x)
		})

	assert.Equal(t, []int{10, 20, 30}, res, "FromSlice Test Error!")
}

func TestFromChan(t *testing.T) {
	ch := make(chan int)
	go func() {
		ch <- 10
		ch <- 20
		ch <- 30
		close(ch)
	}()

	res := []int{}
	From(ch).Subscribe(
		func(x int) {
			res = append(res, x)
		})

	assert.Equal(t, []int{10, 20, 30}, res, "FromChan Test Error!")
}

func TestFromObservable(t *testing.T) {
	res := []int{}
	ob := From([]int{10, 20, 30}).Map(func(x int) int {
		return x + 1
	})
	From(ob).Subscribe(
		func(x int) {
			res = append(res, x)
		})
	assert.Equal(t, []int{11, 21, 31}, res, "FromObservable Test Error!")
}

func TestThrow(t *testing.T) {
	var ee error
	Throw(ErrEoFlow).Subscribe(
		ObserverMonitor{
			Next: func(x interface{}) {
				t.Errorf("No data expected! but %v", x)
			},
			Error: func(e error) {
				ee = e
			},
			Completed: func() {
				fmt.Println("Completed")
			},
		})

	assert.Error(t, ee, "No error")
}

func TestEmpty(t *testing.T) {
	var b bool
	Empty().Subscribe(
		func(x interface{}) {
			b = true
		})
	assert.False(t, b, "Not Empty")
}

func TestNeverWithCancel(t *testing.T) {

	var oberver = ObserverMonitor{
		Next: func(x interface{}) {
			t.Errorf("Ragne Test expect %v ", "Nothng")
		},
	}

	oberver.Context = func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		oberver.CancelObservables = cancel
		//fmt.Println("ctx created ", ctx)
		return ctx
	}

	oberver.AfterConnected = func() {
		go func() {
			<-time.After(time.Nanosecond * 1000)
			//fmt.Println("time over!")
			oberver.Unsubscribe()
		}()
	}
	Never().Subscribe(oberver)
}
