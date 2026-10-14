package commands

import (
	"fmt"
	"strings"

	"github.com/resim-ai/api-client/api"
	"github.com/spf13/viper"
)

const cacheMethodKey = "cache-method"
const cacheMethodHelp = "Caching method: NONE, EBS, or EFS. Omit to use the organization default on creation or preserve the current method on update."

func cacheMethodFromFlags(checkLegacy bool) (*api.CacheMethod, error) {
	if !viper.IsSet(cacheMethodKey) {
		return nil, nil
	}
	method := api.CacheMethod(strings.ToUpper(viper.GetString(cacheMethodKey)))
	switch method {
	case api.NONE, api.EBS, api.EFS:
	default:
		return nil, fmt.Errorf("invalid --cache-method %q: expected NONE, EBS, or EFS", method)
	}
	if checkLegacy && viper.IsSet(assetCacheExemptKey) && viper.GetBool(assetCacheExemptKey) != (method == api.NONE) {
		return nil, fmt.Errorf("--cache-exempt conflicts with --cache-method %s", method)
	}
	return &method, nil
}
