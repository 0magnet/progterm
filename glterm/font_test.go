package glterm

import (
	"os/exec"
	"testing"
)

func TestParseFont(t *testing.T) {
	for in, want := range map[string]struct {
		family string
		pt     float64
	}{
		"DejaVu Sans Mono 10":    {"DejaVu Sans Mono", 10},
		"'Adwaita Mono 11'":      {"Adwaita Mono", 11},
		"Monospace":              {"Monospace", DefaultPt},
		"Iosevka Term 10.5":      {"Iosevka Term", 10.5},
		"  Fira Code  ":          {"Fira Code", DefaultPt},
		"Source Code Pro Bold 9": {"Source Code Pro Bold", 9},
	} {
		f, pt := ParseFont(in)
		if f != want.family || pt != want.pt {
			t.Errorf("%q: %q %v, want %q %v", in, f, pt, want.family, want.pt)
		}
	}
}

func TestLoadFontFallsBack(t *testing.T) {
	f, err := LoadFont("No Such Font Anywhere 12")
	if _, lookErr := exec.LookPath("fc-match"); lookErr == nil && err == nil {
		t.Fatal("a missing font loaded without an error")
	}
	if f.Family != "Go Mono" || f.Pt != 12 {
		t.Fatalf("fell back to %s", f)
	}
	if _, err := NewGridFont(f, 16); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFontFromFontconfig(t *testing.T) {
	if out, ok := run("fc-match", "-f", "%{family}", "DejaVu Sans Mono"); !ok || out != "DejaVu Sans Mono" {
		t.Skip("no DejaVu Sans Mono here")
	}
	f, err := LoadFont("DejaVu Sans Mono 10")
	if err != nil {
		t.Fatal(err)
	}
	if f.Family != "DejaVu Sans Mono" || f.Pt != 10 {
		t.Fatalf("loaded %s", f)
	}
	if f.styles[1] == f.styles[0] || f.styles[2] == f.styles[0] || f.styles[3] == f.styles[0] {
		t.Fatal("a style of DejaVu Sans Mono fell back to the regular")
	}
	g, err := NewGridFont(f, 10*96.0/72)
	if err != nil {
		t.Fatal(err)
	}
	if g.CellW < 7 || g.CellW > 9 || g.CellH < 14 || g.CellH > 18 {
		t.Fatalf("a 10 pt cell is %dx%d", g.CellW, g.CellH)
	}
}
