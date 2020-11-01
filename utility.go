// Copyright 2018 The SS.SYSU Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rxgo

import (
	"fmt"
	"reflect"
)

// Test Observer
type InnerObserver struct {
	name string
}

var _ Observer = InnerObserver{"test"}

func (o InnerObserver) OnNext(x interface{}) {
	fmt.Println(o.name, "Receive value ", x)
}

func (o InnerObserver) OnError(e error) {
	fmt.Println(o.name, "Error ", e)
}

func (o InnerObserver) OnCompleted() {
	fmt.Println(o.name, "Down ")
}

// func type check, such as `func(x int) bool` satisfied for `func(x anytype) bool`
func checkFuncUpcast(fv reflect.Value, inType, outType []reflect.Type, ctx_sup bool) (b, ctx_b bool) {
	//fmt.Println(fv.Kind(),reflect.Func)
	// 首先判断是否是函数
	if fv.Kind() != reflect.Func {
		return // Not func
	}

	//
	ft := fv.Type()
	if ft.NumOut() != len(outType) {
		return // Error result parameters
	}
	if !ctx_sup {
		if ft.NumIn() != len(inType) {
			return
		}
	} else {
		// 函数的参数数量与 inType 的数量不一致
		if ft.NumIn() == 0 {
			if len(inType) != 0 {
				return
			}
		} else {
			// 第一个参数实现了上下文
			if ft.In(0).Implements(typeContext) {
				ctx_b = true

				// 参数数量此时必须等于 inType 的个数+1
				if ft.NumIn() != len(inType)+1 {
					return
				}
			} else {
				if ft.NumIn() != len(inType) {
					return
				}
			}
		}
	}

	for i, t := range inType {
		var real_t reflect.Type
		if ctx_b {
			real_t = ft.In(i + 1)
		} else {
			real_t = ft.In(i)
		}


		//todo: ptr or slice check
		switch {
		case real_t == t:
		case t.Kind() == reflect.Interface && real_t.Implements(t):
		//case ft.In(i).AssignableTo(t):
		//case ft.In(i).ConvertibleTo(t):
		default:
			return
		}
	}
	for i, t := range outType {
		//fmt.Println(ft.Out(i), t)
		//todo: ptr or slice check
		switch {
		case ft.Out(i) == t:
		case t.Kind() == reflect.Interface && ft.Out(i).Implements(t):
		default:
			return
		}
	}
	b = true
	return
}

// ckeck gunction the first parameter can accept error
func checkFuncAcceptError(fv reflect.Value) (b bool) {
	if fv.Kind() != reflect.Func {
		return // Not func
	}
	ft := fv.Type()
	if ft.NumIn() == 0 {
		return
	}

	i := 0 //ptr to first para
	if ft.In(0).Implements(typeContext) {
		i++
	}
	if ft.NumIn() <= i {
		return
	}
	t := ft.In(i)
	if t.Kind() == reflect.Interface && (t.Implements(typeAny) || t.Implements(typeError)) {
		return true
	}

	return
}

// wrap exception when call user function
func userFuncCall(fv reflect.Value, params []reflect.Value) (res []reflect.Value, skip, stop bool, eout error) {
	defer func() {
		if e := recover(); e != nil {
			if fe, ok := e.(FlowableError); ok {
				eout = fe
				return
			}
			switch e {
			case ErrSkipItem:
				skip = true
				return
			case ErrEoFlow:
				stop = true
				return
			default:
				panic(e)
			}
		}
	}()

	res = fv.Call(params)
	return
}

func isInBuf(buf []interface{}, val reflect.Value) bool {

	for _, value := range buf {
		value = reflect.ValueOf(value).Interface()
		if value == val.Interface() {
			return true
		}
	}
	return false
}

// partion the input stream by giving condition
// isTake ---- true, indicates that it is take mode
// isTake ---- false,  indicates that it is skip mode
func partionFlow(isTake bool, division int, in []interface{}) ([]interface{}, error) {
	// get the first few
	fmt.Println(in)
	if (isTake && division > 0) || (!isTake && division < 0) {
		if !isTake {
			division = len(in) + division
		}
		if division >= len(in) || division <= 0{
			return nil, OutOfBounds
		}
		return in[:division], nil
	}

	// get the first few
	if (isTake && division < 0) || (!isTake && division > 0) {
		if isTake {
			division = len(in) + division
		}
		if division >= len(in) || division <= 0{
			return nil, OutOfBounds
		}
		return in[division:], nil
	}
	return nil, OutOfBounds
}