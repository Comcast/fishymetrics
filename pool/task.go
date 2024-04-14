/*
 * Copyright 2023 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pool

import (
	"sync"

	"github.com/comcast/fishymetrics/common"
)

// Task encapsulates a work item that should go in a work pool
type Task struct {
	// Err holds an error that occurred during a task. Its
	// result is only meaningful after Run has been called
	// for the pool that holds it.
	Err error

	Body           []byte
	MetricHandlers []common.Handler

	f func() ([]byte, error)
}

// MoonshotTask encapsulates a work item that should go in a
// moonshot work pool.
type MoonshotTask struct {
	// Err holds an error that occurred during a task. Its
	// result is only meaningful after Run has been called
	// for the pool that holds it.
	Err error

	Body       []byte
	MetricType string
	Device     string

	f func() ([]byte, string, string, error)
}

// NewTask initializes a new task based on a given work
// function.
func NewTask(f func() ([]byte, error), handlers []common.Handler) *Task {
	return &Task{MetricHandlers: handlers, f: f}
}

// NewMoonshotTask initializes a new task based on a given
// HPE moonshot work function.
func NewMoonshotTask(f func() ([]byte, string, string, error)) *MoonshotTask {
	return &MoonshotTask{f: f}
}

// Run runs a Task and does appropriate accounting via a
// given sync.WorkGroup.
func (t *Task) Run(wg *sync.WaitGroup) {
	t.Body, t.Err = t.f()
	wg.Done()
}

// Run runs a Task and does appropriate accounting via a
// given sync.WorkGroup.
func (t *MoonshotTask) Run(wg *sync.WaitGroup) {
	t.Body, t.Device, t.MetricType, t.Err = t.f()
	wg.Done()
}
