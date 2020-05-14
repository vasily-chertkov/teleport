/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// stateOK means Teleport is operating normally.
	stateOK = iota
	// stateRecovering means Teleport has begun recovering from a degraded state.
	stateRecovering
	// stateDegraded means some kind of connection error has occurred to put
	// Teleport into a degraded state.
	stateDegraded
	// stateStarting means the process is starting but hasn't joined the
	// cluster yet.
	stateStarting
)

var stateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: teleport.MetricState,
	Help: "State of the teleport process: 0 - ok, 1 - recovering, 2 - degraded, 3 - starting",
})

func init() {
	prometheus.MustRegister(stateGauge)
	stateGauge.Set(stateStarting)
}

// processState tracks the state of the Teleport process.
type processState struct {
	process      *TeleportProcess
	recoveryTime time.Time
	currentState int64
}

// newProcessState returns a new FSM that tracks the state of the Teleport process.
func newProcessState(process *TeleportProcess) *processState {
	return &processState{
		process:      process,
		recoveryTime: process.Clock.Now(),
		currentState: stateStarting,
	}
}

// Process updates the state of Teleport.
func (f *processState) Process(event Event) {
	switch event.Name {
	// Ready event means Teleport has started successfully.
	case TeleportReadyEvent:
		atomic.StoreInt64(&f.currentState, stateOK)
		stateGauge.Set(stateOK)
		f.process.Infof("Detected that service started and joined the cluster successfully.")
	// If a degraded event was received, always change the state to degraded.
	case TeleportDegradedEvent:
		atomic.StoreInt64(&f.currentState, stateDegraded)
		stateGauge.Set(stateDegraded)
		f.process.Infof("Detected Teleport is running in a degraded state.")
	// If the current state is degraded, and a OK event has been
	// received, change the state to recovering. If the current state is
	// recovering and a OK events is received, if it's been longer
	// than the recovery time (2 time the server keep alive ttl), change
	// state to OK.
	case TeleportOKEvent:
		switch atomic.LoadInt64(&f.currentState) {
		case stateDegraded:
			atomic.StoreInt64(&f.currentState, stateRecovering)
			stateGauge.Set(stateRecovering)
			f.recoveryTime = f.process.Clock.Now()
			f.process.Infof("Teleport is recovering from a degraded state.")
		case stateRecovering:
			if f.process.Clock.Now().Sub(f.recoveryTime) > defaults.ServerKeepAliveTTL*2 {
				atomic.StoreInt64(&f.currentState, stateOK)
				stateGauge.Set(stateOK)
				f.process.Infof("Teleport has recovered from a degraded state.")
			}
		}
	}
}

// GetState returns the current state of the system.
func (f *processState) GetState() int64 {
	return atomic.LoadInt64(&f.currentState)
}
