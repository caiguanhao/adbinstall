package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

var (
	version = "1.0"

	imageDir = filepath.Join(os.Getenv("PROGRAMDATA"), "AndroidUpdater", "image")

	md            *walk.Dialog
	adbAddress    *walk.ComboBox
	scanButton    *walk.LinkLabel
	console       *walk.TextEdit
	viewButton    *walk.PushButton
	imageButton   *walk.PushButton
	flashButton   *walk.PushButton
	apkLinkLabel  *walk.LinkLabel
	apkFilePaths  []string
	openButton    *walk.PushButton
	installButton *walk.PushButton
	installedPkgs *walk.ComboBox
	reloadButton  *walk.PushButton
	uninstallBtn  *walk.PushButton

	existingAdbPid = -1

	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	lastAdbAddress string
)

func init() {
	reader, writer := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			if console == nil {
				continue
			}
			console.AppendText(scanner.Text() + "\r\n")
		}
	}()
	log.SetOutput(writer)
}

func main() {
	windowTitle := fmt.Sprintf("Android Updater (ver %s)", version)
	if alreadyRunning() {
		win.SetForegroundWindow(win.FindWindow(nil, syscall.StringToUTF16Ptr(windowTitle)))
		return
	}
	Dialog{
		AssignTo:  &md,
		Layout:    VBox{},
		Title:     windowTitle,
		MinSize:   Size{600, 400},
		FixedSize: true,
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					TextLabel{
						Text:          "ADB address:",
						StretchFactor: 1,
						TextAlignment: AlignHNearVCenter,
					},
					Composite{
						StretchFactor: 5,
						Layout: HBox{
							Margins: Margins{
								Top: 1,
							},
						},
						Children: []Widget{
							ComboBox{
								AssignTo: &adbAddress,
								Editable: true,
							},
							LinkLabel{
								AssignTo:      &scanButton,
								StretchFactor: 1,
								Text:          "<a>Scan</a>",
								OnLinkActivated: func(_ *walk.LinkLabelLink) {
									go func() {
										scanButton.SetText("Scanning")
										defer scanButton.SetText("<a>Scan</a>")
										adbAddress.SetModel(getLocalADBAddresses())
									}()
								},
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						StretchFactor: 1,
					},
					HSplitter{
						StretchFactor: 5,
						Children: []Widget{
							PushButton{
								AssignTo: &viewButton,
								Text:     "VIEW",
								OnClicked: func() {
									start()
								},
							},
							PushButton{
								AssignTo: &imageButton,
								Text:     "IMAGE...",
								OnClicked: func() {
									showDownloader()
								},
							},
							PushButton{
								AssignTo: &flashButton,
								Text:     "FLASH",
								OnClicked: func() {
									flash()
								},
							},
							TextLabel{
								StretchFactor: 2,
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						Text:          "APK files:",
						StretchFactor: 1,
						TextAlignment: AlignHNearVCenter,
					},
					HSplitter{
						StretchFactor: 5,
						Children: []Widget{
							LinkLabel{
								AssignTo:      &apkLinkLabel,
								StretchFactor: 1,
								Text:          "No file selected",
								OnLinkActivated: func(link *walk.LinkLabelLink) {
									exec.Command("explorer.exe", "/select,", link.URL()).Run()
								},
							},
							TextLabel{
								StretchFactor: 1,
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						StretchFactor: 1,
					},
					HSplitter{
						StretchFactor: 5,
						Children: []Widget{
							PushButton{
								AssignTo: &openButton,
								Text:     "SELECT...",
								OnClicked: func() {
									openFile()
								},
							},
							PushButton{
								AssignTo: &installButton,
								Text:     "INSTALL",
								OnClicked: func() {
									install()
								},
							},
							TextLabel{
								StretchFactor: 3,
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						Text:          "Installed Packages:",
						StretchFactor: 1,
						TextAlignment: AlignHNearVCenter,
					},
					Composite{
						StretchFactor: 5,
						Layout: VBox{
							Margins: Margins{
								Top: 1,
							},
						},
						Children: []Widget{
							ComboBox{
								AssignTo: &installedPkgs,
								OnCurrentIndexChanged: func() {
									uninstallBtn.SetEnabled(installedPkgs.Text() != "")
								},
							},
						},
					},
				},
			},
			HSplitter{
				Children: []Widget{
					TextLabel{
						StretchFactor: 1,
					},
					HSplitter{
						StretchFactor: 5,
						Children: []Widget{
							PushButton{
								AssignTo: &reloadButton,
								Text:     "RELOAD",
								OnClicked: func() {
									reload()
								},
							},
							PushButton{
								AssignTo: &uninstallBtn,
								Text:     "UNINSTALL",
								OnClicked: func() {
									uninstall()
								},
							},
							TextLabel{
								StretchFactor: 3,
							},
						},
					},
				},
			},
			TextEdit{
				AssignTo: &console,
				VScroll:  true,
				ReadOnly: true,
			},
			LinkLabel{
				Alignment: AlignHFarVCenter,
				Text:      `<a href="https://github.com/caiguanhao/adbinstall">View Source</a>`,
				OnLinkActivated: func(link *walk.LinkLabelLink) {
					exec.Command("explorer.exe", link.URL()).Run()
				},
			},
		},
	}.Create(nil)
	updateDialog(md)
	enable()
	md.Run()
	if existingAdbPid == 0 {
		// kill adb server if it is created by this program
		if pid := findADBProcess(); pid > 0 {
			if p, _ := os.FindProcess(pid); p != nil {
				p.Kill()
			}
		}
	}
}

func updateDialog(d *walk.Dialog) {
	dpi := float64(d.DPI()) / 96
	screenWidth := int(float64(win.GetSystemMetrics(win.SM_CXSCREEN)) / dpi)
	screenHeight := int(float64(win.GetSystemMetrics(win.SM_CYSCREEN)) / dpi)
	d.SetX((screenWidth - d.MinSize().Width) / 2)
	d.SetY((screenHeight - d.MinSize().Height) / 2)
	icon, _ := walk.NewIconFromResourceId(2)
	if icon != nil {
		d.SetIcon(icon)
	}
}

func disable() {
	adbAddress.SetEnabled(false)
	viewButton.SetEnabled(false)
	imageButton.SetEnabled(false)
	flashButton.SetEnabled(false)
	openButton.SetEnabled(false)
	installButton.SetEnabled(false)
	installedPkgs.SetEnabled(false)
	reloadButton.SetEnabled(false)
	uninstallBtn.SetEnabled(false)
}

func enable() {
	adbAddress.SetEnabled(true)
	viewButton.SetEnabled(true)
	imageButton.SetEnabled(true)
	flashButton.SetEnabled(true)
	openButton.SetEnabled(true)
	installButton.SetEnabled(len(apkFilePaths) > 0)
	installedPkgs.SetEnabled(true)
	reloadButton.SetEnabled(true)
	uninstallBtn.SetEnabled(installedPkgs.Text() != "")
}

func connect() (funcs []func() bool) {
	addr := strings.TrimSpace(adbAddress.Text())
	if addr == lastAdbAddress {
		return
	}
	if addr == "" {
		lastAdbAddress = ""
		return
	}
	funcs = append(funcs,
		run("cmd", "/c", `lib\adb.exe`, "disconnect"),
		run("cmd", "/c", `lib\adb.exe`, "connect", addr),
		func() bool {
			lastAdbAddress = addr
			return true
		},
	)
	return
}

func start() {
	go disable()
	go func() {
		defer enable()
		funcs := connect()
		funcs = append(funcs,
			run("cmd", "/c", `lib\adb.exe`, "devices"),
			run("cmd", "/c", `lib\scrcpy.exe`),
		)
		if existingAdbPid == -1 {
			existingAdbPid = findADBProcess()
		}
		for _, f := range funcs {
			if f() != true {
				return
			}
		}
	}()
}

func flash() {
	ret := walk.MsgBox(md, "Flash Image",
		"Are you sure you want to flash image to the Android device? This will delete everything on the device!",
		walk.MsgBoxYesNo|walk.MsgBoxIconExclamation|walk.MsgBoxDefButton2,
	)
	if ret != walk.DlgCmdYes {
		return
	}
	go disable()
	go func() {
		defer enable()
		funcs := connect()
		funcs = append(funcs,
			run("cmd", "/c", `lib\adb.exe`, "reboot", "bootloader"),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "devcfg", filepath.Join(imageDir, "devcfg.mbn")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "devcfgbak", filepath.Join(imageDir, "devcfg.mbn")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "dsp", filepath.Join(imageDir, "adspso.bin")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "cache", filepath.Join(imageDir, "cache.img")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "aboot", filepath.Join(imageDir, "emmc_appsboot.mbn")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "boot", filepath.Join(imageDir, "boot.img")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "persist", filepath.Join(imageDir, "persist.img")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "recovery", filepath.Join(imageDir, "recovery.img")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "-S", "500M", "system", filepath.Join(imageDir, "system.img")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "flash", "userdata", filepath.Join(imageDir, "userdata.img")),
			run("cmd", "/c", filepath.Join(imageDir, "fastboot.exe"), "reboot"),
		)
		if existingAdbPid == -1 {
			existingAdbPid = findADBProcess()
		}
		for _, f := range funcs {
			if f() != true {
				return
			}
		}
	}()
}

func openFile() {
	dlg := new(walk.FileDialog)
	dlg.Filter = "APK (*.apk)|*.apk"
	dlg.Title = "Select an APK"
	dlg.ShowOpenMultiple(md)
	apkFilePaths = dlg.FilePaths[:]
	if len(apkFilePaths) > 0 {
		text := ""
		for i, p := range apkFilePaths {
			if i > 1 {
				text += fmt.Sprintf(" and %d file(s)", len(apkFilePaths)-i)
				break
			} else if i > 0 {
				text += ", "
			}
			text += fmt.Sprintf(`<a href="%s">%s</a>`, p, filepath.Base(p))
		}
		apkLinkLabel.SetText(text)
	} else {
		apkLinkLabel.SetText("No file selected")
	}
	installButton.SetEnabled(len(apkFilePaths) > 0)
}

func install() {
	go disable()
	go func() {
		defer enable()
		funcs := connect()
		for _, path := range apkFilePaths {
			apk := strings.TrimSpace(path)
			if apk != "" {
				funcs = append(funcs,
					println("Installing", apk),
					run("cmd", "/c", `lib\adb.exe`, "install", "-r", apk),
				)
			}
		}
		if existingAdbPid == -1 {
			existingAdbPid = findADBProcess()
		}
		for _, f := range funcs {
			if f() != true {
				return
			}
		}
		reload()
	}()
}

func reload() {
	go disable()
	go func() {
		defer enable()
		funcs := connect()
		if existingAdbPid == -1 {
			existingAdbPid = findADBProcess()
		}
		for _, f := range funcs {
			if f() != true {
				return
			}
		}
		out := output("cmd", "/c", `lib\adb.exe`, "shell", "cmd", "package", "list", "packages", "-3")
		installedPkgs.SetModel(strings.Fields(regexp.MustCompile("(?m)^package:").ReplaceAllString(string(out), "")))
		installedPkgs.SetCurrentIndex(0)
	}()
}

func uninstall() {
	go disable()
	go func() {
		defer enable()
		funcs := connect()
		pkg := strings.TrimSpace(installedPkgs.Text())
		if pkg != "" {
			funcs = append(funcs,
				println("Uninstalling", pkg),
				run("cmd", "/c", `lib\adb.exe`, "uninstall", pkg),
			)
		}
		if existingAdbPid == -1 {
			existingAdbPid = findADBProcess()
		}
		for _, f := range funcs {
			if f() != true {
				return
			}
		}
		reload()
	}()
}

func run(name string, args ...string) func() bool {
	return func() (success bool) {
		cmd := exec.Command(name, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Println(err)
			return
		}
		go logReader(stdout)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Println(err)
			return
		}
		go logReader(stderr)
		if err := cmd.Start(); err != nil {
			log.Println(err)
			return
		}
		if err := cmd.Wait(); err != nil {
			log.Println(err)
			return
		}
		success = true
		return
	}
}

func println(v ...interface{}) func() bool {
	return func() bool {
		log.Println(v...)
		return true
	}
}

func output(name string, args ...string) (out []byte) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, _ = cmd.Output()
	return
}

func logReader(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
}

func alreadyRunning() bool {
	procCreateMutex := kernel32.NewProc("CreateMutexW")
	_, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("adbinstall"))))
	return int(err.(syscall.Errno)) != 0
}

func findADBProcess() (pid int) {
	// github.com/mitchellh/go-ps
	handle, _, _ := kernel32.NewProc("CreateToolhelp32Snapshot").Call(0x00000002, 0)
	if handle < 0 {
		return
	}
	defer kernel32.NewProc("CloseHandle").Call(handle)
	var entry struct {
		Size              uint32
		CntUsage          uint32
		ProcessID         uint32
		DefaultHeapID     uintptr
		ModuleID          uint32
		CntThreads        uint32
		ParentProcessID   uint32
		PriorityClassBase int32
		Flags             uint32
		ExeFile           [260]uint16
	}
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := kernel32.NewProc("Process32FirstW").Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return
	}
	for {
		e := &entry
		end := 0
		for {
			if e.ExeFile[end] == 0 {
				break
			}
			end++
		}
		if syscall.UTF16ToString(e.ExeFile[:end]) == "adb.exe" {
			pid = int(e.ProcessID)
			return
		}
		ret, _, _ := kernel32.NewProc("Process32NextW").Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return
}
