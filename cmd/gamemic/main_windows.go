package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"github.com/qiudeng7/GameMic/internal/audio"
	"github.com/qiudeng7/GameMic/internal/settings"
	"golang.org/x/sys/windows"
)

var version = "dev"

type application struct {
	mw                                        *walk.MainWindow
	mic                                       *walk.ComboBox
	gain, threshold                           *walk.Slider
	gate                                      *walk.CheckBox
	gainLabel, thresholdLabel, status, meters *walk.Label
	inputMeter, outputMeter                   *walk.ProgressBar
	start, refresh, driver                    *walk.PushButton
	tray                                      *walk.NotifyIcon
	engine                                    *audio.Engine
	inputs, outputs                           []audio.Endpoint
	config                                    settings.Settings
	configPath                                string
	loading, quitting, smoke                  bool
	dirty                                     bool
}

func main() {
	runtime.LockOSThread()
	check := len(os.Args) > 1 && os.Args[1] == "--check-cable"
	smoke := len(os.Args) > 1 && os.Args[1] == "--smoke-test"
	if check {
		e, err := audio.New()
		if err != nil {
			os.Exit(2)
		}
		_, out, err := e.Devices()
		e.Close()
		if err != nil || len(out) == 0 {
			os.Exit(1)
		}
		return
	}
	if !smoke {
		name, _ := windows.UTF16PtrFromString(`Local\GameMic.SingleInstance`)
		h, err := windows.CreateMutex(nil, false, name)
		if h != 0 {
			defer windows.CloseHandle(h)
		}
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			walk.MsgBox(nil, "GameMic", "GameMic 已在运行，请点击任务栏右下角的托盘图标。", walk.MsgBoxIconInformation)
			return
		}
		if err != nil {
			walk.MsgBox(nil, "GameMic", err.Error(), walk.MsgBoxIconError)
			os.Exit(1)
		}
	}
	a := &application{config: settings.Default(), smoke: smoke, loading: true}
	var loadErr error
	if !smoke {
		a.configPath, loadErr = settings.Path()
		if loadErr == nil {
			a.config, loadErr = settings.Load(a.configPath)
		}
	}
	engine, engineErr := audio.New()
	if engineErr == nil {
		a.engine = engine
		defer engine.Close()
		engine.SetParams(a.config.Params())
	}
	if err := a.createUI(); err != nil {
		if smoke {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		walk.MsgBox(nil, "GameMic 启动失败", err.Error(), walk.MsgBoxIconError)
		os.Exit(1)
	}
	defer a.mw.Dispose()
	a.loading = false
	a.updateParams()
	a.dirty = false
	if engineErr != nil {
		a.status.SetText(engineErr.Error())
		a.start.SetEnabled(false)
	} else {
		a.enumerate()
	}
	if loadErr != nil {
		walk.MsgBox(a.mw, "设置未载入", "无法读取设置，已使用默认值：\n"+loadErr.Error(), walk.MsgBoxIconWarning)
	}
	if !smoke {
		a.createTray()
	}
	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		a.save()
		if !a.quitting && a.tray != nil && reason == walk.CloseReasonUser {
			*canceled = true
			a.mw.Hide()
			return
		}
	})
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		timer := time.NewTicker(100 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-quit:
				return
			case <-timer.C:
				a.mw.Synchronize(func() {
					if !a.quitting {
						a.tick()
					}
				})
			}
		}
	}()
	if smoke {
		go func() { time.Sleep(2 * time.Second); a.mw.Synchronize(func() { a.quitting = true; a.mw.Close() }) }()
	}
	a.mw.Run()
	a.quitting = true
	if a.tray != nil {
		a.tray.Dispose()
	}
	a.save()
}

func (a *application) createUI() error {
	return (MainWindow{
		AssignTo: &a.mw, Title: "GameMic · 麦克风放大 " + version,
		MinSize: Size{Width: 540, Height: 580}, Size: Size{Width: 580, Height: 650},
		Font:   Font{Family: "Microsoft YaHei UI", PointSize: 10},
		Layout: VBox{Margins: Margins{Left: 20, Top: 16, Right: 20, Bottom: 16}, Spacing: 9},
		Children: []Widget{
			Label{Text: "让队友听清你的声音", Font: Font{Family: "Microsoft YaHei UI", PointSize: 17, Bold: true}},
			Label{Text: "选择麦克风 → 调整增益 → 点击开始"},
			Label{Text: "真实麦克风"},
			Composite{Layout: HBox{}, Children: []Widget{
				ComboBox{AssignTo: &a.mic, StretchFactor: 1, OnCurrentIndexChanged: func() {
					if !a.loading {
						a.rememberInput()
					}
				}},
				PushButton{AssignTo: &a.refresh, Text: "刷新设备", OnClicked: a.enumerate},
			}},
			Label{AssignTo: &a.gainLabel, Text: "声音增益"},
			Slider{AssignTo: &a.gain, MinValue: 0, MaxValue: 30, Value: int(a.config.GainDB), Tracking: true, OnValueChanged: a.updateParams},
			Label{Text: "0 dB 原音量     +20 dB = 10 倍振幅     +30 dB 最大"},
			CheckBox{AssignTo: &a.gate, Text: "简单降噪（噪声门，默认关闭）", Checked: a.config.Gate, OnCheckedChanged: a.updateParams},
			Label{AssignTo: &a.thresholdLabel, Text: "降噪阈值"},
			Slider{AssignTo: &a.threshold, MinValue: -65, MaxValue: -25, Value: int(a.config.ThresholdDB), Tracking: true, OnValueChanged: a.updateParams},
			Label{Text: "只压低停顿时的底噪；如果轻声被截断，请关闭降噪。"},
			Label{AssignTo: &a.meters, Text: "输入 / 输出电平"},
			ProgressBar{AssignTo: &a.inputMeter, MinValue: 0, MaxValue: 60},
			ProgressBar{AssignTo: &a.outputMeter, MinValue: 0, MaxValue: 60},
			Label{Text: "游戏的麦克风请选择：CABLE Output (VB-Audio Virtual Cable)", Font: Font{Family: "Microsoft YaHei UI", PointSize: 10, Bold: true}},
			Label{AssignTo: &a.status, Text: "正在检查音频设备…", MinSize: Size{Width: 0, Height: 42}},
			Composite{Layout: HBox{}, Children: []Widget{
				PushButton{AssignTo: &a.start, Text: "开始放大", OnClicked: a.toggle},
				PushButton{AssignTo: &a.driver, Text: "安装虚拟麦克风", OnClicked: a.installDriver},
				HSpacer{},
				PushButton{Text: "声音设置", OnClicked: func() { a.open("ms-settings:sound") }},
			}},
			Composite{Layout: HBox{}, Children: []Widget{
				PushButton{Text: "后台运行", OnClicked: func() {
					if a.tray != nil {
						a.mw.Hide()
					} else {
						win.ShowWindow(a.mw.Handle(), win.SW_MINIMIZE)
					}
				}},
				PushButton{Text: "退出", OnClicked: a.exit},
				HSpacer{},
				PushButton{Text: "关于 / VB-CABLE", OnClicked: a.about},
			}},
		},
	}).Create()
}

func (a *application) updateParams() {
	if a.loading || a.gain == nil || a.gate == nil || a.threshold == nil {
		return
	}
	a.config.GainDB = float64(a.gain.Value())
	a.config.Gate = a.gate.Checked()
	a.config.ThresholdDB = float64(a.threshold.Value())
	a.gainLabel.SetText(fmt.Sprintf("声音增益：+%.0f dB", a.config.GainDB))
	a.thresholdLabel.SetText(fmt.Sprintf("噪声门阈值：%.0f dBFS（越靠右越容易截断轻声）", a.config.ThresholdDB))
	a.threshold.SetEnabled(a.config.Gate)
	if a.engine != nil {
		a.engine.SetParams(a.config.Params())
	}
	a.dirty = true
}
func (a *application) rememberInput() {
	i := a.mic.CurrentIndex()
	if i >= 0 && i < len(a.inputs) {
		a.config.InputID = a.inputs[i].ID
		a.dirty = true
	}
}
func (a *application) enumerate() {
	if a.engine == nil {
		return
	}
	if a.engine.Running() {
		return
	}
	a.loading = true
	defer func() { a.loading = false }()
	in, out, err := a.engine.Devices()
	if err != nil {
		a.status.SetText("设备枚举失败：" + err.Error())
		a.start.SetEnabled(false)
		return
	}
	a.inputs, a.outputs = in, out
	names := make([]string, len(in))
	index := -1
	for i, d := range in {
		names[i] = d.Name
		if d.Default {
			index = i
		}
	}
	if index < 0 && len(in) > 0 {
		index = 0
	}
	for i, d := range in {
		if d.ID == a.config.InputID {
			index = i
			break
		}
	}
	a.mic.SetModel(names)
	a.mic.SetCurrentIndex(index)
	a.start.SetEnabled(len(in) > 0 && len(out) > 0)
	a.driver.SetEnabled(len(out) == 0)
	if len(out) == 0 {
		a.status.SetText("尚未发现虚拟麦克风。请安装驱动，重启电脑后再打开 GameMic。")
	} else if len(in) == 0 {
		a.status.SetText("未发现真实麦克风。请接入麦克风，然后刷新设备。")
	} else {
		a.status.SetText("已就绪。开始放大后，在游戏中选择 CABLE Output。")
		a.rememberInput()
	}
}
func (a *application) toggle() {
	if a.engine == nil {
		return
	}
	if a.engine.Running() {
		a.engine.Stop()
		a.runningUI(false)
		a.status.SetText("已停止；虚拟麦克风不再输出声音。")
		return
	}
	i := a.mic.CurrentIndex()
	if i < 0 || i >= len(a.inputs) || len(a.outputs) == 0 {
		return
	}
	a.start.SetEnabled(false)
	err := a.engine.Start(a.inputs[i], a.outputs[0])
	a.start.SetEnabled(true)
	if err != nil {
		a.status.SetText("启动失败，请检查设备和麦克风权限。")
		walk.MsgBox(a.mw, "无法开始放大", err.Error()+"\n\n请确认 Windows 已允许桌面应用访问麦克风，设备未被独占。", walk.MsgBoxIconError)
		return
	}
	a.rememberInput()
	a.save()
	a.runningUI(true)
	a.status.SetText("正在放大 → CABLE Output。关闭窗口会继续在托盘运行。")
}
func (a *application) runningUI(running bool) {
	a.mic.SetEnabled(!running)
	a.refresh.SetEnabled(!running)
	if running {
		a.start.SetText("停止放大")
	} else {
		a.start.SetText("开始放大")
	}
	if a.tray != nil {
		if running {
			a.tray.SetToolTip("GameMic · 正在放大麦克风")
		} else {
			a.tray.SetToolTip("GameMic · 已停止")
		}
	}
}
func db(v float32) float64 {
	if v <= 0 {
		return -60
	}
	return math.Max(-60, math.Min(0, 20*math.Log10(float64(v))))
}
func (a *application) tick() {
	if a.engine == nil {
		return
	}
	if a.engine.Running() && !a.engine.Healthy() {
		a.engine.Stop()
		a.runningUI(false)
		a.status.SetText("音频设备已断开或停止。请刷新设备后重新开始。")
		a.start.SetEnabled(false)
		if a.tray != nil {
			a.tray.ShowWarning("GameMic 音频中断", "请重新连接麦克风，刷新设备后点击开始放大。")
		}
	}
	in, out := a.engine.Peaks()
	a.inputMeter.SetValue(int(db(in) + 60))
	a.outputMeter.SetValue(int(db(out) + 60))
	a.meters.SetText(fmt.Sprintf("输入：%.0f dBFS     输出：%.0f dBFS", db(in), db(out)))
}
func (a *application) save() {
	if a.smoke || !a.dirty || a.configPath == "" {
		return
	}
	if err := settings.Save(a.configPath, a.config); err != nil {
		a.status.SetText("设置保存失败：" + err.Error())
		return
	}
	a.dirty = false
}
func (a *application) exit() { a.save(); a.quitting = true; a.mw.Close() }
func (a *application) createTray() {
	tray, err := walk.NewNotifyIcon(a.mw)
	if err != nil {
		return
	}
	if err = tray.SetIcon(walk.IconApplication()); err != nil {
		tray.Dispose()
		return
	}
	tray.SetToolTip("GameMic · 已停止")
	show := func() {
		a.mw.Show()
		win.ShowWindow(a.mw.Handle(), win.SW_RESTORE)
		win.SetForegroundWindow(a.mw.Handle())
	}
	tray.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			show()
		}
	})
	for _, item := range []struct {
		name string
		fn   func()
	}{{"打开 GameMic", show}, {"退出", a.exit}} {
		act := walk.NewAction()
		act.SetText(item.name)
		act.Triggered().Attach(item.fn)
		tray.ContextMenu().Actions().Add(act)
	}
	if err = tray.SetVisible(true); err != nil {
		tray.Dispose()
		return
	}
	a.tray = tray
}
func (a *application) open(target string) {
	verb, _ := windows.UTF16PtrFromString("open")
	file, _ := windows.UTF16PtrFromString(target)
	if err := windows.ShellExecute(windows.Handle(a.mw.Handle()), verb, file, nil, nil, win.SW_SHOWNORMAL); err != nil {
		walk.MsgBox(a.mw, "无法打开", err.Error(), walk.MsgBoxIconError)
	}
}
func (a *application) installDriver() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	path := filepath.Join(filepath.Dir(exe), "driver", "VBCABLE_Setup_x64.exe")
	if _, err = os.Stat(path); err != nil {
		walk.MsgBox(a.mw, "需要完整安装包", "请从 GitHub Releases 下载 GameMic-Setup 安装包，里面已包含驱动。", walk.MsgBoxIconInformation)
		a.open("https://github.com/qiudeng7/GameMic/releases/latest")
		return
	}
	if walk.MsgBox(a.mw, "安装虚拟麦克风", "将打开随包提供的 VB-CABLE 驱动安装器。\n点击 Install Driver，完成后重启电脑。\n如果显示 Remove Driver，说明已经安装，请关闭安装器。\n\nVB-CABLE 来自 vb-audio.com，为 Donationware；欢迎支持作者。", walk.MsgBoxOKCancel|walk.MsgBoxIconInformation) != walk.DlgCmdOK {
		return
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(path)
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(path))
	if err = windows.ShellExecute(windows.Handle(a.mw.Handle()), verb, file, nil, dir, win.SW_SHOWNORMAL); err != nil {
		walk.MsgBox(a.mw, "驱动安装未启动", err.Error(), walk.MsgBoxIconError)
	}
}
func (a *application) about() {
	walk.MsgBox(a.mw, "关于 GameMic", "GameMic "+version+"\nGo 编写的本地麦克风放大工具，不登录、不上传音频。\n\nVB-CABLE © VB-Audio Software / Vincent Burel\n来源：https://vb-audio.com/Cable/\nVB-CABLE is donationware; all participations are welcome.\n捐赠 / 许可：https://vb-audio.com/Services/licensing.htm\n\n噪声门只减少停顿底噪，不会消除说话时的键盘声。\n软件放大会同时放大麦克风中的噪声。", walk.MsgBoxIconInformation)
}
