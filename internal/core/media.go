package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// mediaRefRe matches a reference to a file under the store's media/ dir, in any
// of the forms a body can carry one: a markdown image, a `src:` line inside a
// video/embed/compare fence, or a bare path.
var mediaRefRe = regexp.MustCompile(`media/[A-Za-z0-9._\-/]+`)

// MediaRefs returns every media/ reference in a body, in order of appearance
// (duplicates included). It is the one place the reference syntax is known, so
// doctor, the visual-coverage check and any future consumer agree on what
// counts as a reference.
func MediaRefs(body string) []string {
	return mediaRefRe.FindAllString(body, -1)
}

// HasVisual reports whether an entry ships at least one visual: a cover image
// or any media/ reference in its body. The skill asks every entry for one, so
// this is the check behind the doctor report and the stamp-time nudge.
func HasVisual(e Entry) bool {
	return e.FM.Cover != "" || len(MediaRefs(e.Body)) > 0
}

// MediaKind classifies an imported artifact so it renders with the right block.
type MediaKind string

const (
	KindImage MediaKind = "image" // png/jpg/webp/gif → inline image with lightbox
	KindVideo MediaKind = "video" // mp4/webm/mov → <video> player
	KindEmbed MediaKind = "embed" // html → sandboxed iframe artifact
)

// KindFor guesses a media kind from a file extension.
func KindFor(path string) MediaKind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".webm", ".mov", ".m4v":
		return KindVideo
	case ".html", ".htm":
		return KindEmbed
	default:
		return KindImage
	}
}

// ImportMedia copies src into the store's media/ dir and returns the path to
// reference it by (relative to an entry, i.e. "media/<name>"). If name is empty
// the source filename is used; collisions get a numeric suffix.
func (s *Store) ImportMedia(src, name string) (string, error) {
	if name == "" {
		name = filepath.Base(src)
	} else if filepath.Ext(name) == "" {
		name += filepath.Ext(src)
	}
	name = sanitizeFilename(name)
	if err := os.MkdirAll(s.MediaDir(), 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(s.MediaDir(), name)
	for n := 2; fileExists(dst); n++ {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		dst = filepath.Join(s.MediaDir(), fmt.Sprintf("%s-%d%s", base, n, ext))
	}
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	return "media/" + filepath.Base(dst), nil
}

// MediaBlock renders the markdown/component snippet for a piece of media.
func MediaBlock(ref, caption string, kind MediaKind) string {
	switch kind {
	case KindVideo:
		b := "```video\nsrc: " + ref + "\n"
		if caption != "" {
			b += "caption: " + caption + "\n"
		}
		return b + "```\n"
	case KindEmbed:
		b := "```embed\nsrc: " + ref + "\nheight: 480\n"
		if caption != "" {
			b += "caption: " + caption + "\n"
		}
		return b + "```\n"
	default:
		return fmt.Sprintf("![%s](%s)\n", caption, ref)
	}
}

// CompareBlock renders a before/after slider component.
func CompareBlock(before, after, caption string) string {
	b := "```compare\nbefore: " + before + "\nafter: " + after + "\n"
	if caption != "" {
		b += "caption: " + caption + "\n"
	}
	return b + "```\n"
}

// AppendBody appends a markdown chunk to an entry's body and saves it. A body
// that is still nothing but the `katra new` placeholder counts as empty, so the
// first append replaces the prompt instead of writing underneath it.
func (s *Store) AppendBody(e *Entry, chunk string) error {
	body := strings.TrimRight(e.Body, "\n")
	if IsDraftPlaceholder(body) {
		body = ""
	}
	if body != "" {
		body += "\n\n"
	}
	e.Body = body + strings.TrimRight(chunk, "\n") + "\n"
	return e.Save()
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.' || r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, name)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
