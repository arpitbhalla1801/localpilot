package cli

import (
	"strconv"
	"testing"

	"github.com/arpitbhalla1801/localpilot/internal/testutil"
)

// TestListenHelperProcess is not a real test. It is re-exec'd as a
// subprocess (the standard os/exec "helper process" pattern) so other
// tests can exercise commands against a real, killable process listening
// on a real port, instead of the test binary itself.
func TestListenHelperProcess(t *testing.T) { testutil.ListenHelperMain() }

func itoa(n int) string { return strconv.Itoa(n) }

func freePort(t *testing.T) int { return testutil.FreePort(t) }

type listenerHelper = testutil.ListenerHelper

func startListenerHelper(t *testing.T, port int) *listenerHelper {
	return testutil.StartListenerHelper(t, port)
}
