package firewall_rule

import (
	"testing"

	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"
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
