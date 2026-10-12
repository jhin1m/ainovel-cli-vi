package lang

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]Code{"": VI, "vi": VI, " VI ": VI, "vietnamese": VI, "zh": ZH, "Chinese": ZH}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Fatalf("Parse(%q) = %q, %v; muốn %q", in, got, err, want)
		}
	}
	if _, err := Parse("en"); err == nil {
		t.Fatal("Parse(en) phải báo lỗi")
	}
}

func TestSetCurrent(t *testing.T) {
	t.Cleanup(func() { Set(Default) })
	if Current() != VI || !IsVI() {
		t.Fatalf("mặc định phải là vi, có %q", Current())
	}
	Set(ZH)
	if Current() != ZH || IsVI() {
		t.Fatalf("sau Set(ZH) có %q", Current())
	}
}
