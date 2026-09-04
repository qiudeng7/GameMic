package dsp

import (
	"math"
	"testing"
)

func TestTwentyDBIsTenTimesAmplitude(t *testing.T) {
	p := New(Params{GainDB:20, ThresholdDB:-50})
	if got := p.Sample(0.02); math.Abs(float64(got)-0.2)>1e-6 { t.Fatalf("+20 dB: got %v, want 0.2",got) }
}

func TestLimiterBoundsBothPolaritiesAndBadInput(t *testing.T) {
	p := New(Params{GainDB:30})
	for _, x := range []float32{0,1,-1,0.01,10,-10,float32(math.Inf(1)),float32(math.NaN())} {
		for i:=0;i<4800;i++ {
			y:=p.Sample(x)
			if math.IsNaN(float64(y)) || math.IsInf(float64(y),0) || math.Abs(float64(y))>0.950001 { t.Fatalf("unsafe output %v for %v",y,x) }
		}
	}
}

func TestGateSuppressesNoiseAndPassesVoice(t *testing.T) {
	p:=New(Params{GainDB:20,Gate:true,ThresholdDB:-50})
	var y float32
	for i:=0;i<SampleRate;i++ { y=p.Sample(0.0001) }
	if math.Abs(float64(y))>0.00001 { t.Fatalf("noise leaked: %v",y) }
	for i:=0;i<SampleRate/5;i++ { y=p.Sample(0.02) }
	if y<0.19 { t.Fatalf("voice was gated: %v",y) }
	for i:=0;i<SampleRate;i++ { y=p.Sample(0.0001) }
	if math.Abs(float64(y))>0.00001 { t.Fatalf("gate did not close: %v",y) }
}

func TestGainChangeIsSmoothed(t *testing.T) {
	p:=New(Params{GainDB:0})
	p.Configure(Params{GainDB:20})
	if y:=p.Sample(0.01); y>0.011 { t.Fatalf("gain jumped: %v",y) }
	var y float32
	for i:=0;i<SampleRate;i++ { y=p.Sample(0.01) }
	if math.Abs(float64(y)-0.1)>1e-5 { t.Fatalf("gain never reached target: %v",y) }
}

func TestProcessingDoesNotAllocate(t *testing.T) {
	p:=New(Params{GainDB:20})
	if n:=testing.AllocsPerRun(100,func(){ for i:=0;i<480;i++ { p.Sample(0.01) } });n!=0 {t.Fatalf("allocations: %v",n)}
}
