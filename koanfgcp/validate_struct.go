package koanfgcp

import (
	"fmt"
	"github.com/pkg/errors"
	"reflect"
	"strings"
)

func validateAndResolve(cfg interface{}) (map[string]string, error) {
	s, err := validateStruct(cfg)
	if err != nil {
		return nil, err
	}

	return resolveSecretNames(false, []string{}, s), nil
}

func validateStruct(cfg interface{}) (reflect.Value, error) {
	rootType := reflect.TypeOf(cfg)
	rootKind := rootType.Kind()

	if rootKind != reflect.Ptr {
		return reflect.Value{}, errors.New("cfg argument must be a pointer to a struct, got:" + rootType.Kind().String())
	}

	rootValElem := reflect.ValueOf(cfg).Elem()
	rootValElemKind := rootValElem.Kind()
	if rootValElemKind != reflect.Struct {
		return reflect.Value{}, errors.New("cfg argument must be a pointer to a struct, got: " + rootValElemKind.String())
	}

	for i := 0; i < rootValElem.NumField(); i++ {
		field := rootValElem.Field(i)
		fieldType := field.Kind()
		fieldName := field.Type().Name()
		_ = fieldName
		if fieldType == reflect.Struct {
			_, err := validateStruct(field.Addr().Interface())
			if err != nil {
				return reflect.Value{}, err
			}
			continue
		}
		if err := validateField(field); err != nil {
			return reflect.Value{}, err
		}
	}

	return rootValElem, nil
}

func validateField(field reflect.Value) error {
	name := field.Type().Name()
	fieldType := field.Kind()

	if !field.IsValid() || !field.CanSet() {
		return errors.New(fmt.Sprintf("field %s is not valid - check if field is value and that it is exported from struct", name))
	}

	if fieldType != reflect.String {
		//return errors.New(fmt.Sprintf("pointer struct can only contain string fields - field '%s' is of type '%s'", name, field.Type().Name()))
	}

	return nil
}

func resolveSecretNames(skipNil bool, prefix []string, s reflect.Value) map[string]string {
	secrets := make(map[string]string)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fieldType := f.Kind()
		koanftag := s.Type().Field(i).Tag.Get(koanfTag)
		koanfTagWithPrefix := append(prefix, koanftag)

		gcpTag := s.Type().Field(i).Tag.Get(gcpSecretTag)
		if fieldType == reflect.Struct || (fieldType == reflect.Ptr && f.Elem().Kind() == reflect.Struct) {
			if fieldType == reflect.Ptr {
				f = f.Elem()
			}
			res := resolveSecretNames(skipNil, koanfTagWithPrefix, f)
			for k, v := range res {
				secrets[k] = v
			}
			continue
		}

		if gcpTag == "" {
			continue
		}

		koanfKey := strings.Join(koanfTagWithPrefix, ".")
		secrets[koanfKey] = gcpTag
	}
	return secrets
}
