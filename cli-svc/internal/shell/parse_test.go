package shell

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantVerb string
		wantRes  string
		wantArgs map[string]string
		wantErr  bool
	}{
		{"full", "get market latest market gold", "get", "market.latest", map[string]string{"market": "gold"}, false},
		{"no args", "get monitoring overview", "get", "monitoring.overview", map[string]string{}, false},
		{"quoted value with spaces", `update schedule update key crawler_gold cron_expression "0 0 2 * * *" enabled true`,
			"update", "schedule.update",
			map[string]string{"key": "crawler_gold", "cron_expression": "0 0 2 * * *", "enabled": "true"}, false},
		{"category only (incomplete)", "get market", "get", "market", map[string]string{}, false},
		{"empty", "", "", "", nil, true},
		{"missing arg value", "get market latest market", "", "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if p.Verb != tt.wantVerb || p.Resource != tt.wantRes {
				t.Errorf("verb=%q res=%q want verb=%q res=%q", p.Verb, p.Resource, tt.wantVerb, tt.wantRes)
			}
			if len(p.Args) != len(tt.wantArgs) {
				t.Errorf("args=%v want %v", p.Args, tt.wantArgs)
			}
			for k, v := range tt.wantArgs {
				if p.Args[k] != v {
					t.Errorf("arg %s=%q want %q", k, p.Args[k], v)
				}
			}
		})
	}
}
