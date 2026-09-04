package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndReplace(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config", "settings.json")
	s, err := Load(p)
	if err != nil || s.GainDB != 20 || s.Gate {
		t.Fatal(s, err)
	}
	s.InputID = "mic-1"
	s.GainDB = 24
	if err = Save(p, s); err != nil {
		t.Fatal(err)
	}
	s.GainDB = 18
	if err = Save(p, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || got != s {
		t.Fatal(got, err)
	}
}
func TestInvalidConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(p, []byte(`{"gain_db":999,"gate_threshold_db":-999}`), 0600)
	s, err := Load(p)
	if err != nil || s.GainDB != 30 || s.ThresholdDB != -65 {
		t.Fatal(s, err)
	}
	os.WriteFile(p, []byte(`{broken`), 0600)
	s, err = Load(p)
	if err == nil || s != Default() {
		t.Fatal(s, err)
	}
}
