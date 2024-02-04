package proxy

import (
	"log"

	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
)

var realtimeList map[string]*sdk.Stream[types.UserData]

func (p *Proxy) DBCreate(db string, userdata any) {
	_, err := p.grroxydb.Create(db, userdata)
	if err != nil {
		log.Print(err)
	}
	// log.Println("[Database] Added To Collection!!: ", userdata)
}

func (p *Proxy) DBNewSitemap(data types.SitemapGet) {

	log.Println("[DBNewSitemap] Sending Sitemap: ", data)

	err := p.grroxydb.SitemapNew(data)
	if err != nil {
		log.Print(err)
	}
	log.Println("[DBNewSitemap] Created: ", data)
}

func (p *Proxy) DBUpdate(db string, id string, userdata any) {
	err := p.grroxydb.Update(db, id, userdata)
	if err != nil {
		log.Print(err)
	}
	log.Println("[Database] Updated To Collection!!: ", id)
}

func (p *Proxy) DBSubscribe(endpoint string, db string, userdata *types.UserData) {
	collection := sdk.CollectionSet[types.UserData](p.grroxydb, db)

	var err error
	realtimeList["endpoint"], err = collection.Subscribe()

	if err != nil {
		log.Print(err)
	}
	// defer stream.Unsubscribe()
	<-realtimeList[endpoint].Ready()
	for ev := range realtimeList[endpoint].Events() {
		log.Print(ev.Action, ev.Record)
	}
}

func DBUnsubscribe(endpoint string, db string, userdata *types.UserData) {
	realtimeList[endpoint].Unsubscribe()
}
