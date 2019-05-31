package envstruct

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
)

// ReportWriter struct writing to stderr by default
var ReportWriter io.Writer = os.Stderr

// WriteReport will take a struct that is setup for envstruct and print
// out a report containing the struct field name, field type, environment
// variable for that field, whether or not the field is required and
// the value of that field. The report is written to `ReportWriter`
// which defaults to `os.StdOut`. By default all values are omitted. This
// prevents logging of secrets. To not omit a value, you must add the `report`
// value in the `env` struct tag.
func WriteReport(t interface{}) error {
	w := tabwriter.NewWriter(ReportWriter, 0, 8, 2, ' ', 0)

	fmt.Fprintln(w, "FIELD NAME:\tTYPE:\tENV:\tREQUIRED:\tVALUE:")

	if err := writeReport(t, w); err != nil {
		return err
	}

	return w.Flush()
}

func writeReport(t interface{}, w io.Writer) error {
	name := reflect.TypeOf(t).Elem().Name()
	val := reflect.ValueOf(t).Elem()

	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		// If field does not have the `env` tag, check to see if it is a struct,
		// if it is not, then continue to next field, otherwise write the report
		// for the sub struct.
		if tag.Get("env") == "" {
			if valueField.Kind() == reflect.Struct {
				if err := writeReport(valueField.Addr().Interface(), w); err != nil {
					return err

				}
			}

			if valueField.Kind() == reflect.Ptr {
				if err := writeReport(valueField.Interface(), w); err != nil {
					return err

				}
			}

			continue
		}

		tagProperties := separateOnComma(tag.Get("env"))
		envVar := strings.ToUpper(tagProperties[indexEnvVar])
		isRequired := tagPropertiesContains(tagProperties, tagRequired)

		displayedValue := "(OMITTED)"
		if tagPropertiesContains(tagProperties, tagReport) {
			displayedValue = fmt.Sprint(valueField)
		}

		fmt.Fprintln(w, fmt.Sprintf(
			"%s.%v\t%v\t%v\t%t\t%v",
			name,
			typeField.Name,
			valueField.Type(),
			envVar,
			isRequired,
			displayedValue))
	}

	return nil
}
