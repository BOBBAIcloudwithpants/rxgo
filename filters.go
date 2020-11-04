package rxgo

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"
)

var (
	NoInputStream = errors.New("There are no input stream.")
	OutOfBounds = errors.New("The number of expected flow is beyond the length of the input flow")
	ElementOutOfBounds = errors.New("The element you are looking for is out of the input bound")
	)


type filterOperator struct{
	opFunc func(ctx context.Context, o *Observable, item reflect.Value, out chan interface{}) (end bool)
}



func (tsop filterOperator) op(ctx context.Context, o *Observable) {

	in := o.pred.outflow
	out := o.outflow

	// set the time interval
	interval := o.debounce_timespan


	//fmt.Println(o.name, "operator in/out chan ", in, out)
	var wg sync.WaitGroup

	var out_buf []interface{}

	go func() {
		is_appear := make(map[interface{}]bool)
		end := false
		// acquire the starting time
		start := time.Now()
		sample_start := time.Now()
		for x := range in {
			// receiving time since last receive
			rt := time.Since(start)
			st := time.Since(sample_start)
			start = time.Now()

			if end {
				continue
			}

			// Not satisfy the sample period
			if o.sample_interval > 0 && st < o.sample_interval {
				continue
			}

			// if the receiving time is less than the debounce interval, not send it to the flow
			if interval > time.Duration(0) && rt < interval {
				continue
			}
			// can not pass a interface as parameter (pointer) to gorountion for it may change its value outside!
			xv := reflect.ValueOf(x)
			// send an error to stream if the flip not accept error
			if e, ok := x.(error); ok && !o.flip_accept_error {
				o.sendToFlow(ctx, e, out)
				continue
			}

			// buffer the input flow
			o.mu.Lock()
			out_buf = append(out_buf, x)
			o.mu.Unlock()

			// if using ElementAt operator, skip the loop
			if o.element_at > 0 {
				continue
			}

			// if using Take, Takelast, Skip, SkipLast operator, directly skip the loop
			if o.take != 0 || o.skip != 0 {
				continue
			}
			// if using the 'Last' operator, save the input flow val in out_buf and continuing receiving
			if o.only_last {
				continue
			}
			//_, ok := is_appear[xv]
			// if using the 'Distinct' operator, firstly judge whether the xv has already appeared previously, if so, skip to next loop
			if o.only_distinct && is_appear[xv.Interface()] {
				continue
			}
			// memorizing the appearance of the element
			o.mu.Lock()
			is_appear[xv.Interface()] = true
			o.mu.Unlock()

			if interval > time.Duration(0){
				// save the last item, if it exists
				if len(out_buf) > 2 {
					xv = reflect.ValueOf(out_buf[len(out_buf) - 2])
				}
			}

			// scheduler
			switch threading := o.threading; threading {
			case ThreadingDefault:
				if o.sample_interval > 0 {
					sample_start = sample_start.Add(o.sample_interval)
				}

				if tsop.opFunc(ctx, o, xv, out) {
					end = true
				}

			case ThreadingIO:
				fallthrough
			case ThreadingComputing:
				wg.Add(1)
				if o.sample_interval > 0 {
					sample_start.Add(o.sample_interval)
				}

				go func() {
					defer wg.Done()
					if tsop.opFunc(ctx, o, xv, out) {
						end = true
					}
				}()
			default:
			}
			// if using 'First' operator, end the loop once the first input is sent to opFunc
			if o.only_first {
				break
			}
		}

		// if in Last Filter, send the last element in the outbuf to Last
		if o.only_last && len(out_buf) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				xv := reflect.ValueOf(out_buf[len(out_buf)-1])
				tsop.opFunc(ctx, o, xv, out)
			}()
		}

		// Take, TakeLast, Skip, SkipLast operator
		if o.take != 0 || o.skip != 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var div int
				if o.is_taking {
					div = o.take
				} else {
					div = o.skip
				}
				new_in,err := partionFlow(o.is_taking, div, out_buf)

				if err != nil {
					o.sendToFlow(ctx, err, out)
				} else {
					xv := new_in
					for _, val := range xv {
						tsop.opFunc(ctx, o, reflect.ValueOf(val), out)
					}
				}
			}()
		}

		// ElementAt operator
		if o.element_at != 0 {
			if o.element_at < 0 || o.element_at > len(out_buf) {
				o.sendToFlow(ctx, ElementOutOfBounds, out)
			} else {
				xv := reflect.ValueOf(out_buf[o.element_at-1])
				tsop.opFunc(ctx, o, xv, out)
			}
		}
		wg.Wait() //waiting all go-routines completed

		if (o.only_last || o.only_first) && len(out_buf) == 0 && !o.flip_accept_error  {
			o.sendToFlow(ctx, NoInputStream, out)
		}


		o.closeFlow(out)
	}()


}

// initialize a new FilterObservable
func (parent *Observable) newFilterObservable(name string) (o *Observable) {
	o = newObservable()

	o.Name = name

	//chain Observables
	parent.next = o
	o.pred = parent
	o.root = parent.root

	//set options
	o.buf_len = BufferLen
	return
}

// First emit only the first item (or the first item that meets some condition) emitted by an Observable
func (parent *Observable) First() (o *Observable) {
	o = parent.newFilterObservable("first")
	o.only_first, o.only_last, o.only_distinct = true,false, false
	o.debounce_timespan = 0
	o.take, o.skip = 0, 0
	o.operator = firstOperator
	return o

}

var firstOperator = filterOperator{func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {
	//fv := reflect.ValueOf(o.flip)
	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// Last emit only the last item (or the last item that meets some condition) emitted by an Observable
func (parent *Observable) Last() (o *Observable) {
	o = parent.newFilterObservable("last")
	o.operator = lastOperator

	o.only_first, o.only_last, o.only_distinct = false, true, false
	o.debounce_timespan = 0
	o.take, o.skip = 0, 0

	return o
}

var lastOperator = filterOperator{func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {
	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// Debounce only emit an item from an Observable if a particular timespan has passed without it emitting another item
func (parent *Observable) Debounce(t time.Duration) (o *Observable) {
	o = parent.newFilterObservable("debounce")
	o.only_first = false
	o.only_last = false
	o.only_distinct = false
	o.debounce_timespan = t
	o.take, o.skip = 0, 0
	o.operator = lastOperator
	return o
}


var debounceOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {
	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// Distinct suppress duplicate items emitted by an Observable
func (parent *Observable) Distinct() (o *Observable) {
	o = parent.newFilterObservable("distinct")
	o.only_first = false
	o.only_last = false
	o.only_distinct = true
	o.debounce_timespan = 0
	o.take, o.skip = 0, 0
	o.operator = distinctOperator
	return o
}

var distinctOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {
	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}
// Take emit only the first n items emitted by an Observable
func (parent *Observable) Take(num int) (o *Observable) {

	o = parent.newFilterObservable("Take")
	o.only_first, o.only_last, o.only_distinct = false, false, false
	o.debounce_timespan = 0
	o.skip = 0
	o.take = num
	o.is_taking = true
	o.operator = takeOperator

	return o
}

var takeOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {

	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// TakeLast emit only the last n items emitted by an Observable
func (parent *Observable) TakeLast(num int) (o *Observable) {
	o = parent.newFilterObservable("takeLast")
	o.only_first, o.only_last, o.only_distinct = false, false, false
	o.debounce_timespan = 0
	o.skip = 0
	o.take = -num
	o.is_taking = true
	o.operator = takeLastOperator
	return o
}

var takeLastOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {

	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// Skip suppress the first n items emitted by an Observable
func (parent *Observable) Skip(num int) (o *Observable) {
	o = parent.newFilterObservable("skip")
	o.only_first, o.only_last, o.only_distinct = false, false, false
	o.debounce_timespan = 0
	o.skip = num
	o.take = 0
	o.is_taking = false
	o.operator = skipOperator

	return o
}

var skipOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {

	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// SkipLast suppress the last n items emitted by an Observable
func (parent *Observable) SkipLast(num int) (o *Observable) {
	o = parent.newFilterObservable("skipLast")
	o.only_first, o.only_last, o.only_distinct = false, false, false
	o.debounce_timespan = 0
	o.skip = -num
	o.take = 0
	o.is_taking = false
	o.operator = skipLastOperator

	return o
}

var skipLastOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {

	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// ElementAt emit only item n emitted by an Observable
func (parent *Observable) ElementAt(num int) (o *Observable) {

	o = parent.newFilterObservable("elementAt")
	o.only_first, o.only_last, o.only_distinct = false, false, false
	o.debounce_timespan = 0
	o.skip = 0
	o.take = 0
	o.is_taking = false
	o.element_at = num
	o.operator = elementAtOperator

	return
}

var elementAtOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {

	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

// Sample emit the most recent item emitted by an Observable within periodic time intervals.
func (parent *Observable) Sample(st time.Duration) (o *Observable) {
	o = parent.newFilterObservable("sample")
	o.only_first, o.only_last, o.only_distinct = false, false, false
	o.debounce_timespan = 0
	o.sample_interval = st
	o.skip = 0
	o.take = 0
	o.is_taking = false
	o.element_at = 0
	o.operator = sampleOperator

	return o
}

var sampleOperator = filterOperator{opFunc: func(ctx context.Context, o *Observable, x reflect.Value, out chan interface{}) (end bool) {

	var params = []reflect.Value{x}
	x = params[0]
	// send data
	if !end {
		end = o.sendToFlow(ctx, x.Interface(), out)
	}
	return
},
}

