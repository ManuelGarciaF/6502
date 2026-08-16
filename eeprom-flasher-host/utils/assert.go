package assert

import (
	"cmp"
	"fmt"
)

func True(condition bool, msg string) {
	if !condition {
		panic(fmt.Sprintf("Assertion failed: %v", msg))
	}
}

func False(condition bool, msg string) {
	if condition {
		panic(fmt.Sprintf("Assertion failed: %v", msg))
	}
}

func Equal[T comparable](a, b T, msg string) {
	if a != b {
		panic(fmt.Sprintf("Assertion failed: %s (%v != %v)", msg, a, b))
	}
}

func NotEqual[T comparable](a, b T, msg string) {
	if a == b {
		panic(fmt.Sprintf("Assertion failed: %s (expected != %v)", msg, b))
	}
}

func GreaterThan[T cmp.Ordered](a, b T, msg string) {
	if a <= b {
		panic(fmt.Sprintf("Assertion failed: %s (%v <= %v)", msg, a, b))
	}
}

func GreaterThanOrEqual[T cmp.Ordered](a, b T, msg string) {
	if a < b {
		panic(fmt.Sprintf("Assertion failed: %s (%v < %v)", msg, a, b))
	}
}

func LessThan[T cmp.Ordered](a, b T, msg string) {
	if a >= b {
		panic(fmt.Sprintf("Assertion failed: %s (%v >= %v)", msg, a, b))
	}
}

func LessThanOrEqual[T cmp.Ordered](a, b T, msg string) {
	if a > b {
		panic(fmt.Sprintf("Assertion failed: %s (%v > %v)", msg, a, b))
	}
}
