package archive

import (
	"bytes"
	"go/token"
	"go/types"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gopherjs/gopherjs/compiler"
)

func TestArchive(t *testing.T) {
	a := Archive{
		Entries: []Entry{
			{
				Name:  "no_padding",
				MTime: 42,
				UID:   1,
				GID:   2,
				Mode:  0600,
				Data:  []byte("AAAABBBBCCCCDDDDEEEEFFFF"),
			}, {
				Name: "padded_file",
				Data: []byte("123456789"), // Odd length, must be padded.
			},
		},
	}

	serialized := []byte("!<arch>\n" +
		"no_padding      42          1     2     600     24        `\n" +
		"AAAABBBBCCCCDDDDEEEEFFFF" +
		"padded_file     0           0     0     0       9         `\n" +
		"123456789\x00")

	t.Run("write", func(t *testing.T) {
		buf := &bytes.Buffer{}
		if err := a.Write(buf); err != nil {
			t.Fatalf("a.Write() returned error: %s", err)
		}
		if diff := cmp.Diff(serialized, buf.Bytes()); diff != "" {
			t.Errorf("a.Write() returned diff (-want,+got):\n%s", diff)
		}
	})

	t.Run("read", func(t *testing.T) {
		buf := bytes.NewBuffer(serialized)
		got, err := Load(buf)
		if err != nil {
			t.Fatalf("Load() returned error: %s", err)
		}
		if diff := cmp.Diff(a, got); diff != "" {
			t.Errorf("Load() returned diff (-want,+got):\n%s", diff)
		}
	})

	t.Run("get", func(t *testing.T) {
		tests := []struct {
			name      string
			wantIdx   int
			wantEntry Entry
		}{
			{
				name:      "no_padding",
				wantIdx:   0,
				wantEntry: a.Entries[0],
			}, {
				name:      "padded_file",
				wantIdx:   1,
				wantEntry: a.Entries[1],
			}, {
				name:      "not_found",
				wantIdx:   -1,
				wantEntry: Entry{},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				i, e := a.Get(test.name)
				if i != test.wantIdx {
					t.Errorf("a.Get(%q) returned index %d, want %d", test.name, i, test.wantIdx)
				}
				if !cmp.Equal(e, test.wantEntry) {
					t.Errorf("a.Get(%q) returned entry %+v, want %+v", test.name, e, test.wantEntry)
				}
			})
		}
	})
}

func TestPkgDef(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		p := NewPkgDef("abc/def", types.NewPackage("some/pkg", "main"), token.NewFileSet())
		buf := &bytes.Buffer{}
		if err := p.Write(buf); err != nil {
			t.Fatalf("p.Write() returned error: %s", err)
		}

		b := buf.Bytes()
		pos := bytes.Index(b, []byte("\n\n"))
		if pos == -1 {
			t.Errorf("Failed to find pkgdef header end")
		}
		pos += 2
		header := buf.String()[0:pos]

		want := "go object js js " + compiler.Version + " X:none\n" +
			"build id \"abc/def\"\n" +
			"main\n\n"
		if diff := cmp.Diff(want, header); diff != "" {
			t.Errorf("p.Write() produced diff (-want,+got):\n%s", diff)
		}
	})
}
