// Package infofile provides a mechanism for writing out information about the
// current process to a temporary file. Such information might include port
// numbers for listening services, process IDs, etc.
package infofile

import (
	"log"
	"os"
	"path"

	ini "github.com/go-ini"
)

// InfoFile represents a file containing information about the current process.
type InfoFile struct {
	name string
	path string
}

// Option allows you to override defaults when creating an InfoFile.
type Option func(i *InfoFile)

// WithPath sets the path explicitly.
func WithPath(path string) Option {
	return func(infoFile *InfoFile) {
		infoFile.path = path
	}
}

// New instantiates an InfoFile and creates the file it represents.
func New(name string, opts ...Option) InfoFile {
	i := InfoFile{
		name: name,
		path: path.Join(os.TempDir(), name+".ini"),
	}

	for _, o := range opts {
		o(&i)
	}

	err := initPath(i.path)
	if err != nil {
		log.Printf("failed to create infofile with path: %s, err: %s", i.path, err)
	}

	return i
}

// Path returns the path of the created file.
func (i *InfoFile) Path() string {
	return i.path
}

// Set adds or overwrites a key in the file.
func (i *InfoFile) Set(key, value string) {
	iniFile, err := ini.Load(i.path)
	if err != nil {
		log.Printf("failed to load infofile with path: %s, err: %s", i.path, err)
		return
	}
	_, err = iniFile.Section(i.name).NewKey(key, value)
	if err != nil {
		log.Printf("failed to save to infofile with key: %s path: %s, err: %s", key, i.path, err)
		return
	}

	err = iniFile.SaveTo(i.path)
	if err != nil {
		log.Printf("failed to write infofile with path: %s, err: %s", i.path, err)
	}
}

func initPath(p string) error {
	dir := path.Dir(p)

	err := os.MkdirAll(dir, os.FileMode(0755))
	if err != nil {
		return err
	}

	_, err = os.Create(p)
	if err != nil {
		return err
	}

	return nil
}
