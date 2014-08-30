package main

import (
	"strings"
	"testing"
)

func AssertInclude(t *testing.T, actual string, expected string) {
	if !strings.Contains(actual, expected) {
		t.Errorf("%s did not contain %s", actual, expected)
	}
}
