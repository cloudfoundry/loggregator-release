package cfcomponent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

func ReadConfigInto(config interface{}, configPath string) error {
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not read config file [%s]: %s", configPath, err))
	}
	err = json.Unmarshal(configBytes, config)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not parse config file %s: %s", configPath, err))
	}
	return nil
}
