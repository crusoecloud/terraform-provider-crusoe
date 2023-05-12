package vm

import (
	"reflect"
	"testing"
)

func Test_getDisksDiff(t *testing.T) {
	type args struct {
		orig []vmDiskResourceModel
		new  []vmDiskResourceModel
	}
	tests := []struct {
		name             string
		args             args
		wantDisksAdded   []string
		wantDisksRemoved []string
	}{
		{
			name: "all match",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234"}},
				new:  []vmDiskResourceModel{{ID: "1234"}},
			},
			wantDisksAdded:   nil,
			wantDisksRemoved: nil,
		},
		{
			name: "disk added",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234"}},
				new:  []vmDiskResourceModel{{ID: "1234"}, {ID: "2345"}},
			},
			wantDisksAdded:   []string{"2345"},
			wantDisksRemoved: nil,
		},
		{
			name: "disk removed",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234"}, {ID: "2345"}},
				new:  []vmDiskResourceModel{{ID: "2345"}},
			},
			wantDisksAdded:   nil,
			wantDisksRemoved: []string{"1234"},
		},
		{
			name: "disk added and removed",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234"}},
				new:  []vmDiskResourceModel{{ID: "2345"}},
			},
			wantDisksAdded:   []string{"2345"},
			wantDisksRemoved: []string{"1234"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDisksAdded, gotDisksRemoved := getDisksDiff(tt.args.orig, tt.args.new)
			if !reflect.DeepEqual(gotDisksAdded, tt.wantDisksAdded) {
				t.Errorf("getDisksDiff() gotDisksAdded = %v, want %v", gotDisksAdded, tt.wantDisksAdded)
			}
			if !reflect.DeepEqual(gotDisksRemoved, tt.wantDisksRemoved) {
				t.Errorf("getDisksDiff() gotDisksRemoved = %v, want %v", gotDisksRemoved, tt.wantDisksRemoved)
			}
		})
	}
}
