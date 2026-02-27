package types

import (
	"regexp"
	"time"
)

// Volume describes the details of a volume.
type Volume struct {
	ID         string
	ShortID    string
	Name       string
	Driver     string
	Labels     map[string]string
	Mountpoint string
	Created    time.Time
}

// Match will match given type with given key value pair.
func (v *Volume) Match(typ string, key string, val string) (bool, error) {
	if typ == "name" {
		return v.nameMatch(key)
	}
	if typ == "driver" {
		return v.Driver == key, nil
	}
	if typ != "label" {
		return true, nil
	}
	vv, ok := v.Labels[key]
	if !ok {
		return false, nil
	}
	return vv == val, nil
}

func (v *Volume) nameMatch(key string) (bool, error) {
	// Fast path, exact match
	if v.Name == key {
		return true, nil
	}
	// Fallback to regexp
	match, err := regexp.MatchString(key, v.Name)
	if err != nil {
		return false, err
	}
	return match, nil
}
