package generator

// DO NOT EDIT
// (unless you know what you are doing of course)
var utilTemplateCode = `package template

type Proxy interface {}

// This interface constrains a type parameter of
// a Proxy core type into its pointer type.
// This grants generic functions access to the
// core and the pointer type at the same time.
//
// Such a generic function looks like this:
//
//  func MyFunc[P Proxy, PP ProxyP[P]]() {
//    // The function can create the zero-value
//    // of P and knows its pointer type PP!
//    var _ PP = &P{}
//  }
//
// The PP parameter can be inferred by the
// compiler so this convenient call becomes
// possible:
//
//  MyFunc[ProxyType]()
//
// The opposite inference direction is also
// possible:
//
//  func MyFunc[PP ProxyP[P], P Proxy]()
//
// Can be called like this:
//
//  MyFunc[*ProxyType]()
//
// And even works with other type parameters:
//
//  func MyFunc2[P Proxy, PP ProxyP[P]]() {
//      MyFunc[PP]()
//  }
type ProxyP[P Proxy] interface {
	*P
	core.RecordProxy
	CollectionName() string
}

// Returns the collection name of a proxy type
//
//  collectionName := CName[ProxyType]()
func CollectionNameOf[P Proxy, PP ProxyP[P]]() string {
	return PP.CollectionName(nil)
}

// Creates a new record and wraps it in a new proxy
//
//  proxy := NewProxy[ProxyType](app)
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
//  proxy := WrapRecord[ProxyType](record)
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

func WrapRecords[P Proxy, PP ProxyP[P]](records []*core.Record) ([]PP, error) {
	var ms []PP
	for _, record := range records {
		collectionName := record.Collection().Name
		proxyCollectionName := PP.CollectionName(nil)
		if collectionName != proxyCollectionName {
			return nil, errors.New("the generic proxy type is not of the same collection as the given record")
		}
		var p PP = &P{}
		p.SetProxyRecord(record)
		ms = append(ms, p)
	}

	return ms, nil
}

type RelationField struct {
	FieldName string
	IsMulti   bool
}

// This map contains all relations between the collections that
// have a proxy struct with a CollectionName() method.
// It maps like this:
//
//  collection name
//   -> collection names that it is related to
//    -> list of fields that contain the relation values
var Relations = map[string]map[string][]RelationField{}
`
