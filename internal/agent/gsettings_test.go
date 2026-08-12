package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	outputs map[string]string
	calls   []runnerCall
	err     error
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.outputs[args[len(args)-1]]), nil
}

func TestGSettingsBackendRoundTripsAllSchemas(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"org.gnome.system.proxy":      "org.gnome.system.proxy mode 'manual'\n",
		"org.gnome.system.proxy.http": "org.gnome.system.proxy.http host 'old.example'\norg.gnome.system.proxy.http port uint32 8080\n",
	}}
	backend := &GSettingsBackend{runner: runner}
	state, err := backend.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Apply(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	var got [][]string
	for _, call := range runner.calls {
		if len(call.args) > 0 && call.args[0] == "set" {
			got = append(got, call.args)
		}
	}
	want := [][]string{
		{"set", "org.gnome.system.proxy", "mode", "'manual'"},
		{"set", "org.gnome.system.proxy.http", "host", "'old.example'"},
		{"set", "org.gnome.system.proxy.http", "port", "uint32 8080"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sets = %#v, want %#v", got, want)
	}
}

func TestGSettingsBackendRejectsMalformedState(t *testing.T) {
	runner := &fakeRunner{err: errors.New("must not run")}
	backend := &GSettingsBackend{runner: runner}
	if err := backend.Apply(context.Background(), ProxyState(`{"version":1,"values":[{"schema":"other","key":"x","value":"true"}]}`)); err == nil {
		t.Fatal("expected invalid schema error")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %#v", runner.calls)
	}
}
