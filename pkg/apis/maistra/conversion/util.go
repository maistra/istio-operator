package conversion

func int64Ptr(val int64) *int64 {
	valPtr := new(int64)
	*valPtr = val
	return valPtr
}

func boolPtr(val bool) *bool {
	valPtr := new(bool)
	*valPtr = val
	return valPtr
}

func strPtr(val string) *string {
	valPtr := new(string)
	*valPtr = val
	return valPtr
}

func interfaceToStringArray(in []interface{}) []string {
	strArr := make([]string, len(in))
	for i, v := range in {
		strArr[i] = v.(string)
	}
	return strArr
}

func stringToInterfaceArray(value []string) []interface{} {
	valLen := len(value)
	rawVal := make([]interface{}, valLen)
	for i, val := range value {
		rawVal[i] = val
	}
	return rawVal
}

func mapOfInterfaceToString(in map[string]interface{}) map[string]string {
	valLen := len(in)
	rawVal := make(map[string]string, valLen)
	for k, v := range in {
		rawVal[k] = v.(string)
	}
	return rawVal
}

func mapOfStringToInterface(in map[string]string) map[string]interface{} {
	valLen := len(in)
	rawVal := make(map[string]interface{}, valLen)
	for k, v := range in {
		rawVal[k] = v
	}
	return rawVal
}
