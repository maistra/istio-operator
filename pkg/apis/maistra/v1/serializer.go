package v1

import (
	"encoding/json"
)

// MarshalJSON treats Gateways as an embedded type (it can't be embedded because it's a map type)
func (gc GatewaysConfig) MarshalJSON() ([]byte, error) {
	var data, gatewayData, commonData []byte
    var err error
    // marshal components
	commonData, err = json.Marshal(gc.CommonComponentConfig)
	if err != nil {
		return []byte{}, err
	}

    if len(gc.Gateways) > 0 {
		gatewayData, err = json.Marshal(gc.Gateways)
		if err != nil {
			return []byte{}, err
		}
	} else {
		gatewayData = []byte("{}")
    }

    // assemble components
    commonLen := len(commonData)
    gatewayLen := len(gatewayData)
	data = make([]byte, 0, commonLen+gatewayLen)
	if commonLen > 2 {
        data = append(data, commonData[:commonLen-1]...)
	} else {
		data = append(data, '{')
	}
    if gatewayLen > 2 {
        if len(data) > 1 {
            data = append(data, byte(','))
        }
        data = append(data, gatewayData[1:]...)
    } else {
        data = append(data, '}')
    }
	return data, nil
}

// UnmarshalJSON treats Gateways as an embedded type (it can't be embedded because it's a map type)
func (gc *GatewaysConfig) UnmarshalJSON(data []byte) error {
    rawKeyedData := map[string]json.RawMessage{}
    err := json.Unmarshal(data, &rawKeyedData)
    if err != nil {
        return err
    }
    if value, ok := rawKeyedData["enabled"]; ok {
        err = json.Unmarshal(value, &gc.Enabled)
        if err != nil {
            return err
        }
        delete(rawKeyedData, "enabled")
    }
    if value, ok := rawKeyedData["global"]; ok {
        err = json.Unmarshal(value, &gc.Global)
        if err != nil {
            return err
        }
        delete(rawKeyedData, "nameOverride")
    }
    if value, ok := rawKeyedData["nameOverride"]; ok {
        err = json.Unmarshal(value, &gc.NameOverride)
        if err != nil {
            return err
        }
        delete(rawKeyedData, "fullnameOverride")
    }
    if value, ok := rawKeyedData["fullnameOverride"]; ok {
        err = json.Unmarshal(value, &gc.FullnameOverride)
        if err != nil {
            return err
        }
        delete(rawKeyedData, "fullnameOverride")
    }
    gc.Gateways = map[string]GatewayConfig{}
    for key, value := range rawKeyedData {
        g := GatewayConfig{}
        err = json.Unmarshal(value, &g)
        if err != nil {
            return err
        }
        gc.Gateways[key] = g
    }
	return nil
}