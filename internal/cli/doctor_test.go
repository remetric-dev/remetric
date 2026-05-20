package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

func newPromStub(t *testing.T, version string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"` + version + `","revision":"x","goVersion":"go1.22.0","buildUser":"","buildDate":""}}`))
	})
	mux.HandleFunc("/api/v1/status/tsdb", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"headStats":{"numSeries":100,"numLabelPairs":0,"chunkCount":0,"minTime":0,"maxTime":0},"seriesCountByMetricName":[],"labelValueCountByLabelName":[],"memoryInBytesByLabelName":[],"seriesCountByLabelValuePair":[]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDoctor_Happy(t *testing.T) {
	srv := newPromStub(t, "2.51.2")
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", srv.URL, "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0\nout:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "[OK]") {
		t.Errorf("doctor output missing [OK]:\n%s", out.String())
	}
}

func TestDoctor_OldVersion(t *testing.T) {
	srv := newPromStub(t, "2.29.1")
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", srv.URL, "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "below minimum") {
		t.Errorf("output missing minimum-version message:\n%s", out.String())
	}
}

func TestDoctor_Unreachable(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", "http://127.0.0.1:1", "--no-color", "--timeout", "2s"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "[FAIL]") {
		t.Errorf("output missing [FAIL]:\n%s", out.String())
	}
}

func TestDoctor_MissingURL(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2 (invalid flags)", code)
	}
}
