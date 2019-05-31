package envstruct

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	indexEnvVar = 0

	tagRequired = "required"
	tagReport   = "report"
)

// Unmarshaller is a type which unmarshals itself from an environment variable.
type Unmarshaller interface {
	UnmarshalEnv(v string) error
}

// Load will use the `env` tags from a struct to populate the structs values and
// perform validations.
func Load(t interface{}) error {
	missing, err := load(t)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			"missing required environment variables: %s",
			strings.Join(uniqueStrings(missing), ", "),
		)
	}

	return nil
}

func load(t interface{}) (missing []string, err error) {
	val := reflect.ValueOf(t).Elem()

	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		tagProperties := separateOnComma(tag.Get("env"))
		envVar := tagProperties[indexEnvVar]
		envVal := os.Getenv(envVar)
		required := tagPropertiesContains(tagProperties, tagRequired)

		if isInvalid(envVal, required) {
			missing = append(missing, envVar)
			continue
		}

		hasEnvTag := envVar != ""
		subMissing, err := setField(valueField, envVal, hasEnvTag)
		if err != nil {
			return nil, err
		}

		missing = append(missing, subMissing...)
	}

	return missing, nil
}

// ToEnv will return a slice of strings that can be used with exec.Cmd.Env
// formatted as `ENVAR_NAME=value` for a given struct.
func ToEnv(t interface{}) []string {
	val := reflect.ValueOf(t).Elem()

	var results []string
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		tagProperties := separateOnComma(tag.Get("env"))
		envVar := tagProperties[indexEnvVar]

		switch valueField.Kind() {
		case reflect.Slice:
			results = append(results, formatSlice(envVar, valueField))
		case reflect.Map:
			results = append(results, formatMap(envVar, valueField))
		case reflect.Struct:
			results = append(results, ToEnv(valueField.Addr().Interface())...)
		case reflect.Ptr:
			if valueField.Type() == reflect.TypeOf(&url.URL{}) {
				results = append(results, fmt.Sprintf("%s=%+v", envVar, valueField))
				continue
			}

			results = append(results, ToEnv(valueField.Interface())...)
		default:
			results = append(results, fmt.Sprintf("%s=%+v", envVar, valueField))
		}
	}

	return results
}

func tagPropertiesContains(properties []string, match string) bool {
	for _, v := range properties {
		if v == match {
			return true
		}
	}

	return false
}

func unmarshaller(v reflect.Value) (Unmarshaller, bool) {
	if unmarshaller, ok := v.Interface().(Unmarshaller); ok {
		return unmarshaller, ok
	}
	if v.CanAddr() {
		return unmarshaller(v.Addr())
	}
	return nil, false
}

func setField(value reflect.Value, input string, hasEnvTag bool) (missing []string, err error) {
	if !value.CanSet() {
		return nil, nil
	}

	if input == "" &&
		(value.Kind() != reflect.Ptr) &&
		(value.Kind() != reflect.Struct) {

		return nil, nil
	}

	if unmarshaller, ok := unmarshaller(value); ok {
		return nil, unmarshaller.UnmarshalEnv(input)
	} else {
		if value.Kind() == reflect.Struct && hasEnvTag {
			return nil, errors.New(fmt.Sprintf("Nested struct %s with env tag needs to have an UnmarshallEnv method\n", value.Type().Name()))
		}
	}

	switch value.Type() {
	case reflect.TypeOf(time.Second):
		return nil, setDuration(value, input)
	case reflect.TypeOf(&url.URL{}):
		return nil, setURL(value, input)
	}

	switch value.Kind() {
	case reflect.String:
		return nil, setString(value, input)
	case reflect.Bool:
		return nil, setBool(value, input)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return nil, setInt(value, input)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return nil, setUint(value, input)
	case reflect.Float32, reflect.Float64:
		return nil, setFloat(value, input)
	case reflect.Complex64, reflect.Complex128:
		return nil, setComplex(value, input)
	case reflect.Slice:
		return nil, setSlice(value, input, hasEnvTag)
	case reflect.Map:
		return nil, setMap(value, input)
	case reflect.Struct:
		return setStruct(value)
	case reflect.Ptr:
		return setPointerToStruct(value, input, hasEnvTag)
	}

	return nil, nil
}

func separateOnComma(input string) []string {
	inputs := strings.Split(input, ",")

	for i, v := range inputs {
		inputs[i] = strings.TrimSpace(v)
	}

	return inputs
}

func isInvalid(input string, required bool) bool {
	return required && input == ""
}

func setStruct(value reflect.Value) (missing []string, err error) {
	return load(value.Addr().Interface())
}

func setPointerToStruct(value reflect.Value, input string, hasEnvTag bool) (missing []string, err error) {
	if value.IsNil() {
		p := reflect.New(value.Type().Elem())
		value.Set(p)
	}

	if value.Type().Elem().Kind() == reflect.Struct {
		return load(value.Interface())
	}

	return setField(value.Elem(), input, hasEnvTag)
}

func setDuration(value reflect.Value, input string) error {
	d, err := time.ParseDuration(input)
	if err != nil {
		return err
	}

	value.Set(reflect.ValueOf(d))

	return nil
}

func setURL(value reflect.Value, input string) error {
	u, err := url.Parse(input)
	if err != nil {
		return err
	}

	value.Set(reflect.ValueOf(u))

	return nil
}

func setString(value reflect.Value, input string) error {
	value.SetString(input)

	return nil
}

func setBool(value reflect.Value, input string) error {
	value.SetBool(input == "true" || input == "1")

	return nil
}

func setInt(value reflect.Value, input string) error {
	n, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return err
	}

	value.SetInt(int64(n))

	return nil
}

func setUint(value reflect.Value, input string) error {
	n, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		return err
	}

	value.SetUint(uint64(n))

	return nil
}

func setFloat(value reflect.Value, input string) error {
	n, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return err
	}

	value.SetFloat(float64(n))

	return nil
}

func setComplex(value reflect.Value, input string) error {
	var n complex128

	count, err := fmt.Sscanf(input, "%g", &n)
	if err != nil {
		return err
	}

	if count != 1 {
		return fmt.Errorf("Expected to parse 1 complex number, found %d", count)
	}

	value.SetComplex(n)

	return nil
}

func setSlice(value reflect.Value, input string, hasEnvTag bool) error {
	inputs := separateOnComma(input)

	rs := reflect.MakeSlice(value.Type(), len(inputs), len(inputs))
	for i, val := range inputs {
		_, err := setField(rs.Index(i), val, hasEnvTag)
		if err != nil {
			return err
		}
	}

	value.Set(rs)

	return nil
}

func setMap(value reflect.Value, input string) error {
	inputs := separateOnComma(input)

	m := make(map[string]string)
	for _, i := range inputs {
		kv := strings.SplitN(i, ":", 2)

		if len(kv) == 0 {
			continue
		}

		if len(kv) < 2 {
			return fmt.Errorf("map[string]string key '%s' is missing a value", kv[0])
		}

		m[kv[0]] = kv[1]
	}

	value.Set(reflect.ValueOf(m))

	return nil
}

func formatSlice(envVar string, value reflect.Value) string {
	var parts []string
	for i := 0; i < value.Len(); i++ {
		parts = append(parts, fmt.Sprintf("%+v", value.Index(i)))
	}

	return fmt.Sprintf("%s=%+v", envVar, strings.Join(parts, ","))
}

func formatMap(envVar string, value reflect.Value) string {
	var parts []string

	keys := value.MapKeys()
	for _, k := range keys {
		v := value.MapIndex(k)

		parts = append(parts, fmt.Sprintf("%+v:%+v", k, v))
	}

	return fmt.Sprintf("%s=%+v", envVar, strings.Join(parts, ","))
}

func uniqueStrings(s []string) []string {
	m := make(map[string]bool)
	for _, str := range s {
		m[str] = true
	}

	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}

	sort.Strings(res)

	return res
}
