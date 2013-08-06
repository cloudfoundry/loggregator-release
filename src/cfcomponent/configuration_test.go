package cfcomponent

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

type Config struct {
	Username string
}

func TestReadingFromJsonFile(t *testing.T) {
	file, err := ioutil.TempFile("", "config")
	defer func() {
		os.Remove(file.Name())
	}()
	assert.NoError(t, err)
	_, err = file.Write([]byte(`{"UserName":"User"}`))
	assert.NoError(t, err)

	err = file.Close()
	assert.NoError(t, err)

	config := &Config{}
	err = ReadConfigInto(config, file.Name())
	assert.NoError(t, err)

	assert.Equal(t, config.Username, "User")
}

func TestReturnsErrorIfFileNotFound(t *testing.T) {
	config := &Config{}
	err := ReadConfigInto(config, "/foo/config.json")
	assert.Error(t, err)
}

func TestReturnsErrorIfInvalidJson(t *testing.T) {
	file, err := ioutil.TempFile("", "config")
	defer func() {
		os.Remove(file.Name())
	}()
	assert.NoError(t, err)
	_, err = file.Write([]byte(`NotJson`))
	assert.NoError(t, err)

	err = file.Close()
	assert.NoError(t, err)

	config := &Config{}
	err = ReadConfigInto(config, file.Name())
	assert.Error(t, err)
}
