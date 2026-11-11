package content

import "testing"

func TestRegistryOverrideKeepsOrder(t *testing.T) {
	var r Registry[int]
	r.init("test")
	r.pack = "core"
	r.Add("core:a", 1)
	r.Add("core:b", 2)
	r.Add("bad", 3)
	r.pack = "mod"
	r.Add("core:a", 10)
	r.Patch("core:b", func(v *int) { *v *= 5 })
	if got := r.IDs(); len(got) != 2 || got[0] != "core:a" {
		t.Fatalf("ids = %v", got)
	}
	if a, _ := r.Get("core:a"); *a != 10 {
		t.Errorf("override: %d", *a)
	}
	if b, _ := r.Get("core:b"); *b != 10 {
		t.Errorf("patch: %d", *b)
	}
}
