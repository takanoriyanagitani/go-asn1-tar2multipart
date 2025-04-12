package tar2mpart

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/asn1"
	"errors"
	"io"
	"iter"
	"mime/multipart"
	"net/textproto"
	"os"
	"time"
)

type FileType asn1.Enumerated

const (
	FileTypeUnspecified FileType = 0x00
	FileTypeRegular     FileType = 0x10
	FileTypeSymlink     FileType = 0x12
	FileTypeDir         FileType = 0x15
)

type Format asn1.Enumerated

const (
	FormatUnspecified Format = 0x00
	FormatUSTAR       Format = 0x10
	FormatPAX         Format = 0x20
	FormatGNU         Format = 0x30
)

type FileHeader struct {
	FileType

	Name     string `asn1:"utf8"`
	Linkname string `asn1:"utf8"`

	Size int64

	Mode int64

	Uid int
	Gid int

	Uname string `asn1:"utf8"`
	Gname string `asn1:"utf8"`

	Modified time.Time

	Format
}

func (h FileHeader) ToDerBytes() ([]byte, error) { return asn1.Marshal(h) }

type TarHeader struct {
	*tar.Header
}

func (t TarHeader) ToFileType() FileType {
	switch t.Header.Typeflag {
	case tar.TypeReg:
		return FileTypeRegular
	case tar.TypeSymlink:
		return FileTypeSymlink
	case tar.TypeDir:
		return FileTypeDir
	default:
		return FileTypeUnspecified
	}
}

func (t TarHeader) FileName() string { return t.Header.Name }
func (t TarHeader) LinkName() string { return t.Header.Linkname }

func (t TarHeader) FileSize() int64 { return t.Header.Size }
func (t TarHeader) FileMode() int64 { return t.Header.Mode }

func (t TarHeader) UserID() int  { return t.Header.Uid }
func (t TarHeader) GroupID() int { return t.Header.Gid }

func (t TarHeader) UserName() string  { return t.Header.Uname }
func (t TarHeader) GroupName() string { return t.Header.Gname }

func (t TarHeader) Modified() time.Time { return t.Header.ModTime }

func (t TarHeader) TarFormat() Format {
	switch t.Header.Format {
	case tar.FormatUSTAR:
		return FormatUSTAR
	case tar.FormatPAX:
		return FormatPAX
	case tar.FormatGNU:
		return FormatGNU
	default:
		return FormatUnspecified
	}
}

func (t TarHeader) ToHeader() FileHeader {
	return FileHeader{
		FileType: t.ToFileType(),
		Name:     t.FileName(),
		Linkname: t.LinkName(),
		Size:     t.FileSize(),
		Mode:     t.FileMode(),
		Uid:      t.UserID(),
		Gid:      t.GroupID(),
		Uname:    t.UserName(),
		Gname:    t.GroupName(),
		Modified: t.Modified(),
		Format:   t.TarFormat(),
	}
}

type TarItemAsn1 struct {
	FileHeader
	Content []byte
}

func (i TarItemAsn1) ToDerBytes() ([]byte, error) { return asn1.Marshal(i) }

type TarReader struct{ *tar.Reader }

func TarReaderFromStdin() TarReader {
	return TarReader{tar.NewReader(os.Stdin)}
}

func (r TarReader) ToItems(limit int64) iter.Seq2[TarItemAsn1, error] {
	return func(yield func(TarItemAsn1, error) bool) {
		var buf bytes.Buffer
		var empty TarItemAsn1
		for {
			hdr, e := r.Reader.Next()
			if io.EOF == e {
				return
			}
			if nil != e {
				yield(empty, e)
				return
			}

			buf.Reset()

			limited := &io.LimitedReader{
				R: r.Reader,
				N: limit,
			}
			_, e = io.Copy(&buf, limited)
			if nil != e {
				yield(empty, e)
				return
			}

			var th TarHeader = TarHeader{hdr}
			var fh FileHeader = th.ToHeader()

			item := TarItemAsn1{
				FileHeader: fh,
				Content:    buf.Bytes(),
			}

			if !yield(item, nil) {
				return
			}
		}
	}
}

type MultipartWriter struct{ *multipart.Writer }

const ContentTypeKey string = "Content-Type"

const ContentTypeVal string = "application/asn1-der"

func (w MultipartWriter) CreateAsn1Header() textproto.MIMEHeader {
	ret := textproto.MIMEHeader{}
	ret.Set(ContentTypeKey, ContentTypeVal)
	return ret
}

func (w MultipartWriter) WriteAsn1Bytes(der []byte) error {
	var hdr textproto.MIMEHeader = w.CreateAsn1Header()
	wtr, e := w.Writer.CreatePart(hdr)
	if nil != e {
		return e
	}
	_, e = wtr.Write(der)
	return e
}

func (w MultipartWriter) WriteItems(
	ctx context.Context,
	i iter.Seq2[TarItemAsn1, error],
) error {
	for item, e := range i {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if nil != e {
			return e
		}

		der, e := item.ToDerBytes()
		if nil != e {
			return e
		}

		e = w.WriteAsn1Bytes(der)
		if nil != e {
			return e
		}
	}
	return nil
}

type Writer struct{ io.Writer }

func (w Writer) WriteAll(
	ctx context.Context,
	i iter.Seq2[TarItemAsn1, error],
) error {
	var wtr *multipart.Writer = multipart.NewWriter(w.Writer)
	e := MultipartWriter{wtr}.WriteItems(
		ctx,
		i,
	)
	return errors.Join(e, wtr.Close())
}

func ItemsToStdout(
	ctx context.Context,
	i iter.Seq2[TarItemAsn1, error],
) error {
	wtr := Writer{os.Stdout}
	return wtr.WriteAll(ctx, i)
}
