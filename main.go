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

// TODO make it possible to "go generate" on Windows (https://github.com/josephspurrier/goversioninfo/issues/52).
//go:generate /bin/bash -c "go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest -arm -64 -icon=assets/icon.ico - <<< '{}'"

package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/gonutz/w32/v2"

	"github.com/ahmetb/RectangleWin/w32ex"
)

var lastResized w32.HWND

func main() {
	runtime.LockOSThread() // since we bind hotkeys etc that need to dispatch their message here
	if !w32ex.SetProcessDPIAware() {
		panic("failed to set DPI aware")
	}

	autorun, err := AutoRunEnabled()
	if err != nil {
		panic(err)
	}
	fmt.Printf("autorun enabled=%v\n", autorun)
	printMonitors()

	edgeFuncs := [][]resizeFunc{
		{leftHalf, leftThreeQuarters, leftOneQuarter},
		{rightHalf, rightThreeQuarters, rightOneQuarter},
		{topHalf, topThreeQuarters, topOneQuarter},
		{bottomHalf, bottomThreeQuarters, bottomOneQuarter}}
	edgeFuncTurn := make([]int, len(edgeFuncs))
	cornerFuncs := [][]resizeFunc{
		{topLeftHalf, topLeftThreeQuarters, topLeftOneQuarter},
		{topRightHalf, topRightThreeQuarters, topRightOneQuarter},
		{bottomLeftHalf, bottomLeftThreeQuarters, bottomLeftOneQuarter},
		{bottomRightHalf, bottomRightThreeQuarters, bottomRightOneQuarter}}
	cornerFuncTurn := make([]int, len(cornerFuncs))
	quarterStepFuncs := [][]resizeFunc{
		{leftQuarterStep0, leftQuarterStep1, leftQuarterStep2, leftQuarterStep3},
		{rightQuarterStep0, rightQuarterStep1, rightQuarterStep2, rightQuarterStep3},
		{topQuarterStep0, topQuarterStep1, topQuarterStep2, topQuarterStep3},
		{bottomQuarterStep0, bottomQuarterStep1, bottomQuarterStep2, bottomQuarterStep3}}
	quarterStepTurn := make([]int, len(quarterStepFuncs))

	indexToDir := []direction{dirLeft, dirRight, dirUp, dirDown}

	var lastCycleGroup *[]int
	var lastCycleIndex int = -1

	resetAllCycles := func() {
		edgeFuncTurn = make([]int, len(edgeFuncs))
		cornerFuncTurn = make([]int, len(cornerFuncs))
		quarterStepTurn = make([]int, len(quarterStepFuncs))
		lastCycleGroup = nil
		lastCycleIndex = -1
	}

	cycleFuncs := func(funcs [][]resizeFunc, turns *[]int, i int, dir direction) {
		hwnd := w32.GetForegroundWindow()
		if hwnd == 0 {
			panic("foreground window is NULL")
		}
		if lastResized != hwnd || turns != lastCycleGroup || i != lastCycleIndex {
			resetAllCycles()
		}
		lastCycleGroup = turns
		lastCycleIndex = i
		if _, err := resize(hwnd, funcs[i][(*turns)[i]%len(funcs[i])], dir); err != nil {
			fmt.Printf("warn: resize: %v\n", err)
			return
		}
		(*turns)[i]++
	}

	cycleEdgeFuncs := func(i int) { cycleFuncs(edgeFuncs, &edgeFuncTurn, i, indexToDir[i]) }
	cycleCornerFuncs := func(i int) { cycleFuncs(cornerFuncs, &cornerFuncTurn, i, indexToDir[i]) }
	cycleQuarterStepFuncs := func(i int) { cycleFuncs(quarterStepFuncs, &quarterStepTurn, i, indexToDir[i]) }

	hks := []HotKey{
		(HotKey{id: 1, mod: MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_LEFT, callback: func() { cycleEdgeFuncs(0) }}),
		(HotKey{id: 2, mod: MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_RIGHT, callback: func() { cycleEdgeFuncs(1) }}),
		(HotKey{id: 3, mod: MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_UP, callback: func() { cycleEdgeFuncs(2) }}),
		(HotKey{id: 4, mod: MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_DOWN, callback: func() { cycleEdgeFuncs(3) }}),
		// Corner func #1
		(HotKey{id: 5, mod: MOD_CONTROL | MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_LEFT, callback: func() { cycleCornerFuncs(0) }}),
		(HotKey{id: 6, mod: MOD_CONTROL | MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_UP, callback: func() { cycleCornerFuncs(1) }}),
		(HotKey{id: 7, mod: MOD_CONTROL | MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_DOWN, callback: func() { cycleCornerFuncs(2) }}),
		(HotKey{id: 8, mod: MOD_CONTROL | MOD_ALT | MOD_WIN | MOD_NOREPEAT, vk: w32.VK_RIGHT, callback: func() { cycleCornerFuncs(3) }}),
		(HotKey{id: 50, mod: MOD_SHIFT | MOD_WIN, vk: 0x46 /*F*/, callback: func() {
			resetAllCycles()
			if err := maximize(); err != nil {
				fmt.Printf("warn: maximize: %v\n", err)
				return
			}
		}}),
		(HotKey{id: 60, mod: MOD_ALT | MOD_WIN, vk: 0x43 /*C*/, callback: func() {
			resetAllCycles()
			if _, err := resize(w32.GetForegroundWindow(), center, dirNone); err != nil {
				fmt.Printf("warn: resize: %v\n", err)
				return
			}
		}}),
		(HotKey{id: 70, mod: MOD_ALT | MOD_WIN, vk: 0x41 /*A*/, callback: func() {
			resetAllCycles()
			hwnd := w32.GetForegroundWindow()
			if err := toggleAlwaysOnTop(hwnd); err != nil {
				fmt.Printf("warn: toggleAlwaysOnTop: %v\n", err)
				return
			}
			fmt.Printf("> toggled always on top: %v\n", hwnd)
		}}),
	}

	myConfig := fetchConfiguration()
	// start from id 200
	id := 200
	for _, keyBinding := range myConfig.Keybindings {
		switch keyBinding.BindFeature {
		case "moveToTop":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleEdgeFuncs(2) }}))
		case "moveToBottom":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleEdgeFuncs(3) }}))
		case "moveToLeft":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleEdgeFuncs(0) }}))
		case "moveToRight":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleEdgeFuncs(1) }}))
		case "moveToTopLeft":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleCornerFuncs(0) }}))
		case "moveToTopRight":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleCornerFuncs(1) }}))
		case "moveToBottomLeft":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleCornerFuncs(2) }}))
		case "moveToBottomRight":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleCornerFuncs(3) }}))
		case "makeLarger":
			id += 1
			hks = append(hks, (HotKey{
				id:  id,
				mod: int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:  int(keyBinding.KeyCode),
				callback: func() {
					resetAllCycles()
					if _, err := resize(w32.GetForegroundWindow(), makeLarger, dirNone); err != nil {
						fmt.Printf("warn: resize: %v\n", err)
						return
					}
				}}))
		case "makeSmaller":
			id += 1
			hks = append(hks, (HotKey{
				id:  id,
				mod: int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:  int(keyBinding.KeyCode),
				callback: func() {
					resetAllCycles()
					if _, err := resize(w32.GetForegroundWindow(), makeSmaller, dirNone); err != nil {
						fmt.Printf("warn: resize: %v\n", err)
						return
					}
				}}))
		case "makeFullHeight":
			id += 1
			hks = append(hks, (HotKey{
				id:  id,
				mod: int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:  int(keyBinding.KeyCode),
				callback: func() {
					resetAllCycles()
					if _, err := resize(w32.GetForegroundWindow(), maxHeight, dirNone); err != nil {
						fmt.Printf("warn: resize: %v\n", err)
						return
					}
				}}))
		case "leftOneQuarter":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleQuarterStepFuncs(0) }}))
		case "rightOneQuarter":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleQuarterStepFuncs(1) }}))
		case "topOneQuarter":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleQuarterStepFuncs(2) }}))
		case "bottomOneQuarter":
			id += 1
			hks = append(hks, (HotKey{
				id:       id,
				mod:      int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:       int(keyBinding.KeyCode),
				callback: func() { cycleQuarterStepFuncs(3) }}))
		case "leftHalf", "leftTwoThirds", "leftOneThirds", "leftThreeQuarters",
			"rightHalf", "rightTwoThirds", "rightOneThirds", "rightThreeQuarters",
			"topHalf", "topTwoThirds", "topOneThirds", "topThreeQuarters",
			"bottomHalf", "bottomTwoThirds", "bottomOneThirds", "bottomThreeQuarters":
			directFuncs := map[string]resizeFunc{
				"leftHalf": leftHalf, "leftTwoThirds": leftTwoThirds, "leftOneThirds": leftOneThirds,
				"leftThreeQuarters": leftThreeQuarters,
				"rightHalf":         rightHalf, "rightTwoThirds": rightTwoThirds, "rightOneThirds": rightOneThirds,
				"rightThreeQuarters": rightThreeQuarters,
				"topHalf":            topHalf, "topTwoThirds": topTwoThirds, "topOneThirds": topOneThirds,
				"topThreeQuarters": topThreeQuarters,
				"bottomHalf":       bottomHalf, "bottomTwoThirds": bottomTwoThirds, "bottomOneThirds": bottomOneThirds,
				"bottomThreeQuarters": bottomThreeQuarters,
			}
			fn := directFuncs[keyBinding.BindFeature]
			directDir := dirNone
			switch {
			case len(keyBinding.BindFeature) >= 4 && keyBinding.BindFeature[:4] == "left":
				directDir = dirLeft
			case len(keyBinding.BindFeature) >= 5 && keyBinding.BindFeature[:5] == "right":
				directDir = dirRight
			case len(keyBinding.BindFeature) >= 3 && keyBinding.BindFeature[:3] == "top":
				directDir = dirUp
			case len(keyBinding.BindFeature) >= 6 && keyBinding.BindFeature[:6] == "bottom":
				directDir = dirDown
			}
			dd := directDir
			id += 1
			hks = append(hks, (HotKey{
				id:  id,
				mod: int(keyBinding.CombinedMod) | MOD_NOREPEAT,
				vk:  int(keyBinding.KeyCode),
				callback: func() {
					resetAllCycles()
					if _, err := resize(w32.GetForegroundWindow(), fn, dd); err != nil {
						fmt.Printf("warn: resize: %v\n", err)
						return
					}
				}}))
		default:
			continue
		}
	}

	var failedHotKeys []HotKey
	for _, hk := range hks {
		if !RegisterHotKey(hk) {
			failedHotKeys = append(failedHotKeys, hk)
		}
	}
	if len(failedHotKeys) > 0 {
		fmt.Printf("RegisterHotKey failed for %d hotkey(s), falling back to low-level keyboard hook:\n", len(failedHotKeys))
		for _, hk := range failedHotKeys {
			fmt.Printf("  - %s\n", hk.Describe())
		}
		installLLHook(failedHotKeys)
	}

	exitCh := make(chan os.Signal)
	signal.Notify(exitCh, os.Interrupt)
	go func() {
		<-exitCh
		fmt.Println("exit signal received")
		systray.Quit() // causes WM_CLOSE, WM_QUIT, not sure if a side-effect
	}()

	// TODO systray/systray.go already locks the OS thread in init()
	// however it's not clear if GetMessage(0,0) will continue to work
	// as we run "go initTray()" and not pin the thread that initializes the
	// tray.
	initTray()
	if err := msgLoop(); err != nil {
		panic(err)
	}
}

func showMessageBox(text string) {
	w32.MessageBox(w32.GetActiveWindow(), text, "RectangleWin", w32.MB_ICONWARNING|w32.MB_OK)
}

type resizeFunc func(disp, cur w32.RECT) w32.RECT

func center(disp, cur w32.RECT) w32.RECT {
	// TODO find a way to round up divisions consistently as it causes multiple runs to shift by 1px
	w := (disp.Width() - cur.Width()) / 2
	h := (disp.Height() - cur.Height()) / 2
	return w32.RECT{
		Left:   disp.Left + w,
		Right:  disp.Left + w + cur.Width(),
		Top:    disp.Top + h,
		Bottom: disp.Top + h + cur.Height()}
}

func resize(hwnd w32.HWND, f resizeFunc, dir direction) (bool, error) {
	if !isZonableWindow(hwnd) {
		fmt.Printf("warn: non-zonable window: %s\n", w32.GetWindowText(hwnd))
		return false, nil
	}
	rect := w32.GetWindowRect(hwnd)
	mon := w32.MonitorFromWindow(hwnd, w32.MONITOR_DEFAULTTONEAREST)
	hdc := w32.GetDC(hwnd)
	displayDPI := w32.GetDeviceCaps(hdc, w32.LOGPIXELSY)
	if !w32.ReleaseDC(hwnd, hdc) {
		return false, fmt.Errorf("failed to ReleaseDC:%d", w32.GetLastError())
	}
	var monInfo w32.MONITORINFO
	if !w32.GetMonitorInfo(mon, &monInfo) {
		return false, fmt.Errorf("failed to GetMonitorInfo:%d", w32.GetLastError())
	}

	ok, frame := w32.DwmGetWindowAttributeEXTENDED_FRAME_BOUNDS(hwnd)
	if !ok {
		return false, fmt.Errorf("failed to DwmGetWindowAttributeEXTENDED_FRAME_BOUNDS:%d", w32.GetLastError())
	}
	windowDPI := w32ex.GetDpiForWindow(hwnd)
	resizedFrame := resizeForDpi(frame, int32(windowDPI), int32(displayDPI))

	fmt.Printf("> window: 0x%x %#v (w:%d,h:%d) mon=0x%X(@ display DPI:%d)\n", hwnd, rect, rect.Width(), rect.Height(), mon, displayDPI)
	fmt.Printf("> DWM frame:        %#v (W:%d,H:%d) @ window DPI=%v\n", frame, frame.Width(), frame.Height(), windowDPI)
	fmt.Printf("> DPI-less frame:   %#v (W:%d,H:%d)\n", resizedFrame, resizedFrame.Width(), resizedFrame.Height())

	// calculate how many extra pixels go to win10 invisible borders
	lExtra := resizedFrame.Left - rect.Left
	rExtra := -resizedFrame.Right + rect.Right
	tExtra := resizedFrame.Top - rect.Top
	bExtra := -resizedFrame.Bottom + rect.Bottom

	newPos := f(monInfo.RcWork, resizedFrame)

	// adjust offsets based on invisible borders
	newPos.Left -= lExtra
	newPos.Top -= tExtra
	newPos.Right += rExtra
	newPos.Bottom += bExtra

	lastResized = hwnd
	if sameRect(rect, &newPos) {
		if dir == dirUp {
			// Up arrow when already at position: maximize the window
			fmt.Println("> maximizing window")
			if !w32.ShowWindow(hwnd, w32.SW_MAXIMIZE) {
				return false, fmt.Errorf("failed to ShowWindow(SW_MAXIMIZE):%d", w32.GetLastError())
			}
			return true, nil
		}
		if dir != dirLeft && dir != dirRight {
			fmt.Println("no resize")
			return false, nil
		}
		// Check if the snap covers >=50% of the monitor area
		monArea := int64(monInfo.RcWork.Width()) * int64(monInfo.RcWork.Height())
		snapArea := int64(newPos.Width()) * int64(newPos.Height())
		if snapArea*2 < monArea {
			fmt.Println("no resize (snap <50% area, skipping monitor move)")
			return false, nil
		}
		nextMon := getNextMonitor(mon)
		if nextMon == 0 {
			fmt.Println("no resize (no next monitor)")
			return false, nil
		}
		var nextMonInfo w32.MONITORINFO
		if !w32.GetMonitorInfo(nextMon, &nextMonInfo) {
			return false, fmt.Errorf("failed to GetMonitorInfo for next monitor:%d", w32.GetLastError())
		}
		fmt.Printf("> moving to next monitor 0x%X\n", nextMon)
		newPos = f(nextMonInfo.RcWork, resizedFrame)
		newPos.Left -= lExtra
		newPos.Top -= tExtra
		newPos.Right += rExtra
		newPos.Bottom += bExtra
	}

	fmt.Printf("> resizing to: %#v (W:%d,H:%d)\n", newPos, newPos.Width(), newPos.Height())
	if !w32.ShowWindow(hwnd, w32.SW_SHOWNORMAL) { // normalize window first if it's set to SW_SHOWMAXIMIZE (and therefore stays maximized)
		return false, fmt.Errorf("failed to normalize window ShowWindow:%d", w32.GetLastError())
	}
	if !w32.SetWindowPos(hwnd, 0, int(newPos.Left), int(newPos.Top), int(newPos.Width()), int(newPos.Height()), w32.SWP_NOZORDER|w32.SWP_NOACTIVATE) {
		return false, fmt.Errorf("failed to SetWindowPos:%d", w32.GetLastError())
	}
	rect = w32.GetWindowRect(hwnd)
	fmt.Printf("> post-resize: %#v(W:%d,H:%d)\n", rect, rect.Width(), rect.Height())
	return true, nil
}

func maximize() error {
	hwnd := w32.GetForegroundWindow()
	if !isZonableWindow(hwnd) {
		return errors.New("foreground window is not zonable")
	}
	if !w32.ShowWindow(hwnd, w32.SW_MAXIMIZE) {
		return fmt.Errorf("failed to ShowWindow:%d", w32.GetLastError())
	}
	return nil
}

func toggleAlwaysOnTop(hwnd w32.HWND) error {
	if !isZonableWindow(hwnd) {
		return errors.New("foreground window is not zonable")
	}

	if w32.GetWindowLong(hwnd, w32.GWL_EXSTYLE)&w32.WS_EX_TOPMOST != 0 {
		if !w32.SetWindowPos(hwnd, w32.HWND_NOTOPMOST, 0, 0, 0, 0, w32.SWP_NOMOVE|w32.SWP_NOSIZE) {
			return fmt.Errorf("failed to SetWindowPos(HWND_NOTOPMOST): %v", w32.GetLastError())
		}
	} else {
		if !w32.SetWindowPos(hwnd, w32.HWND_TOPMOST, 0, 0, 0, 0, w32.SWP_NOMOVE|w32.SWP_NOSIZE) {
			return fmt.Errorf("failed to SetWindowPos(HWND_TOPMOST) :%v", w32.GetLastError())
		}
	}
	return nil
}

func resizeForDpi(src w32.RECT, from, to int32) w32.RECT {
	return w32.RECT{
		Left:   src.Left * to / from,
		Right:  src.Right * to / from,
		Top:    src.Top * to / from,
		Bottom: src.Bottom * to / from,
	}
}

func sameRect(a, b *w32.RECT) bool {
	return a != nil && b != nil && reflect.DeepEqual(*a, *b)
}
