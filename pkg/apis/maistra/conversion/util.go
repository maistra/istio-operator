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

func stringToInterfaceArray(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i, val := range in {
		out[i] = val
	}
	return out
}

func mapOfInterfaceToString(in map[string]interface{}) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v.(string)
	}
	return out
}

func mapOfStringToInterface(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
