# GameMic

轻量 Windows 麦克风放大工具。Go 原生界面，只做“把真实麦克风的声音放大，再送进游戏可选的虚拟麦克风”。无需账号、无需显卡加速，不上传音频。

**[下载安装包](https://github.com/qiudeng7/GameMic/releases/latest)** · [构建状态](https://github.com/qiudeng7/GameMic/actions/workflows/windows.yml)

## 使用

1. 下载 `GameMic-Setup-版本-windows-amd64.exe`。完整安装包已包含 VB-CABLE；不需要分别下载多个软件。
2. 首次安装会打开官方 VB-CABLE 安装器，点击 **Install Driver**，完成后重启电脑。如果显示 **Remove Driver**，说明已安装，关闭该窗口即可。
3. 打开 GameMic，选择真实麦克风，先用默认 **+20 dB**，点击“开始放大”。
4. 在游戏 / Discord 的麦克风设置里选择 **CABLE Output (VB-Audio Virtual Cable)**。

Windows 的默认扬声器保持为你的耳机或音箱。GameMic 自动将处理后的声音送至 `CABLE Input`；游戏从 `CABLE Output` 收音。不要把虚拟输出选作 GameMic 的源麦克风。

按“后台运行”或关闭窗口后继续在托盘运行；右键托盘或点击“退出”才会退出。程序不会自动开机启动，也不会在下次启动时未经操作开始录音。设置保存在 `%APPDATA%\GameMic\settings.json`。

## 功能与边界

- 增益 0～+30 dB 可调，默认 +20 dB（10 倍振幅），调节时平滑过渡。
- 始终开启峰值限幅，控制输出在 ±0.95 内，降低数字削波风险。
- 可关闭的噪声门，阈值 -65～-25 dBFS，默认关闭。它只压低停顿时的底噪，不是 AI 降噪；轻声被吞时关闭它。
- 两条电平表分别显示原始输入、处理后输出。
- 48 kHz 单声道采集，双声道相同语音输出；WASAPI 共享模式，由 miniaudio 处理设备格式转换。
- 10 ms 请求处理周期；实际端到端延迟由声卡、VB-CABLE 和游戏缓冲共同决定，未承诺固定延迟。
- 禁止把检测到的 VB-CABLE 回录为输入，只向基础 VB-CABLE 输出，避免误送扬声器产生反馈。
- 音频中断时提示并停止；重新连接后刷新设备，再开始。

软件放大也会放大原有噪声，无法恢复麦克风采集时已经丢失或削波的信号。游戏自身的自动增益、降噪和语音门限仍可能影响队友听到的效果。

## 系统与打包

Windows 10/11 x64，安装驱动需要管理员权限；日常运行不需要管理员权限。Go 写应用主体，系统虚拟设备由 VB-CABLE 驱动实现。

- `GameMic-Setup-…exe`：首次使用推荐，包含应用与原版驱动包。
- `GameMic-…portable.zip`：适合已经安装 VB-CABLE 的电脑。
- `SHA256SUMS.txt`：下载文件校验值。

GameMic 应用和外层安装器未签名，Windows 可能显示未知发布者。构建会验证内附 VB-CABLE 官方安装器的 Authenticode 签名，并记录驱动压缩包哈希。卸载 GameMic 保留共享的 VB-CABLE，以免影响其他应用；不再需要时用官方驱动安装器单独卸载。

## 常见问题

**没有虚拟麦克风**：首次安装驱动后重启；打开 GameMic 刷新设备。若仍缺失，检查系统声音设置是否禁用了 CABLE Input / Output。

**输入电平不动 / 启动失败**：确认选择了真实麦克风，并在 Windows 设置 → 隐私与安全性 → 麦克风中允许桌面应用访问。关闭独占麦克风的其他程序。

**输入有电平但队友听不到**：确认点击了开始，游戏选的是 CABLE Output，游戏内麦克风音量没有归零。检查是否开启了按键说话或过高的语音激活阈值。

**轻声被截掉**：先关闭 GameMic 噪声门，再检查游戏自己的降噪和门限。

**声音破音 / 一直很响**：降低增益。软件限幅不能修复进入电脑前就已失真的麦克风信号。

**需要重命名设备吗**：不需要，直接选择官方名称。若重命名导致检测不到，请恢复带有 VB-Audio Virtual Cable 的设备名称。

## 开发与发布

依赖 Go 1.25.1、MinGW-w64（gcc/windres）和 Inno Setup 6。音频使用 `malgo/miniaudio`，界面使用 Walk/Win32；无 Electron、WebView 或外置运行时依赖。

在 Windows PowerShell 中，确保 Go 与 MinGW-w64 在 PATH：

```powershell
$env:CGO_ENABLED = '1'
./scripts/package.ps1
```

构建脚本执行依赖验证、DSP/设置测试、资源编译、应用编译、`go vet`、GUI 启动退出检查，随后下载和验证官方 VB-CABLE，生成完整安装包与便携包。

推送 main 自动构建；`VERSION` 对应的 Release 不存在时自动发布。已有 Release 不会覆盖；修改 `VERSION` 并更新 `RELEASE-NOTES.md` 才发布新版。PR 只构建不发布。

```text
cmd/gamemic/       原生窗口、托盘、设备选择
internal/audio/    WASAPI 采集与虚拟输出
internal/dsp/      增益、噪声门、限幅（纯 Go，可独立测试）
internal/settings/设置保存与恢复
build/            Windows manifest、Inno Setup 配方
scripts/          完整构建与打包
```

自动测试不等于真实硬件验证。发布前后的人工检查：安装 / 重启 / 设备出现；真实麦克风 +20 dB；游戏选择虚拟麦克风；托盘运行；拔插设备；停止 / 退出后静音。首版硬件与游戏兼容性需在目标电脑验证。

## 许可证

GameMic 源码采用 MIT。第三方组件见 [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)。

VB-CABLE © VB-Audio Software / Vincent Burel，为独立的 Donationware，不属于 GameMic 的 MIT 许可。

- 来源：https://vb-audio.com/Cable/
- **VB-CABLE is donationware; all participations are welcome.**
- 支持作者和许可说明：https://vb-audio.com/Services/licensing.htm

完整安装包按官方基础 VB-CABLE 分发条款保留原始文件、作者信息和捐赠渠道；专业部署请遵循厂商对应许可。
