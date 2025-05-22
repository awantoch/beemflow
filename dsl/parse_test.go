package dsl

import (
	"testing"

	"github.com/stretchr/testify/require"

	pproto "github.com/awantoch/beemflow/spec/proto"
)

func TestParse_DefaultTrigger(t *testing.T) {
	flow, err := ParseFromString("steps: []")
	require.NoError(t, err)
	_, ok := flow.On.Kind.(*pproto.Trigger_CliManual)
	require.True(t, ok, "expected default CLI manual trigger")
}

func TestParse_CLIManualTrigger(t *testing.T) {
	yamlStr := `
on: "cli.manual"
steps: []
`
	flow, err := ParseFromString(yamlStr)
	require.NoError(t, err)
	_, ok := flow.On.Kind.(*pproto.Trigger_CliManual)
	require.True(t, ok, "expected CLI manual trigger")
}

func TestParse_MCPEventTrigger(t *testing.T) {
	yamlStr := `
on:
  - event: "foo"
steps: []
`
	flow, err := ParseFromString(yamlStr)
	require.NoError(t, err)
	evt := flow.On.GetMcpEvent()
	require.NotNil(t, evt)
	require.Equal(t, "foo", evt.GetSource())
}

func TestParse_ExecShorthand(t *testing.T) {
	yamlStr := `
steps:
  - id: mystep
    use: core.echo
    with:
      text: "hello"
`
	flow, err := ParseFromString(yamlStr)
	require.NoError(t, err)
	require.Len(t, flow.Steps, 1)
	exec := flow.Steps[0].GetExec()
	require.NotNil(t, exec, "expected exec behavior")
	require.Equal(t, "core.echo", exec.GetUse())
	val := exec.GetWith()["text"]
	require.NotNil(t, val)
	require.Equal(t, "hello", val.GetS())
}

func TestParse_ParallelShorthand(t *testing.T) {
	yamlStr := `
steps:
  - id: block
    parallel: true
    steps:
      - id: sub
        use: core.echo
        with:
          text: "ok"
`
	flow, err := ParseFromString(yamlStr)
	require.NoError(t, err)
	require.Len(t, flow.Steps, 1)
	par := flow.Steps[0].GetParallel()
	require.NotNil(t, par, "expected parallel behavior")
	require.Len(t, par.GetSteps(), 1)
	sub := par.GetSteps()[0]
	require.Equal(t, "sub", sub.GetId())
}
