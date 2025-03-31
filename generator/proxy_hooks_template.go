package generator

// DO NOT EDIT
// (unless you know what you are doing of course)
var proxyHooksTemplateCode = 
`package template

type Event = ProxyRecordEvent[StructName, *StructName]
type EnrichEvent = ProxyRecordEnrichEvent[StructName, *StructName]
type ErrorEvent = ProxyRecordErrorEvent[StructName, *StructName]
type ListRequestEvent = ProxyRecordsListRequestEvent[StructName, *StructName]
type RequestEvent = ProxyRecordRequestEvent[StructName, *StructName]

// This struct is a container for all proxy hooks.
// Use NewProxyHooks(app core.App) to create it once.
type ProxyHooks struct {
	Enrich   *hook.Hook[*EnrichEvent]
	Validate *hook.Hook[*Event]

	Create             *hook.Hook[*Event]
	CreateExecute      *hook.Hook[*Event]
	AfterCreateSuccess *hook.Hook[*Event]
	AfterCreateError   *hook.Hook[*ErrorEvent]

	Update             *hook.Hook[*Event]
	UpdateExecute      *hook.Hook[*Event]
	AfterUpdateSuccess *hook.Hook[*Event]
	AfterUpdateError   *hook.Hook[*ErrorEvent]

	Delete             *hook.Hook[*Event]
	DeleteExecute      *hook.Hook[*Event]
	AfterDeleteSuccess *hook.Hook[*Event]
	AfterDeleteError   *hook.Hook[*ErrorEvent]

	ListRequest   *hook.Hook[*ListRequestEvent]
	ViewRequest   *hook.Hook[*RequestEvent]
	CreateRequest *hook.Hook[*RequestEvent]
	UpdateRequest *hook.Hook[*RequestEvent]
	DeleteRequest *hook.Hook[*RequestEvent]
}

// Create a new set of proxy hooks and register them
// on the given app. Keep in mind that calling this
// multiple times will result in multiple duplicate
// hooks being registered. So in general that should be
// avoided.
//
// Usage with an exemplary User proxy that has a name field:
// 	pHooks := NewProxyHooks(app)
// 	pHooks.OnUserCreate.BindFunc(func(e *UserEvent) error {
// 		var user *User = e.PRecord // <-- Proxy events contain the proxy in the PRecord field
// 		fmt.Printf("Hello new user, %v!", user.Name())	
// 		return e.Next()
// 	})
func NewProxyHooks(app core.App) *ProxyHooks {
	pHooks := &ProxyHooks{
		Enrich:             &hook.Hook[*EnrichEvent]{},
		Validate:           &hook.Hook[*Event]{},
		Create:             &hook.Hook[*Event]{},
		CreateExecute:      &hook.Hook[*Event]{},
		AfterCreateSuccess: &hook.Hook[*Event]{},
		AfterCreateError:   &hook.Hook[*ErrorEvent]{},
		Update:             &hook.Hook[*Event]{},
		UpdateExecute:      &hook.Hook[*Event]{},
		AfterUpdateSuccess: &hook.Hook[*Event]{},
		AfterUpdateError:   &hook.Hook[*ErrorEvent]{},
		Delete:             &hook.Hook[*Event]{},
		DeleteExecute:      &hook.Hook[*Event]{},
		AfterDeleteSuccess: &hook.Hook[*Event]{},
		AfterDeleteError:   &hook.Hook[*ErrorEvent]{},
		ListRequest:        &hook.Hook[*ListRequestEvent]{},
		ViewRequest:        &hook.Hook[*RequestEvent]{},
		CreateRequest:      &hook.Hook[*RequestEvent]{},
		UpdateRequest:      &hook.Hook[*RequestEvent]{},
		DeleteRequest:      &hook.Hook[*RequestEvent]{},
	}
	pHooks.registerProxyHooks(app)
	return pHooks
}

func (pHooks *ProxyHooks) registerProxyHooks(app core.App) {
	registerProxyEnrichEventHook(
		app.OnRecordEnrich("collection_name"),
		pHooks.Enrich,
	)

	registerProxyEventHook(
		app.OnRecordValidate("collection_name"),
		pHooks.Validate,
	)

	registerProxyEventHook(
		app.OnRecordCreate("collection_name"),
		pHooks.Create,
	)
	registerProxyEventHook(
		app.OnRecordCreateExecute("collection_name"),
		pHooks.CreateExecute,
	)
	registerProxyEventHook(
		app.OnRecordAfterCreateSuccess("collection_name"),
		pHooks.AfterCreateSuccess,
	)
	registerProxyErrorEventHook(
		app.OnRecordAfterCreateError("collection_name"),
		pHooks.AfterCreateError,
	)

	registerProxyEventHook(
		app.OnRecordUpdate("collection_name"),
		pHooks.Update,
	)
	registerProxyEventHook(
		app.OnRecordUpdateExecute("collection_name"),
		pHooks.UpdateExecute,
	)
	registerProxyEventHook(
		app.OnRecordAfterUpdateSuccess("collection_name"),
		pHooks.AfterUpdateSuccess,
	)
	registerProxyErrorEventHook(
		app.OnRecordAfterUpdateError("collection_name"),
		pHooks.AfterUpdateError,
	)

	registerProxyEventHook(
		app.OnRecordDelete("collection_name"),
		pHooks.Delete,
	)
	registerProxyEventHook(
		app.OnRecordDeleteExecute("collection_name"),
		pHooks.DeleteExecute,
	)
	registerProxyEventHook(
		app.OnRecordAfterDeleteSuccess("collection_name"),
		pHooks.AfterDeleteSuccess,
	)
	registerProxyErrorEventHook(
		app.OnRecordAfterDeleteError("collection_name"),
		pHooks.AfterDeleteError,
	)

	registerProxyListRequestEventHook(
		app.OnRecordsListRequest("collection_name"),
		pHooks.ListRequest,
	)
	registerProxyRequestEventHook(
		app.OnRecordViewRequest("collection_name"),
		pHooks.ViewRequest,
	)
	registerProxyRequestEventHook(
		app.OnRecordCreateRequest("collection_name"),
		pHooks.CreateRequest,
	)
	registerProxyRequestEventHook(
		app.OnRecordUpdateRequest("collection_name"),
		pHooks.UpdateRequest,
	)
	registerProxyRequestEventHook(
		app.OnRecordDeleteRequest("collection_name"),
		pHooks.DeleteRequest,
	)
}
`
