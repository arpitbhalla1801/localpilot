package cli

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/arpitbhalla1801/localpilot/internal/testutil"
)

func TestDiagnose_Open_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	res := diagnose(context.Background(), "127.0.0.1", port, time.Second, true)
	if res.Status != "open" || res.HTTPStatus != http.StatusTeapot {
		t.Fatalf("got %+v, want open + 418", res)
	}
}

func TestDiagnose_Refused(t *testing.T) {
	res := diagnose(context.Background(), "127.0.0.1", testutil.FreePort(t), time.Second, false)
	if res.Status != "refused" {
		t.Fatalf("got %+v, want refused", res)
	}
}

func TestDiagnoseCmd_InvalidTarget(t *testing.T) {
	for _, target := range []string{"localhost", "localhost:0", "localhost:abc"} {
		rootCmd.SetArgs([]string{"diagnose", target})
		if err := rootCmd.Execute(); err == nil {
			t.Errorf("diagnose %q should error", target)
		}
	}
}

func TestDiagnoseCmd_UnreachableExitsNonZero(t *testing.T) {
	rootCmd.SetArgs([]string{"diagnose", "127.0.0.1:" + strconv.Itoa(testutil.FreePort(t)), "--json"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("unreachable target should return an error (non-zero exit)")
	}
}
