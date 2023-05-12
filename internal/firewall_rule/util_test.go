package firewall_rule

import (
	"reflect"
	"testing"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
)

func Test_cidrListToString(t *testing.T) {
	type args struct {
		ruleObjects []swagger.FirewallRuleObject
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty list",
			args: args{},
			want: "",
		},
		{
			name: "single CIDR",
			args: args{ruleObjects: []swagger.FirewallRuleObject{{Cidr: "127.0.0.0/16"}}},
			want: "127.0.0.0/16",
		},
		{
			name: "multiple CIDRs",
			args: args{ruleObjects: []swagger.FirewallRuleObject{{Cidr: "127.0.0.0/16"}, {Cidr: "127.0.0.1/24"}}},
			want: "127.0.0.0/16,127.0.0.1/24",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cidrListToString(tt.args.ruleObjects); got != tt.want {
				t.Errorf("cidrListToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stringToSlice(t *testing.T) {
	type args struct {
		s         string
		delimiter string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty string",
			args: args{s: "", delimiter: ","},
			want: []string{},
		},
		{
			name: "single element",
			args: args{s: "asd", delimiter: ","},
			want: []string{"asd"},
		},
		{
			name: "multiple elements, no space",
			args: args{s: "asd,dsa", delimiter: ","},
			want: []string{"asd", "dsa"},
		},
		{
			name: "multiple elements, some spaces",
			args: args{s: "asd,dsa, qwe", delimiter: ","},
			want: []string{"asd", "dsa", "qwe"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringToSlice(tt.args.s, tt.args.delimiter); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stringToSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
