package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandHelpNeedsNoConfig(t *testing.T) {
	t.Setenv("AINOVEL_DATABASE_URL_FILE", "")
	t.Setenv("AINOVEL_MASTER_KEY_FILE", "")
	for _, arg := range []string{"--help", "-h", "help"} {
		var out, errOut bytes.Buffer
		if code := Command([]string{arg}, "test", &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", arg, code, errOut.String())
		}
		if !strings.Contains(out.String(), "invite create") {
			t.Fatalf("%s: thiếu hướng dẫn: %q", arg, out.String())
		}
	}
}
