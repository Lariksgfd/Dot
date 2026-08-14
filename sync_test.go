package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThreadChannels(t *testing.T) {
	tests := []struct {
		name     string
		dotCode  string
		expected string
	}{
		{
			name: "HappyPath",
			dotCode: `import "../stdlib/sync.dot"
fn main() {
	ch = create_channel[int]()
	ch.send(42)
	val, ok = ch.try_recv()
	if ok { print(val) } else { print("fail") }
	ch.close()
}
`,
			expected: "42",
		},
		{
			name: "EmptyQueue",
			dotCode: `import "../stdlib/sync.dot"
fn main() {
	ch = create_channel[int]()
	val, ok = ch.try_recv()
	if not ok { print("empty") } else { print("fail") }
	ch.close()
}
`,
			expected: "empty",
		},
		{
			name: "FIFOOrder",
			dotCode: `import "../stdlib/sync.dot"
fn main() {
	ch = create_channel[int]()
	ch.send(1)
	ch.send(2)
	ch.send(3)
	v1, _ = ch.try_recv()
	v2, _ = ch.try_recv()
	v3, _ = ch.try_recv()
	print(v1)
	print(v2)
	print(v3)
	ch.close()
}
`,
			expected: "1",
		},
		{
			name: "ABA_Scenario",
			dotCode: `import "../stdlib/sync.dot"
fn main() {
	ch = create_channel[int]()
	ch.send(10)
	ch.try_recv()
	ch.send(20)
	ch.send(30)
	v, _ = ch.try_recv()
	print(v)
	ch.close()
}
`,
			expected: "20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join("testdata", "tmp_sync_test_"+tt.name+".dot")
			os.WriteFile(tmpFile, []byte(tt.dotCode), 0644)
			defer os.Remove(tmpFile)

			out, err := buildAndRun(t, tmpFile, false)
			if err != nil {
				t.Fatalf("build+run failed: %v\noutput: %s", err, out)
			}
			if !contains(out, tt.expected) {
				t.Errorf("expected %q in output, got: %q", tt.expected, out)
			}
		})
	}
}
