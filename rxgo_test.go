package rxgo_test

import (
	"fmt"
	"testing"

	"github.com/pmlpml/rxgo"
)

func TestMain(t *testing.T) {

	// test Subscribe on any
	ob := rxgo.Just(10, 20, 30).Map(dd)
	ob1 := ob.Map(dd).SubscribeOn(rxgo.ThreadingIO).Debug(true).Map(dd)
	ob1.Subscribe(func(x int) {
		fmt.Println("Just", x)
	}, nil, nil)

	ob = rxgo.Just(0, 12, 7, 34, 2).Filter(func(x int) bool {
		return x < 10
	}).SubscribeOn(rxgo.ThreadingIO)
	ob.Subscribe(
		func(x int) {
			fmt.Println("Filter", x)
		}, nil, nil)
}

func dd(x int) int { return 2 * x }

type observer struct{}

func (o observer) OnNext(x interface{}) {
	fmt.Println("observer", x)
}

func (o observer) OnError(e error) {}
func (o observer) OnCompleted()    {}

func TestObserver(t *testing.T) {
	var s rxgo.Observer = observer{}
	rxgo.Just(1, 2, 3).Subscribe(s, nil, nil)
}
