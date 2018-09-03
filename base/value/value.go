// Copyright 2018 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package value provides atomic values with update notifications.
package value // import "barista.run/base/value"

import (
	"sync"
	"sync/atomic"

	l "barista.run/logging"
)

// Value provides atomic value storage with update notifications.
type Value struct {
	value atomic.Value
	// Observers that will be notified on the next value.
	obs   []chan struct{}
	obsMu sync.Mutex
}

// Next returns a channel that will be closed on the next update.
// Useful in a select, or as <-Next() to wait for value changes.
func (v *Value) Next() <-chan struct{} {
	ch := make(chan struct{})
	v.obsMu.Lock()
	defer v.obsMu.Unlock()
	v.obs = append(v.obs, ch)
	return ch
}

// Get returns the currently stored value.
func (v *Value) Get() interface{} {
	return v.value.Load()
}

// Set updates the stored values and notifies any subscribers.
func (v *Value) Set(value interface{}) {
	v.value.Store(value)
	l.Fine("%s: Store %#v", l.ID(v), value)
	v.obsMu.Lock()
	defer v.obsMu.Unlock()
	for _, o := range v.obs {
		close(o)
	}
	v.obs = nil
}

type valueOrErr struct {
	value interface{}
	err   error
}

// ErrorValue adds an error to Value, allowing storage of either
// a value (interface{}) or an error.
type ErrorValue struct {
	v       Value // of valueOrErr
	logInit sync.Once
}

func (e *ErrorValue) initLogging() {
	e.logInit.Do(func() { l.Attach(e, &e.v, "") })
}

// Next returns a channel that will be closed on the next update,
// value or error.
func (e *ErrorValue) Next() <-chan struct{} {
	e.initLogging()
	return e.v.Next()
}

// Get returns the currently stored value or error.
func (e *ErrorValue) Get() (interface{}, error) {
	e.initLogging()
	if v, ok := e.v.Get().(valueOrErr); ok {
		return v.value, v.err
	}
	// Uninitialised.
	return nil, nil
}

// Set updates the stored value and clears any error.
func (e *ErrorValue) Set(value interface{}) {
	e.initLogging()
	e.v.Set(valueOrErr{value: value})
}

// Error replaces the stored value and returns true if non-nil,
// and simply returns false if nil.
func (e *ErrorValue) Error(err error) bool {
	if err == nil {
		return false
	}
	e.initLogging()
	e.v.Set(valueOrErr{err: err})
	return true
}
