package generator_test

import (
	"testing"

	. "github.com/nedieyassin/pocketbase-gogen/generator"
)

func TestProxyHooksGeneration(t *testing.T) {
	template := `type Proxy1 struct {
	// collection-name: collection_1
	// system: id
	id string
}

type NoCollectionNameProxy struct {
	// system: id
	id string
	other *Proxy1
}
`

	expectedGeneration := `type Proxy1Event = ProxyRecordEvent[Proxy1, *Proxy1]
type Proxy1EnrichEvent = ProxyRecordEnrichEvent[Proxy1, *Proxy1]
type Proxy1ErrorEvent = ProxyRecordErrorEvent[Proxy1, *Proxy1]
type Proxy1ListRequestEvent = ProxyRecordsListRequestEvent[Proxy1, *Proxy1]
type Proxy1RequestEvent = ProxyRecordRequestEvent[Proxy1, *Proxy1]

// This struct is a container for all proxy hooks.
// Use NewProxyHooks(app core.App) to create it once.
type ProxyHooks struct {
	OnProxy1Enrich             *hook.Hook[*Proxy1EnrichEvent]
	OnProxy1Validate           *hook.Hook[*Proxy1Event]
	OnProxy1Create             *hook.Hook[*Proxy1Event]
	OnProxy1CreateExecute      *hook.Hook[*Proxy1Event]
	OnProxy1AfterCreateSuccess *hook.Hook[*Proxy1Event]
	OnProxy1AfterCreateError   *hook.Hook[*Proxy1ErrorEvent]
	OnProxy1Update             *hook.Hook[*Proxy1Event]
	OnProxy1UpdateExecute      *hook.Hook[*Proxy1Event]
	OnProxy1AfterUpdateSuccess *hook.Hook[*Proxy1Event]
	OnProxy1AfterUpdateError   *hook.Hook[*Proxy1ErrorEvent]
	OnProxy1Delete             *hook.Hook[*Proxy1Event]
	OnProxy1DeleteExecute      *hook.Hook[*Proxy1Event]
	OnProxy1AfterDeleteSuccess *hook.Hook[*Proxy1Event]
	OnProxy1AfterDeleteError   *hook.Hook[*Proxy1ErrorEvent]
	OnProxy1ListRequest        *hook.Hook[*Proxy1ListRequestEvent]
	OnProxy1ViewRequest        *hook.Hook[*Proxy1RequestEvent]
	OnProxy1CreateRequest      *hook.Hook[*Proxy1RequestEvent]
	OnProxy1UpdateRequest      *hook.Hook[*Proxy1RequestEvent]
	OnProxy1DeleteRequest      *hook.Hook[*Proxy1RequestEvent]
}

// Create a new set of proxy hooks and register them
// on the given app. Keep in mind that calling this
// multiple times will result in multiple duplicate
// hooks being registered. So in general that should be
// avoided.
//
// Usage with an exemplary User proxy that has a name field:
//
//	pHooks := NewProxyHooks(app)
//	pHooks.OnUserCreate.BindFunc(func(e *UserEvent) error {
//		var user *User = e.PRecord // <-- Proxy events contain the proxy in the PRecord field
//		fmt.Printf("Hello new user, %v!", user.Name())
//		return e.Next()
//	})
func NewProxyHooks(app core.App) *ProxyHooks {
	pHooks := &ProxyHooks{
		OnProxy1Enrich:             &hook.Hook[*Proxy1EnrichEvent]{},
		OnProxy1Validate:           &hook.Hook[*Proxy1Event]{},
		OnProxy1Create:             &hook.Hook[*Proxy1Event]{},
		OnProxy1CreateExecute:      &hook.Hook[*Proxy1Event]{},
		OnProxy1AfterCreateSuccess: &hook.Hook[*Proxy1Event]{},
		OnProxy1AfterCreateError:   &hook.Hook[*Proxy1ErrorEvent]{},
		OnProxy1Update:             &hook.Hook[*Proxy1Event]{},
		OnProxy1UpdateExecute:      &hook.Hook[*Proxy1Event]{},
		OnProxy1AfterUpdateSuccess: &hook.Hook[*Proxy1Event]{},
		OnProxy1AfterUpdateError:   &hook.Hook[*Proxy1ErrorEvent]{},
		OnProxy1Delete:             &hook.Hook[*Proxy1Event]{},
		OnProxy1DeleteExecute:      &hook.Hook[*Proxy1Event]{},
		OnProxy1AfterDeleteSuccess: &hook.Hook[*Proxy1Event]{},
		OnProxy1AfterDeleteError:   &hook.Hook[*Proxy1ErrorEvent]{},
		OnProxy1ListRequest:        &hook.Hook[*Proxy1ListRequestEvent]{},
		OnProxy1ViewRequest:        &hook.Hook[*Proxy1RequestEvent]{},
		OnProxy1CreateRequest:      &hook.Hook[*Proxy1RequestEvent]{},
		OnProxy1UpdateRequest:      &hook.Hook[*Proxy1RequestEvent]{},
		OnProxy1DeleteRequest:      &hook.Hook[*Proxy1RequestEvent]{},
	}
	pHooks.registerProxyHooks(app)
	return pHooks
}

func (pHooks *ProxyHooks) registerProxyHooks(app core.App) {
	registerProxyEnrichEventHook(app.OnRecordEnrich("collection_1"), pHooks.OnProxy1Enrich)
	registerProxyEventHook(app.OnRecordValidate("collection_1"), pHooks.OnProxy1Validate)
	registerProxyEventHook(app.OnRecordCreate("collection_1"), pHooks.OnProxy1Create)
	registerProxyEventHook(app.OnRecordCreateExecute("collection_1"), pHooks.OnProxy1CreateExecute)
	registerProxyEventHook(app.OnRecordAfterCreateSuccess("collection_1"), pHooks.OnProxy1AfterCreateSuccess)
	registerProxyErrorEventHook(app.OnRecordAfterCreateError("collection_1"), pHooks.OnProxy1AfterCreateError)
	registerProxyEventHook(app.OnRecordUpdate("collection_1"), pHooks.OnProxy1Update)
	registerProxyEventHook(app.OnRecordUpdateExecute("collection_1"), pHooks.OnProxy1UpdateExecute)
	registerProxyEventHook(app.OnRecordAfterUpdateSuccess("collection_1"), pHooks.OnProxy1AfterUpdateSuccess)
	registerProxyErrorEventHook(app.OnRecordAfterUpdateError("collection_1"), pHooks.OnProxy1AfterUpdateError)
	registerProxyEventHook(app.OnRecordDelete("collection_1"), pHooks.OnProxy1Delete)
	registerProxyEventHook(app.OnRecordDeleteExecute("collection_1"), pHooks.OnProxy1DeleteExecute)
	registerProxyEventHook(app.OnRecordAfterDeleteSuccess("collection_1"), pHooks.OnProxy1AfterDeleteSuccess)
	registerProxyErrorEventHook(app.OnRecordAfterDeleteError("collection_1"), pHooks.OnProxy1AfterDeleteError)
	registerProxyListRequestEventHook(app.OnRecordsListRequest("collection_1"), pHooks.OnProxy1ListRequest)
	registerProxyRequestEventHook(app.OnRecordViewRequest("collection_1"), pHooks.OnProxy1ViewRequest)
	registerProxyRequestEventHook(app.OnRecordCreateRequest("collection_1"), pHooks.OnProxy1CreateRequest)
	registerProxyRequestEventHook(app.OnRecordUpdateRequest("collection_1"), pHooks.OnProxy1UpdateRequest)
	registerProxyRequestEventHook(app.OnRecordDeleteRequest("collection_1"), pHooks.OnProxy1DeleteRequest)
}
`

	ok, err := expectGeneratedHooks(template, expectedGeneration)
	if err != nil {
		t.Fatal("error during generation of hooks")
	}
	if !ok {
		t.Fatal("the template did not have the expected hooks generation result")
	}
}

func expectGeneratedHooks(input, expectedOutput string) (bool, error) {
	input = addBoilerplate(input)

	parser, err := NewTemplateParser([]byte(input))
	if err != nil {
		return false, err
	}
	outBytes, err := GenerateProxyHooks(parser, ".", "test")
	if err != nil {
		return false, err
	}

	output := removeBoilerplate(outBytes)

	return output == expectedOutput, nil
}
