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
	_ "embed"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/gonutz/w32/v2"
)

//go:embed assets/tray_icon.ico
var icon []byte

const repo = "https://github.com/ahmetb/RectangleWin"

func initTray() {
	systray.Register(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon)
	systray.SetTitle("RectangleWin")
	systray.SetTooltip("RectangleWin")

	autorun, err := AutoRunEnabled()
	if err != nil {
		panic(err)
	}

	mRepo := systray.AddMenuItem("Documentation", "")
	go func() {
		for range mRepo.ClickedCh {
			if err := w32.ShellExecute(0, "open", repo, "", "", w32.SW_SHOWNORMAL); err != nil {
				fmt.Printf("failed to launch browser: (%d), %v\n", w32.GetLastError(), err)
			}
		}
	}()

	systray.AddSeparator()

	mAutoRun := systray.AddMenuItemCheckbox("Run on startup", "", autorun)
	go func() {
		for range mAutoRun.ClickedCh {
			if mAutoRun.Checked() {
				if err := AutoRunDisable(); err != nil {
					mAutoRun.SetTitle(err.Error())
					fmt.Printf("warn: autorun disable: %v\n", err)
					continue
				}
				fmt.Println("disabled autorun")
				mAutoRun.Uncheck()
			} else {
				if err := AutoRunEnable(); err != nil {
					mAutoRun.SetTitle(err.Error())
					fmt.Printf("warn: autorun enable: %v\n", err)
					continue
				}
				fmt.Println("enabled autorun")
				mAutoRun.Check()
			}

		}
	}()

	systray.AddSeparator()

	mConfig := systray.AddMenuItem("Configuration", "")
	go func() {
		<-mConfig.ClickedCh
		fmt.Println("opening editor for default config")
		configFilePath := getValidConfigPathOrCreate()
		maybeDropExampleConfigFile(configFilePath)
		cmd := exec.Command("notepad.exe", configFilePath)
		err := cmd.Start()
		if err != nil {
			showMessageBox(fmt.Sprintf("Failed to open config file %s\n%v", configFilePath, err))
		}
		// TODO add a better way to reload current program.
		// Reloading programmatically is non-trivial because this program registers
		// hotkeys, so it much synchronize to start the child process, but quit
		// parent before the child starts to register hotkeys
	}()

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "")
	go func() {
		<-mQuit.ClickedCh
		fmt.Println("clicked Quit")
		systray.Quit()
	}()

	fmt.Println("tray ready")
	showBalloonNotification("RectangleWin", "Running in system tray")
}

func onExit() {
	fmt.Println("onExit invoked")
}

type balloonNotifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Timeout         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        [16]byte
	BalloonIcon     uintptr
}

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	user32Tray        = syscall.NewLazyDLL("user32.dll")
	pFindWindowW      = user32Tray.NewProc("FindWindowW")
)

func showBalloonNotification(title, message string) {
	className, _ := syscall.UTF16PtrFromString("SystrayClass")
	hwnd, _, _ := pFindWindowW.Call(
		uintptr(unsafe.Pointer(className)),
		0,
	)
	if hwnd == 0 {
		return
	}

	const (
		NIF_INFO   = 0x00000010
		NIM_MODIFY = 0x00000001
		NIIF_INFO  = 0x00000001
	)

	var nid balloonNotifyIconData
	nid.Size = uint32(unsafe.Sizeof(nid))
	nid.Wnd = hwnd
	nid.ID = 100
	nid.Flags = NIF_INFO
	nid.InfoFlags = NIIF_INFO

	titleUTF16, _ := syscall.UTF16FromString(title)
	copy(nid.InfoTitle[:], titleUTF16)

	msgUTF16, _ := syscall.UTF16FromString(message)
	copy(nid.Info[:], msgUTF16)

	pShellNotifyIconW.Call(
		uintptr(NIM_MODIFY),
		uintptr(unsafe.Pointer(&nid)),
	)
}
