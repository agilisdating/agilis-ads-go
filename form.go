package agilisads

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// File is an image to upload. Create one with FileFromPath, FileFromBytes or
// FileFromReader. The content type is detected from the file name unless set
// with WithContentType.
type File struct {
	name        string
	path        string
	data        []byte
	reader      io.Reader
	contentType string
}

// FileFromPath uploads the file at path. It is read when the request is sent.
func FileFromPath(path string) *File {
	return &File{name: filepath.Base(path), path: path}
}

// FileFromBytes uploads data with the given file name.
func FileFromBytes(name string, data []byte) *File {
	return &File{name: name, data: data}
}

// FileFromReader uploads what r holds, with the given file name. The reader is
// read once, when the request is first sent, and the bytes are reused if the
// request is retried.
func FileFromReader(name string, r io.Reader) *File {
	return &File{name: name, reader: r}
}

// WithContentType sets the content type instead of detecting it from the
// name, and returns f.
func (f *File) WithContentType(contentType string) *File {
	f.contentType = contentType
	return f
}

// Name returns the file name sent with the upload.
func (f *File) Name() string { return f.name }

// ContentType returns the content type sent with the upload.
func (f *File) ContentType() string {
	if f.contentType != "" {
		return f.contentType
	}
	return contentTypeFor(f.name)
}

func (f *File) bytes() ([]byte, error) {
	switch {
	case f.data != nil:
		return f.data, nil
	case f.reader != nil:
		b, err := io.ReadAll(f.reader)
		if err != nil {
			return nil, fmt.Errorf("agilisads: reading %s: %w", f.name, err)
		}
		f.data, f.reader = b, nil
		return b, nil
	case f.path != "":
		b, err := os.ReadFile(f.path)
		if err != nil {
			return nil, fmt.Errorf("agilisads: reading %s: %w", f.name, err)
		}
		return b, nil
	}
	return []byte{}, nil
}

func contentTypeFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	if t := mime.TypeByExtension(filepath.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// LatLng is a position: latitude first, then longitude. In JSON it is the
// array [latitude, longitude]; in a multipart form it is two location[] parts.
type LatLng struct {
	Latitude  float64
	Longitude float64
}

// MarshalJSON encodes the position as [latitude, longitude].
func (l LatLng) MarshalJSON() ([]byte, error) {
	return []byte("[" + formatFloat(l.Latitude) + "," + formatFloat(l.Longitude) + "]"), nil
}

// Crop is the area of an uploaded event image to use: the top-left corner and
// the width, in pixels. The height is derived from the 16:9 frame.
type Crop struct {
	X int
	Y int
	W int
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// form builds a multipart/form-data body.
type form struct {
	buf bytes.Buffer
	w   *multipart.Writer
	err error
}

func newForm() *form {
	f := &form{}
	f.w = multipart.NewWriter(&f.buf)
	return f
}

func (f *form) field(name, value string) {
	if f.err != nil {
		return
	}
	f.err = f.w.WriteField(name, value)
}

// str writes a field when the value is not empty.
func (f *form) str(name, value string) {
	if value != "" {
		f.field(name, value)
	}
}

// optStr writes a field when the pointer is set, even to "".
func (f *form) optStr(name string, value *string) {
	if value != nil {
		f.field(name, *value)
	}
}

func (f *form) optFloat(name string, value *float64) {
	if value != nil {
		f.field(name, formatFloat(*value))
	}
}

func (f *form) optInt64(name string, value *int64) {
	if value != nil {
		f.field(name, strconv.FormatInt(*value, 10))
	}
}

func (f *form) location(l LatLng) {
	f.field("location[]", formatFloat(l.Latitude))
	f.field("location[]", formatFloat(l.Longitude))
}

func (f *form) crop(c *Crop) {
	if c == nil {
		return
	}
	f.field("crop[x]", strconv.Itoa(c.X))
	f.field("crop[y]", strconv.Itoa(c.Y))
	f.field("crop[w]", strconv.Itoa(c.W))
}

func (f *form) file(name string, file *File) {
	if f.err != nil || file == nil {
		return
	}
	data, err := file.bytes()
	if err != nil {
		f.err = err
		return
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, quoteEscaper.Replace(name), quoteEscaper.Replace(file.name)))
	h.Set("Content-Type", file.ContentType())
	part, err := f.w.CreatePart(h)
	if err != nil {
		f.err = err
		return
	}
	_, f.err = part.Write(data)
}

func (f *form) finish() ([]byte, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	if err := f.w.Close(); err != nil {
		return nil, "", err
	}
	return f.buf.Bytes(), f.w.FormDataContentType(), nil
}

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
