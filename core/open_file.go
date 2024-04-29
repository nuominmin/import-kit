package core

import (
	"bytes"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

type OpenFileFunc func() (*File, error)

type File struct {
	*excelize.File
}

func newFile() *File {
	return &File{excelize.NewFile()}
}

func OpenLocalFile(filename string) (*File, error) {
	file, err := excelize.OpenFile(filename)
	return &File{file}, err
}

func OpenLocalFileFunc(filename string) OpenFileFunc {
	return func() (*File, error) {
		return OpenLocalFile(filename)
	}
}

func OpenBytesFile(content []byte) (*File, error) {
	file, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	return &File{file}, nil
}

func OpenBytesFileFunc(content []byte) OpenFileFunc {
	return func() (*File, error) {
		return OpenBytesFile(content)
	}
}

func (p *File) SaveAs(filename string) error {
	return p.File.SaveAs(filename)
}

func (p *File) SaveDefaultErrorFile() error {
	return p.File.SaveAs(fmt.Sprintf("%s_error.xlsx", time.Now().Format("20060102150405")))
}
