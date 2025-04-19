package subsite

import (
	"encoding/json"
)

type CacheConfig struct {
}

func GetCacheConfig(cacheConfigInterface interface{}) (*CacheConfig, error) {
	var cacheConfig CacheConfig
	if cacheConfigInterface == nil {
		cacheConfigInterface = "{}"
	}
	actionReqStr := cacheConfigInterface.(string)
	if len(actionReqStr) == 0 {
		actionReqStr = "{}"
	}
	err := json.Unmarshal([]byte(actionReqStr), &cacheConfig)

	if err != nil {
		return nil, err
	}
	return &cacheConfig, nil
}
