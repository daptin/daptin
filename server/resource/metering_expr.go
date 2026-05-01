package resource

import (
	"fmt"
	"math"
	"strconv"

	"github.com/dop251/goja"
)

func EvaluateMeteringCost(expr string, env map[string]interface{}) (int64, error) {
	if expr == "" {
		expr = "1"
	}

	vm := goja.New()
	for key, val := range env {
		if err := vm.Set(key, val); err != nil {
			return 0, err
		}
	}

	value, err := vm.RunString(expr)
	if err != nil {
		return 0, err
	}

	return meteringValueToInt(value.Export())
}

func meteringValueToInt(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case uint:
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("metering value overflows int64: %d", v)
		}
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("invalid metering value: %v", v)
		}
		if v < 0 {
			return 0, nil
		}
		return int64(math.Ceil(v)), nil
	case float32:
		return meteringValueToInt(float64(v))
	case string:
		if v == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, err
		}
		return meteringValueToInt(parsed)
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("metering expression returned unsupported type %T", value)
	}
}
