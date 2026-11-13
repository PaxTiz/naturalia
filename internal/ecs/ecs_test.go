package ecs

import (
	"slices"
	"testing"
)

type pos struct{ x, y float64 }

func TestLifecycle(t *testing.T) {
	w := NewWorld()
	p := NewStore[pos](w)
	names := NewStore[string](w)
	a, b, c := w.New(), w.New(), w.New()
	if a == 0 || a == b || w.Len() != 3 {
		t.Fatalf("ids %v %v %v, %d alive", a, b, c, w.Len())
	}
	p.Set(a, pos{1, 0})
	p.Set(b, pos{2, 0})
	p.Set(c, pos{3, 0})
	names.Set(b, "b")
	p.Get(b).x = 20

	w.Kill(b)
	if w.Alive(b) || p.Has(b) || names.Has(b) || p.Len() != 2 || names.Len() != 0 {
		t.Fatal("b's components survived it")
	}
	if p.Get(a).x != 1 || p.Get(c).x != 3 {
		t.Fatal("the others moved")
	}
	// Its slot comes back under a new generation: the old ID stays dead.
	d := w.New()
	if d.Index() != b.Index() || d == b || w.Alive(b) || p.Has(d) {
		t.Fatalf("reused %v as %v", b, d)
	}
	p.Set(d, pos{4, 0})
	if p.Get(b) != nil {
		t.Fatal("the old ID reads the new entity")
	}
	w.Kill(b) // dead already: nothing
	if !w.Alive(d) || w.Len() != 3 {
		t.Fatal("killing a dead ID killed its successor")
	}
	if got := slices.Collect(w.All()); !slices.Equal(got, []ID{a, d, c}) {
		t.Errorf("world order %v", got)
	}
	if w.Alive(0) || p.Get(0) != nil {
		t.Error("0 is an entity")
	}
}

func TestRemoveWhileIterating(t *testing.T) {
	w := NewWorld()
	p := NewStore[pos](w)
	var ids []ID
	for i := range 10 {
		id := w.New()
		p.Set(id, pos{float64(i), 0})
		ids = append(ids, id)
	}
	seen := 0
	for id, v := range p.All() {
		seen++
		if int(v.x)%2 == 0 {
			w.Kill(id)
		}
	}
	if seen != 10 || p.Len() != 5 || w.Len() != 5 {
		t.Fatalf("saw %d, %d left", seen, p.Len())
	}
	for i, id := range ids {
		if v := p.Get(id); (v != nil) != (i%2 == 1) || (v != nil && int(v.x) != i) {
			t.Errorf("entity %d: %v", i, v)
		}
	}
}

func BenchmarkIterate(b *testing.B) {
	w := NewWorld()
	p := NewStore[pos](w)
	for range 10000 {
		p.Set(w.New(), pos{1, 1})
	}
	b.ResetTimer()
	for range b.N {
		for _, v := range p.All() {
			v.x += v.y
		}
	}
}

func BenchmarkSpawnKill(b *testing.B) {
	w := NewWorld()
	p := NewStore[pos](w)
	for range b.N {
		id := w.New()
		p.Set(id, pos{})
		w.Kill(id)
	}
}
