package expr

import (
	"fmt"
	"strconv"
	"strings"
)

// EvalBool evaluates a minimal boolean expression used by the
// built-in If and Switch nodes.
//
//   - Operators: ==, !=, >, <, >=, <=.
//   - Operands compare numerically when both parse as float64,
//     otherwise as raw strings.
//   - With no operator, non-empty / non-"0" / non-"false" strings
//     are truthy.
//
// No quoting, logical operators, or parentheses are supported.
func EvalBool(expression string) (bool, error) {
	ops := []string{"==", "!=", ">=", "<=", ">", "<"}

	for _, op := range ops {
		if idx := strings.Index(expression, op); idx != -1 {
			left := strings.TrimSpace(expression[:idx])
			right := strings.TrimSpace(expression[idx+len(op):])
			return compare(left, right, op)
		}
	}

	// no operator — truthy check
	expression = strings.TrimSpace(expression)
	if expression == "" || expression == "0" || expression == "false" {
		return false, nil
	}
	return true, nil
}

func compare(left, right, op string) (bool, error) {
	lf, lErr := strconv.ParseFloat(left, 64)
	rf, rErr := strconv.ParseFloat(right, 64)
	if lErr == nil && rErr == nil {
		return compareNumeric(lf, rf, op)
	}
	return compareString(left, right, op)
}

func compareNumeric(l, r float64, op string) (bool, error) {
	switch op {
	case "==":
		return l == r, nil
	case "!=":
		return l != r, nil
	case ">":
		return l > r, nil
	case "<":
		return l < r, nil
	case ">=":
		return l >= r, nil
	case "<=":
		return l <= r, nil
	default:
		return false, fmt.Errorf("expr: unsupported operator %q", op)
	}
}

func compareString(l, r, op string) (bool, error) {
	switch op {
	case "==":
		return l == r, nil
	case "!=":
		return l != r, nil
	case ">":
		return l > r, nil
	case "<":
		return l < r, nil
	case ">=":
		return l >= r, nil
	case "<=":
		return l <= r, nil
	default:
		return false, fmt.Errorf("expr: unsupported operator %q", op)
	}
}
