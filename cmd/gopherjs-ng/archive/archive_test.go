package archive

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestArchive(t *testing.T) {
	a := Archive{
		Entries: []*Entry{
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
			wantEntry *Entry
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
				wantEntry: nil,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				i, e := a.Get(test.name)
				if i != test.wantIdx {
					t.Errorf("a.Get(%q) returned index %d, want %d", test.name, i, test.wantIdx)
				}
				if e != test.wantEntry {
					t.Errorf("a.Get(%q) returned entry %p, want %p", test.name, e, test.wantEntry)
				}
			})
		}
	})
}
