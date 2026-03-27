package phloem

import "testing"

func TestProjectMachineIDStable(t *testing.T) {
	a := ProjectMachineID("/tmp/foo/bar")
	b := ProjectMachineID("/tmp/foo/bar")
	if a != b {
		t.Fatalf("same path: %d vs %d", a, b)
	}
	if a == 0 {
		t.Fatal("expected non-zero id")
	}
	if ProjectMachineID("/other") == a {
		t.Fatal("different paths should not collide trivially")
	}
}

func TestProjectMachineIDEmpty(t *testing.T) {
	if v := ProjectMachineID(""); v != 0 {
		t.Fatalf("empty: got %d", v)
	}
	if v := ProjectMachineID("   "); v != 0 {
		t.Fatalf("whitespace: got %d", v)
	}
}
