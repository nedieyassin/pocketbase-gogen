package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func QuerySchema(dataDir string, includeSystem bool) ([]*core.Collection, error) {
	pb := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDev:     false,
		DefaultDataDir: dataDir,
	})
	if err := pb.Bootstrap(); err != nil {
		return nil, err
	}

	var collections []*core.Collection
	if err := pb.CollectionQuery().All(&collections); err != nil {
		return nil, err
	}

	if !includeSystem {
		filteredCollections := make([]*core.Collection, 0, len(collections))
		for _, c := range collections {
			if !c.System {
				filteredCollections = append(filteredCollections, c)
			}
		}
		return filteredCollections, nil
	}

	return collections, nil
}

func ParseSchemaJson(filepath string, includeSystem bool) ([]*core.Collection, error) {
	rawJson, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	data := []map[string]any{}
	err = json.Unmarshal(rawJson, &data)
	if err != nil {
		errMsg := fmt.Sprintf("Error while parsing pocketbase schema json: %v", err)
		return nil, errors.New(errMsg)
	}

	collections := make([]*core.Collection, 0, len(data))
	for _, cData := range data {
		rawData, err := json.Marshal(cData)
		if err != nil {
			errMsg := fmt.Sprintf("Error while parsing pocketbase schema json: %v", err)
			return nil, errors.New(errMsg)
		}
		collection := &core.Collection{}
		_ = json.Unmarshal(rawData, collection)
		if !collection.System || includeSystem {
			collections = append(collections, collection)
		}
	}

	return collections, nil
}
