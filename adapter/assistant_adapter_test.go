package adapter

import (
	"context"
	"os"
	"testing"
)

func TestExecute_Validation(t *testing.T) {
	tests := []struct {
		name     string
		messages []string
		llmOut   string
		wantErr  bool
		wantVal  bool
	}{
		{
			name:     "valid flow",
			messages: []string{"Draft a flow that echoes hello"},
			llmOut:   "name: test\non: cli.manual\nsteps:\n  - id: s1\n    use: core.echo\n    with:\n      text: hello\n",
			wantErr:  false,
			wantVal:  false,
		},
		{
			name:     "invalid YAML",
			messages: []string{"Draft a flow with bad YAML"},
			llmOut:   "name: test\nsteps: [bad yaml",
			wantErr:  false,
			wantVal:  true,
		},
		{
			name:     "schema error",
			messages: []string{"Draft a flow missing steps"},
			llmOut:   "name: test\non: cli.manual\n",
			wantErr:  false,
			wantVal:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Patch CallLLM to return tt.llmOut
			orig := CallLLM
			CallLLM = func(ctx context.Context, systemPrompt string, userMessages []string) (string, error) {
				return tt.llmOut, nil
			}
			defer func() { CallLLM = orig }()
			draft, valErrs, err := Execute(context.Background(), tt.messages)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if (len(valErrs) > 0) != tt.wantVal {
				t.Errorf("got valErrs=%v, wantVal=%v", valErrs, tt.wantVal)
			}
			if draft != tt.llmOut {
				t.Errorf("got draft=%q, want=%q", draft, tt.llmOut)
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Setenv("BEEMFLOW_SCHEMA", "../beemflow.schema.json")
	os.Exit(m.Run())
}
