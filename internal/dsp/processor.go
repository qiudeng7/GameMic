// Package dsp implements streaming mono voice gain, a noise gate and a limiter.
// All state belongs to one audio callback. Parameters are snapshotted by its caller.
package dsp

import "math"

const SampleRate = 48000

type Params struct {
	GainDB float64
	Gate bool
	ThresholdDB float64
}

func (p Params) Normalized() Params {
	if math.IsNaN(p.GainDB) || math.IsInf(p.GainDB, 0) { p.GainDB = 20 }
	if math.IsNaN(p.ThresholdDB) || math.IsInf(p.ThresholdDB, 0) { p.ThresholdDB = -50 }
	p.GainDB = math.Max(0, math.Min(30, p.GainDB))
	p.ThresholdDB = math.Max(-65, math.Min(-25, p.ThresholdDB))
	return p
}

type Processor struct {
	gain, gate, envelope, limiter float64
	hold int
	targetGain, threshold float64
	gateEnabled bool
}

func New(params Params) *Processor {
	p := &Processor{limiter: 1}
	p.Configure(params)
	p.gain = p.targetGain
	if !p.gateEnabled { p.gate = 1 }
	return p
}

// Configure is called once per block, on the same thread as Sample.
func (p *Processor) Configure(params Params) {
	params = params.Normalized()
	p.targetGain = math.Pow(10, params.GainDB/20)
	p.threshold = math.Pow(10, params.ThresholdDB/20)
	p.gateEnabled = params.Gate
}

// Sample allocates no memory. The peak limiter attacks immediately and releases
// over 80 ms; it bounds samples without relying on downstream hard clipping.
func (p *Processor) Sample(input float32) float32 {
	x := float64(input)
	if math.IsNaN(x) || math.IsInf(x, 0) { x = 0 }
	x = math.Max(-1, math.Min(1, x))
	level := math.Abs(x)
	envRate := 1.0 / (0.050 * SampleRate)
	if level > p.envelope { envRate = 1.0 / (0.001 * SampleRate) }
	p.envelope += envRate * (level - p.envelope)
	if p.envelope >= p.threshold { p.hold = int(0.120 * SampleRate) } else if p.hold > 0 { p.hold-- }
	gateTarget := 0.0
	if !p.gateEnabled || p.hold > 0 { gateTarget = 1 }
	gateRate := 1.0 / (0.060 * SampleRate)
	if gateTarget > p.gate { gateRate = 1.0 / (0.002 * SampleRate) }
	p.gate += gateRate * (gateTarget - p.gate)
	p.gain += (p.targetGain - p.gain) / (0.010 * SampleRate)
	y := x * p.gain * p.gate
	required := 1.0
	if math.Abs(y) > 0.95 { required = 0.95 / math.Abs(y) }
	if required < p.limiter { p.limiter = required } else { p.limiter += (required - p.limiter) / (0.080 * SampleRate) }
	return float32(y * p.limiter)
}
