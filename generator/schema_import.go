package generator

import (
	"encoding/json"
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func QuerySchema(dataDir string, includeSystem bool) []*core.Collection {
	pb := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDev:     false,
		DefaultDataDir: dataDir,
	})
	if err := pb.Bootstrap(); err != nil {
		log.Fatal(err)
	}

	var collections []*core.Collection
	if err := pb.CollectionQuery().All(&collections); err != nil {
		log.Fatal(err)
	}

	if !includeSystem {
		filteredCollections := make([]*core.Collection, 0, len(collections))
		for _, c := range collections {
			if !c.System {
				filteredCollections = append(filteredCollections, c)
			}
		}
		return filteredCollections
	}

	return collections
}

func ParseSchemaJson(filepath string, includeSystem bool) []*core.Collection {
	rawJson, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatal(err)
	}

	data := []map[string]any{}
	err = json.Unmarshal(rawJson, &data)
	if err != nil {
		log.Fatalf("Error while parsing pocketbase schema json: %v", err)
	}

	collections := make([]*core.Collection, 0, len(data))
	for _, cData := range data {
		rawData, err := json.Marshal(cData)
		if err != nil {
			log.Fatalf("Error while parsing pocketbase schema json: %v", err)
		}
		collection := &core.Collection{}
		json.Unmarshal(rawData, collection)
		if !collection.System || includeSystem {
			collections = append(collections, collection)
		}
	}

	return collections
}
