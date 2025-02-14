package generator_test

import (
	"testing"

	. "github.com/snonky/pocketbase-gogen/generator"
)

func TestUtilsGeneration(t *testing.T) {
	template := `type Proxy1 struct {
	// collection-name: collection_1
	// system: id
	id string
}

type Proxy2 struct {
	// collection-name: collection_2
	// system: id
	id     string
	other1 *Proxy1
}

type Proxy3 struct {
	// collection-name: collection_3
	// system: id
	id      string
	others2 []*Proxy2
}

type Proxy4 struct {
	// collection-name: collection_4
	// system: id
	id      string
	other2  *Proxy2
	others3 []*Proxy3
	selfs   []*Proxy4
	self    *Proxy4
	noName  *NoCollectionNameProxy
}

type NoCollectionNameProxy struct {
	// system: id
	id string
	other4 *Proxy4
}
`

	expectedGeneration := `type Proxy interface {
	Proxy1 | Proxy2 | Proxy3 | Proxy4 | NoCollectionNameProxy
}

// This interface constrains a type parameter of
// a Proxy core type into its pointer type.
// This grants generic functions access to the
// core and the pointer type at the same time.
//
// Such a generic function looks like this:
//
//	func MyFunc[P Proxy, PP ProxyP[P]]() {
//	  // The function can create the zero-value
//	  // of P and knows its pointer type PP!
//	  var _ PP = &P{}
//	}
//
// The PP parameter can be inferred by the
// compiler so this convenient call becomes
// possible:
//
//	MyFunc[ProxyType]()
//
// The opposite inference direction is also
// possible:
//
//	func MyFunc[PP ProxyP[P], P Proxy]()
//
// Can be called like this:
//
//	MyFunc[*ProxyType]()
//
// And even works with other type paramters:
//
//	func MyFunc2[P Proxy, PP ProxyP[P]]() {
//	    MyFunc[PP]()
//	}
type ProxyP[P Proxy] interface {
	*P
	core.RecordProxy
	CollectionName() string
}

// Returns the collection name of a proxy type
//
//	collectionName := CName[ProxyType]()
func CName[P Proxy, PP ProxyP[P]]() string {
	return PP.CollectionName(nil)
}

// Creates a new record and wraps it in a new proxy
//
//	proxy := NewProxy[ProxyType](app)
func NewProxy[P Proxy, PP ProxyP[P]](app core.App) (PP, error) {
	var p PP = &P{}
	collectionName := p.CollectionName()
	collection, err := app.FindCachedCollectionByNameOrId(collectionName)
	if err != nil {
		return nil, err
	}
	record := core.NewRecord(collection)
	p.SetProxyRecord(record)
	return p, nil
}

// Wraps a record in a newly created proxy
//
//	proxy := WrapProxy[ProxyType](record)
func WrapRecord[P Proxy, PP ProxyP[P]](record *core.Record) (PP, error) {
	collectionName := record.Collection().Name
	proxyCollectionName := PP.CollectionName(nil)
	if collectionName != proxyCollectionName {
		return nil, errors.New("the generic proxy type is not of the same collection as the given record")
	}
	var p PP = &P{}
	p.SetProxyRecord(record)
	return p, nil
}

type RelationField struct {
	FieldName string
	IsMulti   bool
}

// This map contains all relations between the collections that
// have a proxy struct with a CollectionName() method.
// It maps like this:
//
//	collection name
//	 -> collection names that it is related to
//	  -> list of fields that contain the relation values
var Relations = map[string]map[string][]RelationField{
	"collection_2": {
		"collection_1": {
			{"other1", false},
		},
	},
	"collection_3": {
		"collection_2": {
			{"others2", true},
		},
	},
	"collection_4": {
		"collection_2": {
			{"other2", false},
		},
		"collection_3": {
			{"others3", true},
		},
		"collection_4": {
			{"selfs", true},
			{"self", false},
		},
	},
}
`

	equal, err := expectGeneratedUtils(template, expectedGeneration)
	if err != nil {
		t.Fatal("error during generation if utils")
	}
	if !equal {
		t.Fatal("the template did not have the expected util generation result")
	}
}

func expectGeneratedUtils(input, expectedOutput string) (bool, error) {
	input = addBoilerplate(input)

	parser, err := NewTemplateParser([]byte(input))
	if err != nil {
		return false, err
	}
	outBytes, err := GenerateUtils(parser, ".", "test")
	if err != nil {
		return false, err
	}

	output := removeBoilerplate(outBytes)

	return output == expectedOutput, nil
}
