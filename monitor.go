// Copyright 2022 Ahmet Alp Balkan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"sort"
	"syscall"

	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
)

func EnumMonitors(f func(d w32.HMONITOR) bool) bool {
	callback := syscall.NewCallback(func(h, _, _, _ uintptr) uintptr {
		if f(w32.HMONITOR(h)) {
			return 1
		}
		return 0
	})
	return w32.EnumDisplayMonitors(0, nil, callback, 0)
}

type direction int

const (
	dirNone  direction = -1
	dirLeft  direction = 0
	dirRight direction = 1
	dirUp    direction = 2
	dirDown  direction = 3
)

// getNextMonitor returns the next monitor in a consistent order (sorted by X then Y),
// cycling back to the first after the last. Returns 0 if only one monitor exists.
func getNextMonitor(current w32.HMONITOR) w32.HMONITOR {
	type monEntry struct {
		handle  w32.HMONITOR
		centerX int32
		centerY int32
	}

	var monitors []monEntry
	EnumMonitors(func(h w32.HMONITOR) bool {
		var info w32.MONITORINFO
		if w32.GetMonitorInfo(h, &info) {
			r := info.RcMonitor
			monitors = append(monitors, monEntry{
				handle:  h,
				centerX: (r.Left + r.Right) / 2,
				centerY: (r.Top + r.Bottom) / 2,
			})
		}
		return true
	})

	if len(monitors) <= 1 {
		return 0
	}

	sort.Slice(monitors, func(i, j int) bool {
		if monitors[i].centerX != monitors[j].centerX {
			return monitors[i].centerX < monitors[j].centerX
		}
		return monitors[i].centerY < monitors[j].centerY
	})

	for i, m := range monitors {
		if m.handle == current {
			return monitors[(i+1)%len(monitors)].handle
		}
	}
	return 0
}

func printMonitors() {
	i := 0
	EnumMonitors(func(d w32.HMONITOR) bool {
		var v w32.MONITORINFO
		if !w32.GetMonitorInfo(d, &v) {
			return false
		}
		fmt.Printf("> monitor#%d: 0x%x\n", i, d)
		i++
		fmt.Printf("       rcwork:%#v (w=%v,h=%v)\n", v.RcWork, v.RcWork.Width(), v.RcWork.Height())
		fmt.Printf("    rcmonitor:%#v (w=%v,h=%v)\n", v.RcMonitor, v.RcMonitor.Width(), v.RcWork.Height())
		fmt.Printf("      primary:%#v\n", v.DwFlags&w32.MONITORINFOF_PRIMARY > 0)

		ok, n := w32.GetNumberOfPhysicalMonitorsFromHMONITOR(d)
		if !ok {
			fmt.Printf("  physical monitors: failed to query count: %d\n", w32.GetLastError())
		} else {
			fmt.Printf("  physical monitors: %d\n", n)
			pMon := make([]w32.PHYSICAL_MONITOR, n)
			if !w32.GetPhysicalMonitorsFromHMONITOR(d, pMon) {
				fmt.Printf("  physical monitors: failed to get physical monitors: %d\n", w32.GetLastError())
			} else {
				for i, p := range pMon {
					name := windows.UTF16ToString(p.Description[:])
					fmt.Printf("  > physical monitor#%d: %s\n", i, name)
				}
			}
		}
		return true
	})
}
