package pikago

import (
	"sync"
	"time"
)

type CondTimed struct {
	sync.Cond
}

// WaitRelTimeout 是一个等待超时广播.
func (cv *CondTimed) WaitRelTimeout(when time.Duration) bool {
	timer := time.AfterFunc(when, func() {
		cv.L.Lock()
		cv.Broadcast()
		cv.L.Unlock()
	})
	cv.Wait()
	return timer.Stop()
}

// WaitAbsTimeout绝对时间等待，如果在现在之后，减去现在时间后调用实际超时时间
func (cv *CondTimed) WaitAbsTimeout(when time.Time) bool {
	now := time.Now()
	if when.After(now) {
		return cv.WaitRelTimeout(when.Sub(now))
	}
	return cv.WaitRelTimeout(time.Duration(0))
}

//Waiter 等待完成，和sync.WaitGroup很像，但是包含超时
type Waiter struct {
	cv  CondTimed
	cnt int
	sync.Mutex
}

// 初始化
func (w *Waiter) Init() {
	w.cv.L = w
	w.cnt = 0
}

//Add 在调用goroutine前调用
func (w *Waiter) Add() {
	w.Lock()
	w.cnt++
	w.Unlock()
}

//Done 对应Add，当完成一项工作后调用Done
//当count小于0发出panic，当count等于0广播通知等待的goroutine
func (w *Waiter) Done() {
	w.Lock()
	w.cnt--
	if w.cnt < 0 {
		panic("wait count dropped < 0")
	}
	if w.cnt == 0 {
		w.cv.Broadcast()
	}
	w.Unlock()
}

//Wait 等待超时，当count等于0不等待
func (w *Waiter) Wait() {
	w.Lock()
	for w.cnt != 0 {
		w.cv.Wait()
	}
	w.Unlock()
}

//WaitRelTimeout 等待count为0，否则超时，当count为0返回ture
func (w *Waiter) WaitRelTimeout(d time.Duration) bool {
	w.Lock()
	for w.cnt != 0 {
		if !w.cv.WaitRelTimeout(d) {
			break
		}
	}
	done := w.cnt == 0
	w.Unlock()
	return done
}

//WaitAbsTimeout 像是WaitRelTimeout，但是调用的是绝对时间点
func (w *Waiter) WaitAbsTimeout(t time.Time) bool {
	w.Lock()
	for w.cnt != 0 {
		if !w.cv.WaitAbsTimeout(t) {
			break
		}
	}
	done := w.cnt == 0
	w.Unlock()
	return done
}
