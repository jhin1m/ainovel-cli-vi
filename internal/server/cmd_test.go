package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandHelpNeedsNoConfig(t *testing.T) {
	t.Setenv("AINOVEL_DATABASE_URL_FILE", "")
	t.Setenv("AINOVEL_MASTER_KEY_FILE", "")
	for _, args := range [][]string{
		{"--help"}, {"-h"}, {"help"},
		{"invite", "--help"}, {"invite", "create", "--help"}, {"invite", "create", "-h"},
	} {
		arg := strings.Join(args, " ")
		var out, errOut bytes.Buffer
		if code := Command(args, "test", &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", arg, code, errOut.String())
		}
		if !strings.Contains(out.String(), "invite create") {
			t.Fatalf("%s: thiếu hướng dẫn: %q", arg, out.String())
		}
	}
}
