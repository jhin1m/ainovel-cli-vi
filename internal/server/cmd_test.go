package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommandHelpNeedsNoConfig(t *testing.T) {
	t.Setenv("AINOVEL_DATABASE_URL_FILE", "")
	t.Setenv("AINOVEL_MASTER_KEY_FILE", "")
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		arg := strings.Join(args, " ")
		var out, errOut bytes.Buffer
		if code := Command(args, "test", &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", arg, code, errOut.String())
		}
		if !strings.Contains(out.String(), "ainovel-cli serve") {
			t.Fatalf("%s: thiếu hướng dẫn: %q", arg, out.String())
		}
	}
}

// Lệnh con lạ bị từ chối với exit 2, không cần cấu hình.
func TestCommandRejectsSubcommands(t *testing.T) {
	t.Setenv("AINOVEL_DATABASE_URL_FILE", "")
	t.Setenv("AINOVEL_MASTER_KEY_FILE", "")
	for _, args := range [][]string{{"foo"}, {"admin", "create"}, {"admin", "create", "--help"}} {
		arg := strings.Join(args, " ")
		var out, errOut bytes.Buffer
		if code := Command(args, "test", &out, &errOut); code != 2 {
			t.Fatalf("%s: exit %d, muốn 2", arg, code)
		}
		if !strings.Contains(errOut.String(), "không được hỗ trợ") {
			t.Fatalf("%s: thiếu thông báo lệnh con: %q", arg, errOut.String())
		}
	}
}
