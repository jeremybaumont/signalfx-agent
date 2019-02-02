// +build windows

package cpu

import (
	"reflect"
	"testing"

	"github.com/shirou/gopsutil/cpu"
)

func Test_getTimes(t *testing.T) {
	type args struct {
		perCore bool
	}
	tests := []struct {
		name    string
		args    args
		want    []cpu.TimesStat
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				perCore: true,
			},
			want:    []cpu.TimesStat{cpu.TimesStat{Idle: 100000}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTimes(tt.args.perCore)
			if (err != nil) != tt.wantErr {
				t.Errorf("getTimes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTimes() = %v, want %v", got, tt.want)
			}
		})
	}
}
