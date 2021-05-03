package goroot

import (
	"go/token"
	"testing"

	"github.com/kylelemons/godebug/diff"
)

func TestNosync(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		wantSrc      string
		wantModified bool
	}{
		{
			name: "unnamed import",
			src: `package x
				import (
					"foo/bar"
					"sync"
				)`,
			wantSrc: `package x
				import (
					"foo/bar"
					sync "github.com/gopherjs/gopherjs/nosync"
				)`,
			wantModified: true,
		}, {
			name: "named import",
			src: `package x
				import (
					"foo/bar"
					stdsync "sync"
				)`,
			wantSrc: `package x
				import (
					"foo/bar"
					stdsync "github.com/gopherjs/gopherjs/nosync"
				)`,
			wantModified: true,
		}, {
			name: "not imported",
			src: `package x
				import (
					"foo/bar"
					sync "other/sync"
				)`,
			wantSrc: `package x
				import (
					"foo/bar"
					sync "other/sync"
				)`,
			wantModified: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f := parse(t, fset, test.src)
			modified := nosync(fset, f)

			if modified != test.wantModified {
				t.Errorf("nosync() returned %t, want %t", modified, test.wantModified)
			}

			got := reconstruct(t, fset, f)
			want := gofmt(t, test.wantSrc)

			if diff := diff.Diff(want, got); diff != "" {
				t.Errorf("nosync() produced diff (-want,+got):\n%s", diff)
			}
		})
	}
}
