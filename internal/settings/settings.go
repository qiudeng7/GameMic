package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"github.com/qiudeng7/GameMic/internal/dsp"
)

type Settings struct {
	GainDB float64 `json:"gain_db"`
	Gate bool `json:"noise_gate"`
	ThresholdDB float64 `json:"gate_threshold_db"`
	InputID string `json:"input_id"`
}

func Default() Settings { return Settings{GainDB:20,ThresholdDB:-50} }
func (s Settings) Params() dsp.Params { return dsp.Params{GainDB:s.GainDB,Gate:s.Gate,ThresholdDB:s.ThresholdDB}.Normalized() }
func Path() (string,error) {
	dir,err:=os.UserConfigDir(); if err!=nil {return "",err}
	return filepath.Join(dir,"GameMic","settings.json"),nil
}
func Load(path string) (Settings,error) {
	s:=Default()
	b,err:=os.ReadFile(path)
	if errors.Is(err,os.ErrNotExist) {return s,nil}
	if err!=nil {return s,err}
	if err=json.Unmarshal(b,&s);err!=nil {return Default(),err}
	p:=s.Params();s.GainDB=p.GainDB;s.ThresholdDB=p.ThresholdDB
	return s,nil
}
func Save(path string,s Settings) error {
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
	b,err:=json.MarshalIndent(s,"","  ");if err!=nil{return err}
	f,err:=os.CreateTemp(filepath.Dir(path),"settings-*.tmp");if err!=nil{return err}
	tmp:=f.Name();defer os.Remove(tmp)
	if _,err=f.Write(b);err!=nil {f.Close();return err}
	if err=f.Sync();err!=nil {f.Close();return err}
	if err=f.Close();err!=nil{return err}
	return os.Rename(tmp,path)
}
