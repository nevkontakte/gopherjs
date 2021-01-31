// Package archive implements reading and writing .a files.
//
// The format is similar to what Go toolchain uses, including the container and
// metadata format. However, the object files are stored in GopherJS-specific
// format, rather than ELF or Go object file.
//
// The code in this package is modelled against cmd/internal/archive, with
// GopherJS-specific customizations.
package archive

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gopherjs/gopherjs/compiler"
	"golang.org/x/tools/go/gcexportdata"
)

const (
	// PkgDefName is an archive entry name with package meta information for the compiler.
	PkgDefName = "__.PKGDEF"

	archiveHeader = "!<arch>\n"
	sectionHdrEnd = "`\n"
	// Archive section header format:
	//                   name mtime uid gid mode size
	sectionHdrWrite  = "%-16s%-12d%-6d%-6d%-8o%-10d" + sectionHdrEnd
	sectionHdrParse  = "%12d%6d%6d%8o%10d"
	sectionHdrLength = 60
	sectionAlignment = 2
)

var (
	errEndOfArchive = errors.New("end of archive")
)

// Entry represents a single file in the archive.
type Entry struct {
	Name  string
	MTime int64
	UID   int
	GID   int
	Mode  os.FileMode
	Data  []byte
}

// NewEntry returns an archive entry with metadata set to match Go toolchain's
// defaults.
func NewEntry(name string, data []byte) Entry {
	return Entry{
		Name: name,
		Mode: 0644,
		Data: data,
	}
}

func (e *Entry) write(w io.Writer) error {
	if len(e.Name) > 16 {
		return fmt.Errorf("entry name %q is too long", e.Name)
	}
	if _, err := fmt.Fprintf(w, sectionHdrWrite, e.Name, e.MTime, e.UID, e.GID, e.Mode, len(e.Data)); err != nil {
		return fmt.Errorf("failed to write entry header: %w", err)
	}
	if _, err := w.Write(e.Data); err != nil {
		return fmt.Errorf("failed to write entry body: %w", err)
	}
	if len(e.Data)%sectionAlignment == 1 {
		// Pad data to even byte boundary.
		if _, err := w.Write([]byte{0}); err != nil {
			return fmt.Errorf("failed to write padding: %w", err)
		}
	}
	return nil
}

// parse archive section.
//
// Reader must be pointing at the beginning of the section header. If successful,
// reader will be left pointing at the byte after the last section data and
// padding byte. If a non-nil error is returned, the Entry struct is left in an
// undefined state.
func (e *Entry) parse(r io.Reader) error {
	// Unfortunately, fmt.Fscanf() doesn't support parsing left-aligned fields, so
	// we have to read and parse the header a bit more manually.

	// Read fixed-size section header.
	var hdr [sectionHdrLength]byte
	if _, err := io.ReadFull(r, hdr[:]); errors.Is(err, io.EOF) {
		// We've reached the end of the archive, not a problem.
		return errEndOfArchive
	} else if err != nil {
		return fmt.Errorf("failed to read section header: %w", err)
	}

	// Parse the header.
	if magic := string(hdr[sectionHdrLength-2 : sectionHdrLength]); magic != sectionHdrEnd {
		return fmt.Errorf("invalid section header signature %+q, expected %+q", magic, sectionHdrEnd)
	}
	e.Name = strings.TrimRight(string(hdr[0:16]), " ")
	var size int
	// We can at least parse the numeric fields.
	if _, err := fmt.Sscanf(string(hdr[16:sectionHdrLength-2]), sectionHdrParse,
		&e.MTime, &e.UID, &e.GID, &e.Mode, &size); err != nil {
		return fmt.Errorf("failed to parse section header: %w", err)
	}

	// Read entry body.
	e.Data = make([]byte, size)
	if _, err := io.ReadFull(r, e.Data); err != nil {
		return fmt.Errorf("failed to read section body: %s", err)
	}

	// Skip padding bytes if any. EOF at this stage is generally not a problem.
	if _, err := io.CopyN(ioutil.Discard, r, int64(size%sectionAlignment)); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to skip section padding: %w", err)
	}
	return nil
}

// Archive provides read and write access to .a archives.
//
// File format is based on Unix ar format (https://en.wikipedia.org/wiki/Ar_(Unix)):
// The file starts with a magic sequence "!<arch>\n", which is followed by zero
// or more sections.
//
// Each section begins with a fixed-size header formatted according to
// sectionFormat, then followed by file data padded with "\n" to even byte count
// if necessary.
type Archive struct {
	Entries []Entry
}

// Load .a archive into memory.
//
// Returns Archive instance with entries populated with the data from the reader.
// In order to succeed, theme must be enough RAM available to store entire
// contents of the archive.
func Load(r io.Reader) (Archive, error) {
	a := Archive{}
	if _, err := fmt.Fscanf(r, archiveHeader); err != nil {
		return a, fmt.Errorf("invalid archive signature, expected %q: %w", archiveHeader, err)
	}
	for {
		e := Entry{}
		if err := e.parse(r); errors.Is(err, errEndOfArchive) {
			break
		} else if err != nil {
			return a, fmt.Errorf("failed to parse archive section: %w", err)
		}
		a.Entries = append(a.Entries, e)
	}
	return a, nil
}

// Write archive contents into a file/buffer accorting to the ar format.
//
// Passed writer is expected to point at the beginning of an empty file or buffer,
// it doesn't perform seeks or attempts to truncate the underlying file.
func (a *Archive) Write(w io.Writer) error {
	if _, err := io.WriteString(w, archiveHeader); err != nil {
		return fmt.Errorf("failed to write archive header: %w", err)
	}
	for i, e := range a.Entries {
		if err := e.write(w); err != nil {
			return fmt.Errorf("failed to write archive entry #%d %q: %w", i, e.Name, err)
		}
	}
	return nil
}

// Get returns index and pointer to an archive entry with the given name, or -1
// and Entry{} if such an entry is not present.
func (a *Archive) Get(name string) (int, Entry) {
	for i, e := range a.Entries {
		if e.Name == name {
			return i, e
		}
	}
	return -1, Entry{}
}

// PkgDef represents Go package metadata entry in the archive.
type PkgDef struct {
	GOOS        string
	GOARCH      string
	Version     string
	Experiments string
	BuildID     string
	Package     *types.Package
	FSet        *token.FileSet
}

// NewPkgDef returns PkgDef instance configures according to GopherJS needs.
func NewPkgDef(buildID string, pkg *types.Package, fset *token.FileSet) *PkgDef {
	return &PkgDef{
		GOOS:        "js",
		GOARCH:      "js",
		Version:     compiler.Version,
		Experiments: "X:none",
		BuildID:     buildID,
		Package:     pkg,
		FSet:        fset,
	}
}

func (p *PkgDef) Write(w io.Writer) error {
	// Write metadata file header.
	if _, err := fmt.Fprintf(w, "go object %s %s %s %s\n", p.GOOS, p.GOARCH, p.Version, p.Experiments); err != nil {
		return fmt.Errorf("failed to write go object header: %w", err)
	}
	if p.BuildID != "" {
		if _, err := fmt.Fprintf(w, "build id %q\n", p.BuildID); err != nil {
			return fmt.Errorf("failed to write build id: %w", err)
		}
	}
	if p.Package != nil && p.Package.Name() == "main" {
		if _, err := fmt.Fprintf(w, "main\n"); err != nil {
			return fmt.Errorf("failed to write main package tag: %w", err)
		}
	}
	if _, err := fmt.Fprint(w, "\n"); err != nil {
		return fmt.Errorf("failed to write header end: %w", err)
	}

	// Write compiler package info, if present.
	if p.Package != nil {
		if err := gcexportdata.Write(w, p.FSet, p.Package); err != nil {
			return fmt.Errorf("failed to encode gc export data: %w", err)
		}
	}
	return nil
}

// AsEntry returns an archive entry representing the package definition.
func (p *PkgDef) AsEntry() (Entry, error) {
	buf := &bytes.Buffer{}
	if err := p.Write(buf); err != nil {
		return Entry{}, err
	}
	return NewEntry(PkgDefName, buf.Bytes()), nil
}
