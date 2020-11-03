# rxgo 改进文档

## 个人信息

|      |          |
| ---- | -------- |
| 姓名 | 白家栋   |
| 学号 | 18342001 |
| 专业 | 软件工程 |

## 完成情况简述

本次的任务是在潘老师的 `rxgo` 包基础上改进原有代码或者增加新的操作，我大致做了以下的工作:    
- 在 `Observable` 这个struct的基础上增加了一些属性
- 完成了 `ReactiveX` 官方文档中 [Filtering Observables](http://reactivex.io/documentation/operators.html) 下的全部方法，包括:
  - First()
  - Last()
  - Debounce(time Time.Duration)
  - Distinct()
  - ElementAt(idx int)
  - Sample(time Time.Duration)
  - Take(num int)
  - TakeLast(num int)
  - Skip(num int)
  - SkipLast(num int)

## 目录结构介绍

```
.
├── LICENSE
├── README.md
├── filters.go              // 新增文件，包含了 Filtering Observable 下的方法
├── filters_test.go         // 新增文件，包含了 Filtering Observable 的测试
├── generators.go
├── generators_test.go
├── go.mod
├── go.sum
├── rxgo.go                 // 改动文件，主要对 Observable 数据结构进行了一些增加
├── rxgo_test.go
├── specification.md
├── transform_test.go
├── transforms.go
└── utility.go              // 改动文件，增加了
```

## 设计思路
阅读 `rxgo` 包的源代码可以总结出以下的几个设计的理念与准则:    
1. 每个 Observable 方法(比如 `Map`, `Just` 等等)在 RxGO 中都返回了一个相应的 `type Observable struct` 的实例。
2. 每当一个 Observable 方法被调用时，首先要做的事情是：将每个 `Observable` 实例之间相互连接。这种连接类似于**链表**的连接，每个 Obsevable 都有一个指针 `pred`, 指向它的父亲 Observable; 还有一个指针 `next` 指向下一个 Observable。
3. `type Observable struct` 有一个非常重要的成员函数，`connect`. 在上面的第2个准则中，我们了解到 Observable 之间是怎样连接的，connect 函数就利用了这种连接负责建立并且开始每个 Observable 之间的 **数据流连接**。这里可以看一下已经实现好的 `connect` 方法代码:

```go
// connect all Observable form the first one.
func (o *Observable) connect(ctx context.Context) {
	for po := o.root; po != nil; po = po.next {
		po.outflow = make(chan interface{}, po.buf_len)
		//fmt.Println("conneted", po.Name, po.outflow)
		po.operator.op(ctx, po)
	}
}
```
可以看到，connect 会从根节点开始，沿着 Observable 连接向下，依次 **创建输入输出channel**，并且调用每个 Observable 包含的 `operator` 的 `op` 方法，也就是不同Observable的功能逻辑的实现主体

4. ReactiveX 在官方文档中提到了`懒加载`这个概念，也即当且仅当 **Subscribe** 方法被调用时开始创建数据流，计算得到最终的结果，所以 connect 方法仅会在 `Subscribe` 被调用后才调用

了解了这些设计原则之后，下面的介绍首先从新增的 Observable 属性开始：

### 新增属性

```go
// rxgo.go 文件下
type Observable struct {
  ...
  ...
// indicate whether pick only the first input flow from parent, commonly set to true in 'First' Observer
	only_first		  bool

	// indicate whether pick only the first input flow from parent, commonly set to true in 'Last' Observer
	only_last		  bool

	// indicate after how many seconds without emitting can a value be admitted, commonly used in 'Debounce' Observer
	debounce_timespan time.Duration

	// indicate whether this Observable only emits distinct value
	only_distinct	  bool

	// indicate the sample interval of input flow, commonly used in 'Sample' Filter
	sample_interval   time.Duration

	// indicate the element index in elementAt, commonly used in 'ElementAt' Filter
	element_at		  int

	// indicate first/last (n) item to be skipped, commonly used in 'Skip' Filter
	// if skip is positive, then it means skipping first few items; if skip is negative, then it means skipping last few items
	skip			  int

	// indicate first/last (n) item to be taken, commonly used in 'Skip' Filter
	// if take is positive, then it means taking first few items; if take is negative, then it means taking last few items
	take			  int

	// indicate whether the Operator is 'Take' or 'TakeLast'
	is_taking		  bool
} 
```

可以看到，这些属性的作用主要是在辅助 Filtering Operator 的实现。当在不同的 Filtering Operator 中创建 Observable 对象的时候，会根据不同的 Operator 设置不同的属性。下面介绍每个 Operator 的实现：

### Filtering Operator 实现

#### 1. First

- 定义: First emit only the first item (or the first item that meets some condition) emitted by an Observable. 

- 示意图(根据我的实现)

![](https://tva1.sinaimg.cn/large/0081Kckwgy1gkb8qvz1dpj315k0feabs.jpg)


- 函数定义
```go
// First emit only the first item (or the first item that meets some condition) emitted by an Observable
func (parent *Observable) First() (o *Observable) 
```

- Operator 属性设置   
  
| name    | only_first | only_last | debounce_timespan | only_distinct | sample_interval | element_at | skip | take | is_taking |
| ------- | ---------- | --------- | ----------------- | ------------- | --------------- | ---------- | ---- | ---- | --------- |
| "first" | true       | false     | 0                 | 0             | 0               | 0          | 0    | 0    | false     |


- 实现细节    
  由于 First 的功能是仅将上一层的输入流的第一个元素发送到输出流，所以仅需要在 `filterOperator` 的 op 函数中在接受第一个输入后结束阻塞即可。同时在实现中我添加了一个变量 `out_buf`, 用于缓存输入流的元素，当在所有发送进程结束之后，也即:
```go
func (tsop filterOperator) op(ctx context.Context, o *Observable) {
  ...
  ...
  go func() {
    ...
    ...
    // if using 'First' operator, end the loop once the first input is sent to opFunc
			if o.only_first {
				break
			}
  }
  ...
  // error handling
  if (o.only_last || o.only_first) && len(out_buf) == 0 && !o.flip_accept_error  {
			o.sendToFlow(ctx, NoInputStream, out)
		}
}
```
- 对应测试    
```go
func TestFirst(t *testing.T) {
	res := []int{}
	ob := rxgo.Just(10, 20, 30).Map(func(x int) int {
		return 2 * x
	}).First()
	ob.Subscribe(func(x int) {
		res = append(res, x)
	})
	assert.Equal(t, []int{20}, res, "First Test Error!")
}
```


#### 2. Last

- 定义: Last emit only the last item (or the last item that meets some condition) emitted by an Observable. 

- 示意图(根据我的实现)
![](https://tva1.sinaimg.cn/large/0081Kckwgy1gkb9cgs89sj315e0g0q4s.jpg)


- 函数定义
```go
// Last emit only the last item (or the last item that meets some condition) emitted by an Observable
func (parent *Observable) Last() (o *Observable)
```

- Operator 属性设置   
  
| name   | only_first | only_last | debounce_timespan | only_distinct | sample_interval | element_at | skip | take | is_taking |
| ------ | ---------- | --------- | ----------------- | ------------- | --------------- | ---------- | ---- | ---- | --------- |
| "last" | false      | true      | 0                 | 0             | 0               | 0          | 0    | 0    | false     |


- 实现细节    
利用刚刚提到的 `out_buf`, 每次上一个流发送新的元素的时候就存放在 `out_buf` 中，等发送全部结束，取出最后一个元素返回即可。也即:

```go
func (tsop filterOperator) op(ctx context.Context, o *Observable) {
  ...
  ...
  go func() {
    ...
    ...
    for x := range in {
      // buffer the input flow
			o.mu.Lock()
			out_buf = append(out_buf, x)
			o.mu.Unlock()

    }
  }
  ...
  // if in Last Filter, send the last element in the outbuf to Last
		if o.only_last && len(out_buf) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				xv := reflect.ValueOf(out_buf[len(out_buf)-1])
				tsop.opFunc(ctx, o, xv, out)
			}()
		}

  // error handling
  if (o.only_last || o.only_first) && len(out_buf) == 0 && !o.flip_accept_error  {
			o.sendToFlow(ctx, NoInputStream, out)
		}
}
```     
- 测试设计     
```go
func TestLast(t *testing.T) {
	res := []int{}
	ob := rxgo.Just(10, 20, 30).Map(func(x int) int {
		return 2 * x
	}).Last()
	ob.Subscribe(func(x int) {
		res = append(res, x)
	})

	assert.Equal(t, []int{60}, res, "Last Test Error!")
}
```


#### 3. Debounce
- 定义: Debounce only emit an item from an Observable if a particular timespan has passed without it emitting another item.

- 示意图      
![](https://tva1.sinaimg.cn/large/0081Kckwgy1gkcd6ccandj31660g8jto.jpg)


- 函数定义
```go
// Debounce only emit an item from an Observable if a particular timespan has passed without it emitting another item
func (parent *Observable) Debounce(t time.Duration)
```


- Operator 属性设置   
  
| name       | only_first | only_last | debounce_timespan | only_distinct | sample_interval | element_at | skip | take | is_taking |
| ---------- | ---------- | --------- | ----------------- | ------------- | --------------- | ---------- | ---- | ---- | --------- |
| "debounce" | false      | false     | t                 | 0             | 0               | 0          | 0    | 0    | false     |

- 实现细节     
debounce 要实现的功能是对于给定的 **t** 时间长度，当且仅当接受一个流传送过来的物体的 t 时间内，没有再接收到任何物体，才将这个物体保存。因此每接受一个对象，就要计算距离上一次接受对象的时间差：
  - 如果时间差**小于 t**，则放弃接受，同时重启计时器
  - 如果时间差**大于 t**，则接受上一个对象，同时刷新计时器

对应的代码如下:

```go
for x := range in {
  ...
// if the receiving time is less than the debounce interval, not send it to the flow
			if interval > time.Duration(0) && rt < interval {
				continue
			}
  ...
  ...
  if interval > time.Duration(0){
				// save the last item, if it exists
				if len(out_buf) > 2 {
					xv = reflect.ValueOf(out_buf[len(out_buf) - 2])
				}
			}
}

```

- 测试设计
```go
func TestDebounce(t *testing.T) {
	res := []int{}
	ob := rxgo.Just(10, 20, 30, 40).Map(func(x int) int {
		time.Sleep(20 * time.Millisecond)
		return 2 * x
	}).Debounce(30 * time.Millisecond)
	ob.Subscribe(func(x int) {
		res = append(res, x)
	})

	assert.Equal(t, []int{}, res, "Debounce Test Error!")
}
```

**4. Distinct**

- 定义    
Distinct suppress duplicate items emitted by an Observable

- 示意图
![](https://tva1.sinaimg.cn/large/0081Kckwgy1gkcenxvsg0j31800g440i.jpg)

- 函数定义
```go
// Distinct suppress duplicate items emitted by an Observable
func (parent *Observable) Distinct()
```

- Operator 属性设置   
  
| name       | only_first | only_last | debounce_timespan | only_distinct | sample_interval | element_at | skip | take | is_taking |
| ---------- | ---------- | --------- | ----------------- | ------------- | --------------- | ---------- | ---- | ---- | --------- |
| "distinct" | false      | false     | 0                 | true          | 0               | 0          | 0    | 0    | false     |


- 实现细节    
这里，我使用到了 `map[interface{}]bool` 类型的数据结构。每次上一个流发送来一个数据，都首先判断 `map[obj]` 是否为 true, 为 true 说明之前出现过，根据 distinct 的定义不将这个物体传送到输出流；为 false 说明之前没有出现过，那么首先设置 `map[obj] = true`, 然后发送到输出流。    
```go
...
is_appear := make(map[interface{}]bool)


		for x := range in {
      // if using the 'Distinct' operator, firstly judge whether the xv has already appeared previously, if so, skip to next loop
			if o.only_distinct && is_appear[xv.Interface()] {
				continue
      }
      ...

      // memorizing the appearance of the element
			o.mu.Lock()
			is_appear[xv.Interface()] = true
			o.mu.Unlock()    
    }
```

- 测试设计    
```go
func TestDistinct(t *testing.T) {
	res := []int{}
	ob := rxgo.Just(10, 20, 30, 40, 20, 10, 50).Map(func(x int) int {
		return 2 * x
	}).Distinct()
	ob.Subscribe(func(x int) {
		res = append(res, x)
	})
	assert.Equal(t, []int{20, 40, 60, 80, 100}, res, "Distinct Test Error!")
}
```

### **5. Take**    

- 定义      
Take emit only the first n items emitted by an Observable

- 示意图

![](https://tva1.sinaimg.cn/large/0081Kckwgy1gkcfek9epoj315a0g8wgd.jpg)

- 函数定义    
```go
// Take emit only the first n items emitted by an Observable
func (parent *Observable) Take(num int) (o *Observable)
```
- Operator 属性设置   
  
| name       | only_first | only_last | debounce_timespan | only_distinct | sample_interval | element_at | skip | take | is_taking |
| ---------- | ---------- | --------- | ----------------- | ------------- | --------------- | ---------- | ---- | ---- | --------- |
| "distinct" | false      | false     | 0                 | false         | 0               | 0          | 0    | num  | true      |

- 函数实现    

Take 的作用是根据输入的元素个数从头开始选取给定个数的对象传送到输出流。在实现的过程中为了方便我使用了阻塞的方法，即从 `out_buf` 中根据需求选取对象并且放入输出流。根据不同模式(take, skip, takeLast, skip)来选取元素的函数实现如下:    
```go
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
```

- 测试设计     
```go
func TestTake(t *testing.T) {
	res := []int{}
	ob := rxgo.Just(10, 20, 30, 40, 20, 10, 50).Map(func(x int) int {
		return 2 * x
	}).Take(4)
	ob.Subscribe(func(x int) {
		res = append(res, x)
	})
	assert.Equal(t, []int{20, 40, 60, 80}, res, "Take Test Error!")
}
```


### 6. TakeLast

- 定义    
TakeLast emit only the last n items emitted by an Observable

- 示意图     
![](https://tva1.sinaimg.cn/large/0081Kckwgy1gkcg4361daj31580fmq4s.jpg)

- 函数定义     
```go
// TakeLast emit only the last n items emitted by an Observable
func (parent *Observable) TakeLast(num int) (o *Observable)
```

- Operator 属性设置   
  
| name       | only_first | only_last | debounce_timespan | only_distinct | sample_interval | element_at | skip | take | is_taking |
| ---------- | ---------- | --------- | ----------------- | ------------- | --------------- | ---------- | ---- | ---- | --------- |
| "takeLast" | false      | false     | 0                 | false         | 0               | 0          | 0    | num  | false     |




