// Package engine regroupe ce qui ne connaît pas le jeu : maths, ECS,
// physique, maillages. Le jeu s'appuie sur le moteur, jamais l'inverse.
package engine

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	module = "naturalia/"
	engine = "naturalia/internal/engine"
)

// TestFrontier refuse qu'un fichier du moteur, tests compris, importe un
// paquet du module qui n'est pas lui-même dans le moteur.
func TestFrontier(t *testing.T) {
	seen := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		seen++
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(p, module) && p != engine && !strings.HasPrefix(p, engine+"/") {
				t.Errorf("%s importe %s : le moteur ne dépend pas du jeu", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 10 {
		t.Fatalf("%d fichiers lus seulement : le test ne voit pas le moteur", seen)
	}
}
