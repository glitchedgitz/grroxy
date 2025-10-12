package proxy

import (
	"github.com/glitchedgitz/grroxy-db/utils"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
)

var requestRateLimit = make(chan (int), 500)
var generateIndex = make(chan (float64))
var total = 0

func (p *Proxy) RateLimitManager() {

	collection := sdk.CollectionSet[types.RealtimeRecord](p.grroxydb, "_data")

	result, err := collection.List(types.ParamsList{
		Page: 1, Size: 1,
	})

	total = result.TotalItems

	utils.CheckErr("", err)

	for {
		<-requestRateLimit
		total += 1
		generateIndex <- float64(total)
	}
}
