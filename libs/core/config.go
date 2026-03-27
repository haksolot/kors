package core

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// LoadEnv populates the fields of cfg (a pointer to a struct) from environment variables.
// Field values are read from the env tag. Fields tagged with ",required" cause an error
// if the variable is absent or empty.
//
// Supported field types: string, int, bool.
//
// Example:
//
//	type Config struct {
//	    NATSURL     string `env:"NATS_URL,required"`
//	    DatabaseURL string `env:"DATABASE_URL,required"`
//	    ServiceName string `env:"SERVICE_NAME,required"`
//	    Debug       bool   `env:"DEBUG"`
//	}
func LoadEnv(cfg any) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("LoadEnv: cfg must be a pointer to a struct")
	}
	v = v.Elem()
	t := v.Type()

	for i := range v.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("env")
		if tag == "" {
			continue
		}

		name, required := parseEnvTag(tag)
		val := os.Getenv(name)
		if val == "" && required {
			return fmt.Errorf("LoadEnv: required env var %s is not set", name)
		}
		if val == "" {
			continue
		}

		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(val)
		case reflect.Bool:
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("LoadEnv: %s=%q is not a valid bool: %w", name, val, err)
			}
			fv.SetBool(b)
		case reflect.Int, reflect.Int64:
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return fmt.Errorf("LoadEnv: %s=%q is not a valid int: %w", name, val, err)
			}
			fv.SetInt(n)
		default:
			return fmt.Errorf("LoadEnv: unsupported field type %s for %s", fv.Kind(), name)
		}
	}
	return nil
}

// parseEnvTag splits "VAR_NAME,required" into ("VAR_NAME", true).
func parseEnvTag(tag string) (name string, required bool) {
	for i, c := range tag {
		if c == ',' {
			return tag[:i], tag[i+1:] == "required"
		}
	}
	return tag, false
}
