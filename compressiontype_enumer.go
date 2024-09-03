// Code generated by "enumer -json -text -sql -type CompressionType -transform snake -trimprefix CompressionType"; DO NOT EDIT.

package zencached

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

const _CompressionTypeName = "nonebase64zstandard"

var _CompressionTypeIndex = [...]uint8{0, 4, 10, 19}

const _CompressionTypeLowerName = "nonebase64zstandard"

func (i CompressionType) String() string {
	if i >= CompressionType(len(_CompressionTypeIndex)-1) {
		return fmt.Sprintf("CompressionType(%d)", i)
	}
	return _CompressionTypeName[_CompressionTypeIndex[i]:_CompressionTypeIndex[i+1]]
}

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the stringer command to generate them again.
func _CompressionTypeNoOp() {
	var x [1]struct{}
	_ = x[CompressionTypeNone-(0)]
	_ = x[CompressionTypeBase64-(1)]
	_ = x[CompressionTypeZstandard-(2)]
}

var _CompressionTypeValues = []CompressionType{CompressionTypeNone, CompressionTypeBase64, CompressionTypeZstandard}

var _CompressionTypeNameToValueMap = map[string]CompressionType{
	_CompressionTypeName[0:4]:        CompressionTypeNone,
	_CompressionTypeLowerName[0:4]:   CompressionTypeNone,
	_CompressionTypeName[4:10]:       CompressionTypeBase64,
	_CompressionTypeLowerName[4:10]:  CompressionTypeBase64,
	_CompressionTypeName[10:19]:      CompressionTypeZstandard,
	_CompressionTypeLowerName[10:19]: CompressionTypeZstandard,
}

var _CompressionTypeNames = []string{
	_CompressionTypeName[0:4],
	_CompressionTypeName[4:10],
	_CompressionTypeName[10:19],
}

// CompressionTypeString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func CompressionTypeString(s string) (CompressionType, error) {
	if val, ok := _CompressionTypeNameToValueMap[s]; ok {
		return val, nil
	}

	if val, ok := _CompressionTypeNameToValueMap[strings.ToLower(s)]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to CompressionType values", s)
}

// CompressionTypeValues returns all values of the enum
func CompressionTypeValues() []CompressionType {
	return _CompressionTypeValues
}

// CompressionTypeStrings returns a slice of all String values of the enum
func CompressionTypeStrings() []string {
	strs := make([]string, len(_CompressionTypeNames))
	copy(strs, _CompressionTypeNames)
	return strs
}

// IsACompressionType returns "true" if the value is listed in the enum definition. "false" otherwise
func (i CompressionType) IsACompressionType() bool {
	for _, v := range _CompressionTypeValues {
		if i == v {
			return true
		}
	}
	return false
}

// MarshalJSON implements the json.Marshaler interface for CompressionType
func (i CompressionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface for CompressionType
func (i *CompressionType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("CompressionType should be a string, got %s", data)
	}

	var err error
	*i, err = CompressionTypeString(s)
	return err
}

// MarshalText implements the encoding.TextMarshaler interface for CompressionType
func (i CompressionType) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for CompressionType
func (i *CompressionType) UnmarshalText(text []byte) error {
	var err error
	*i, err = CompressionTypeString(string(text))
	return err
}

func (i CompressionType) Value() (driver.Value, error) {
	return i.String(), nil
}

func (i *CompressionType) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	case fmt.Stringer:
		str = v.String()
	default:
		return fmt.Errorf("invalid value of CompressionType: %[1]T(%[1]v)", value)
	}

	val, err := CompressionTypeString(str)
	if err != nil {
		return err
	}

	*i = val
	return nil
}