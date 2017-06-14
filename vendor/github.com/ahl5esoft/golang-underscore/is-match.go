package underscore

func IsMatch(item interface{}, properties map[string]interface{}) bool {
	if item == nil || len(properties) == 0 {
		return false
	}

	return All(properties, func(pv interface{}, pn string) bool {
		getValue := Property(pn)
		value, err := getValue(item)
		return err == nil && value == pv
	})
}
