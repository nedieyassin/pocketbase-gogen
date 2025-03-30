package generator

// DO NOT EDIT
// (unless you know what you are doing of course)
var proxyHooksTemplateCode = 
`package template

// 0-4: Event type aliases (for readability)
type Event = ProxyRecordEvent[StructName, *StructName]
type EnrichEvent = ProxyRecordEnrichEvent[StructName, *StructName]
type ErrorEvent = ProxyRecordErrorEvent[StructName, *StructName]
type ListRequestEvent = ProxyRecordsListRequestEvent[StructName, *StructName]
type RequestEvent = ProxyRecordRequestEvent[StructName, *StructName]

// 5: Hook container struct
type proxyHooks struct {
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

// 6: Hook struct constructor
func NewProxyHooks(app core.App) *proxyHooks {
	pHooks := &proxyHooks{
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

// 7: Registering function
func (pHooks *proxyHooks) registerProxyHooks(app core.App) {
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
