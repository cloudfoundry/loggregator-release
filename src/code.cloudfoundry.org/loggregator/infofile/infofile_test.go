package infofile_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	ini "github.com/go-ini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/loggregator/infofile"
)

var _ = Describe("Infofile", func() {
	It("creates a new tmp file with component name", func() {
		info := infofile.New("component_name")
		defer os.RemoveAll(info.Path())
		Expect(info.Path()).To(BeAnExistingFile())
		expectedFileName := path.Join(os.TempDir(), "component_name.ini")
		Expect(info.Path()).To(Equal(expectedFileName))
	})

	It("accepts an explicit path", func() {
		infoPath := path.Join(os.TempDir(), "infofile_test_explicit_path", "foobar.ini")
		info := infofile.New("component_name", infofile.WithPath(infoPath))
		defer os.RemoveAll(info.Path())
		Expect(info.Path()).To(BeAnExistingFile())
		Expect(info.Path()).To(Equal(infoPath))
	})

	It("overwrites existing data", func() {
		tempFile, err := ioutil.TempFile("", "")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tempFile.Name())
		_, err = fmt.Fprint(tempFile, "hello world")
		Expect(err).ToNot(HaveOccurred())

		info := infofile.New("component_name", infofile.WithPath(tempFile.Name()))
		reader, err := os.Open(info.Path())
		Expect(err).ToNot(HaveOccurred())

		Expect(ioutil.ReadAll(reader)).To(BeEmpty())
	})

	It("adds data to file, overwriting old values", func() {
		info := infofile.New("append_test")
		defer os.RemoveAll(info.Path())
		info.Set("some_key", "Old Value")
		info.Set("some_key", "New Value")

		data, err := ini.Load(info.Path())
		Expect(err).ToNot(HaveOccurred())

		section, err := data.GetSection("append_test")
		Expect(err).ToNot(HaveOccurred())

		key, err := section.GetKey("some_key")
		Expect(err).ToNot(HaveOccurred())

		Expect(key.String()).To(Equal("New Value"))
	})

	It("silently fails if unable to create", func() {
		infoPath := path.Join(os.TempDir(), "infofile_test_silently", "foobar.ini")
		Expect(os.MkdirAll(infoPath, os.FileMode(0755))).To(Succeed())
		defer os.RemoveAll(infoPath)

		info := infofile.New("component_name", infofile.WithPath(infoPath))
		Expect(info.Path()).To(BeADirectory())
		Expect(info.Path()).To(Equal(infoPath))

		info.Set("some_key", "Some Value")
		Expect(info.Path()).To(BeADirectory())
	})

	It("silently fails if unable to create parent directory", func() {
		badPath := path.Join(os.TempDir(), "infofile_test_silently_fails_parent")
		_, err := os.Create(badPath)
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(badPath)

		infoPath := path.Join(os.TempDir(), "infofile_test_silently_fails_parent", "foobar.ini")
		info := infofile.New("component_name", infofile.WithPath(infoPath))
		defer os.RemoveAll(infoPath)
		Expect(path.Base(info.Path())).ToNot(BeADirectory())
		Expect(info.Path()).To(Equal(infoPath))

		info.Set("some_key", "Some Value")
		Expect(path.Base(info.Path())).ToNot(BeADirectory())
	})

	It("silently fails if unable to write invalid key", func() {
		invalidKey := ""

		info := infofile.New("invalid_key")
		defer os.RemoveAll(info.Path())

		info.Set("foobar", "Some Value")
		info.Set(invalidKey, "Some Value")

		data, err := ini.Load(info.Path())
		Expect(err).ToNot(HaveOccurred())

		section, err := data.GetSection("invalid_key")
		Expect(err).ToNot(HaveOccurred())

		_, err = section.GetKey(invalidKey)
		Expect(err).To(HaveOccurred())
	})
})
