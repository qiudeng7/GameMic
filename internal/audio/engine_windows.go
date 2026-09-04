package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
	"github.com/qiudeng7/GameMic/internal/dsp"
)

type Endpoint struct {
	ID, Name string
	Default  bool
	raw      malgo.DeviceID
}
type Engine struct {
	ctx             *malgo.AllocatedContext
	dev             *malgo.Device
	params          atomic.Pointer[dsp.Params]
	peakIn, peakOut atomic.Uint32
	lastCallback    atomic.Int64
	stopped         atomic.Bool
}

func New() (*Engine, error) {
	ctx, err := malgo.InitContext([]malgo.Backend{malgo.BackendWasapi}, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("初始化 Windows 音频失败：%w", err)
	}
	e := &Engine{ctx: ctx}
	e.SetParams(dsp.Params{GainDB: 20, ThresholdDB: -50})
	return e, nil
}
func (e *Engine) Close() {
	e.Stop()
	if e.ctx != nil {
		e.ctx.Uninit()
		e.ctx.Free()
		e.ctx = nil
	}
}
func (e *Engine) SetParams(p dsp.Params) { v := p.Normalized(); e.params.Store(&v) }

func isCable(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "vb-audio") && strings.Contains(n, "cable") || strings.HasPrefix(n, "cable output") || strings.HasPrefix(n, "cable input")
}
func isPrimaryCable(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "vb-audio virtual cable") || strings.HasPrefix(n, "cable input (") || n == "cable input"
}
func (e *Engine) Devices() (inputs, outputs []Endpoint, err error) {
	for _, kind := range []malgo.DeviceType{malgo.Capture, malgo.Playback} {
		list, x := e.ctx.Devices(kind)
		if x != nil {
			return nil, nil, x
		}
		for _, d := range list {
			name := d.Name()
			item := Endpoint{ID: d.ID.String(), Name: name, Default: d.IsDefault != 0, raw: d.ID}
			if kind == malgo.Capture && !isCable(name) {
				inputs = append(inputs, item)
			}
			if kind == malgo.Playback && isPrimaryCable(name) {
				outputs = append(outputs, item)
			}
		}
	}
	return
}
func (e *Engine) Start(input, output Endpoint) error {
	e.Stop()
	if !isPrimaryCable(output.Name) || isCable(input.Name) {
		return fmt.Errorf("请选择真实麦克风和 VB-CABLE 虚拟输出")
	}
	p := dsp.New(*e.params.Load())
	cfg := malgo.DefaultDeviceConfig(malgo.Duplex)
	cfg.SampleRate = dsp.SampleRate
	cfg.PeriodSizeInMilliseconds = 10
	cfg.Capture.Format = malgo.FormatF32
	cfg.Capture.Channels = 1
	cfg.Playback.Format = malgo.FormatF32
	cfg.Playback.Channels = 2
	// The IDs contain no Go pointers. miniaudio copies them during InitDevice;
	// do not use DeviceID.Pointer(), which allocates C memory owned by the caller.
	inputID, outputID := input.raw, output.raw
	cfg.Capture.DeviceID = unsafe.Pointer(&inputID)
	cfg.Playback.DeviceID = unsafe.Pointer(&outputID)
	cfg.Wasapi.NoAutoStreamRouting = 1
	e.stopped.Store(false)
	dev, err := malgo.InitDevice(e.ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: func(out, in []byte, frames uint32) {
			clear(out)
			p.Configure(*e.params.Load())
			n := min(int(frames), len(in)/4, len(out)/8)
			var inPeak, outPeak float32
			for i := 0; i < n; i++ {
				x := math.Float32frombits(binary.LittleEndian.Uint32(in[i*4:]))
				y := p.Sample(x)
				a := float32(math.Abs(float64(x)))
				if a > inPeak && !math.IsInf(float64(a), 0) {
					inPeak = a
				}
				b := float32(math.Abs(float64(y)))
				if b > outPeak {
					outPeak = b
				}
				bits := math.Float32bits(y)
				binary.LittleEndian.PutUint32(out[i*8:], bits)
				binary.LittleEndian.PutUint32(out[i*8+4:], bits)
			}
			storePeak(&e.peakIn, inPeak)
			storePeak(&e.peakOut, outPeak)
			e.lastCallback.Store(time.Now().UnixNano())
		},
		Stop: func() { e.stopped.Store(true) },
	})
	runtime.KeepAlive(inputID)
	runtime.KeepAlive(outputID)
	if err != nil {
		return fmt.Errorf("打开麦克风或虚拟设备失败：%w", err)
	}
	e.dev = dev
	e.lastCallback.Store(time.Now().UnixNano())
	if err = dev.Start(); err != nil {
		e.Stop()
		return fmt.Errorf("启动音频失败：%w", err)
	}
	return nil
}
func storePeak(dst *atomic.Uint32, v float32) {
	bits := math.Float32bits(v)
	for old := dst.Load(); bits > old; old = dst.Load() {
		if dst.CompareAndSwap(old, bits) {
			return
		}
	}
}
func (e *Engine) Stop() {
	if e.dev != nil {
		e.dev.Uninit()
		e.dev = nil
	}
	e.peakIn.Store(0)
	e.peakOut.Store(0)
}
func (e *Engine) Running() bool { return e.dev != nil }
func (e *Engine) Healthy() bool {
	return e.dev != nil && !e.stopped.Load() && time.Since(time.Unix(0, e.lastCallback.Load())) < 3*time.Second
}
func (e *Engine) Peaks() (float32, float32) {
	return math.Float32frombits(e.peakIn.Swap(0)), math.Float32frombits(e.peakOut.Swap(0))
}
