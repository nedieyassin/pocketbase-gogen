package generator

// DO NOT EDIT
// (unless you know what you are doing of course)
var proxyEventsTemplateCode = 
`package template

type baseProxyEventData[P Proxy, PP ProxyP[P]] struct {
	PRecord PP
}

type ProxyRecordEvent[P Proxy, PP ProxyP[P]] struct {
	hook.Event
	App core.App
	baseProxyEventData[P, PP]
	Context context.Context
	// create, update, delete or validate
	Type string
}

type ProxyRecordErrorEvent[P Proxy, PP ProxyP[P]] struct {
	Error error
	ProxyRecordEvent[P, PP]
}

type ProxyRecordEnrichEvent[P Proxy, PP ProxyP[P]] struct {
	hook.Event
	App core.App
	baseProxyEventData[P, PP]
	RequestInfo *core.RequestInfo
}

type ProxyRecordRequestEvent[P Proxy, PP ProxyP[P]] struct {
	hook.Event
	*core.RequestEvent
	Collection *core.Collection
	baseProxyEventData[P, PP]
}

type ProxyRecordsListRequestEvent[P Proxy, PP ProxyP[P]] struct {
	hook.Event
	*core.RequestEvent
	Collection *core.Collection
	PRecords   []PP
	Result     *search.Result
}

func syncProxyEventWithRecordEvent[P Proxy, PP ProxyP[P]](pe *ProxyRecordEvent[P, PP], re *core.RecordEvent) {
	pe.App = re.App
	pe.Context = re.Context
	pe.Type = re.Type
	pe.PRecord.SetProxyRecord(re.Record)
}

func syncRecordEventWithProxyEvent[P Proxy, PP ProxyP[P]](re *core.RecordEvent, pe *ProxyRecordEvent[P, PP]) {
	re.App = pe.App
	re.Context = pe.Context
	re.Type = pe.Type
	re.Record = pe.PRecord.ProxyRecord()
}

func newProxyEventFromRecordEvent[P Proxy, PP ProxyP[P]](re *core.RecordEvent) *ProxyRecordEvent[P, PP] {
	pe := &ProxyRecordEvent[P, PP]{}

	pe.App = re.App
	pe.Context = re.Context
	pe.Type = re.Type
	pe.PRecord, _ = WrapRecord[P, PP](re.Record)

	return pe
}

func syncProxyErrorEventWithRecordErrorEvent[P Proxy, PP ProxyP[P]](pe *ProxyRecordErrorEvent[P, PP], re *core.RecordErrorEvent) {
	syncProxyEventWithRecordEvent(&pe.ProxyRecordEvent, &re.RecordEvent)
	pe.Error = re.Error
}

func syncRecordErrorEventWithProxyErrorEvent[P Proxy, PP ProxyP[P]](re *core.RecordErrorEvent, pe *ProxyRecordErrorEvent[P, PP]) {
	syncRecordEventWithProxyEvent(&re.RecordEvent, &pe.ProxyRecordEvent)
	re.Error = pe.Error
}

func newProxyErrorEventFromRecordErrorEvent[P Proxy, PP ProxyP[P]](re *core.RecordErrorEvent) *ProxyRecordErrorEvent[P, PP] {
	proxyRecordEvent := newProxyEventFromRecordEvent[P, PP](&re.RecordEvent)

	pe := &ProxyRecordErrorEvent[P, PP]{}
	pe.ProxyRecordEvent = *proxyRecordEvent
	pe.Error = re.Error

	return pe
}

func syncProxyEnrichEventWithRecordEnrichEvent[P Proxy, PP ProxyP[P]](pe *ProxyRecordEnrichEvent[P, PP], re *core.RecordEnrichEvent) {
	pe.App = re.App
	pe.PRecord.SetProxyRecord(re.Record)
}

func syncRecordEnrichEventWithProxyEnrichEvent[P Proxy, PP ProxyP[P]](re *core.RecordEnrichEvent, pe *ProxyRecordEnrichEvent[P, PP]) {
	re.App = pe.App
	re.Record = pe.PRecord.ProxyRecord()
}

func newProxyEnrichEventFromRecordEnrichEvent[P Proxy, PP ProxyP[P]](re *core.RecordEnrichEvent) *ProxyRecordEnrichEvent[P, PP] {
	pe := &ProxyRecordEnrichEvent[P, PP]{}

	pe.App = re.App
	pe.RequestInfo = re.RequestInfo
	pe.PRecord, _ = WrapRecord[P, PP](re.Record)

	return pe
}

func syncProxyRequestEventWithRecordRequestEvent[P Proxy, PP ProxyP[P]](pe *ProxyRecordRequestEvent[P, PP], re *core.RecordRequestEvent) {
	pe.App = re.App
}

func syncRecordRequestEventWithProxyRequestEvent[P Proxy, PP ProxyP[P]](re *core.RecordRequestEvent, pe *ProxyRecordRequestEvent[P, PP]) {
	re.App = pe.App
}

func newProxyRequestEventFromRecordRequestEvent[P Proxy, PP ProxyP[P]](re *core.RecordRequestEvent) *ProxyRecordRequestEvent[P, PP] {
	pe := &ProxyRecordRequestEvent[P, PP]{}

	pe.RequestEvent = re.RequestEvent
	pe.Collection = re.Collection
	pe.PRecord, _ = WrapRecord[P, PP](re.Record)

	return pe
}

func syncProxyListRequestEventWithRecordListRequestEvent[P Proxy, PP ProxyP[P]](pe *ProxyRecordsListRequestEvent[P, PP], re *core.RecordsListRequestEvent) {
	pe.App = re.App
}

func syncRecordListRequestEventWithProxyListRequestEvent[P Proxy, PP ProxyP[P]](re *core.RecordsListRequestEvent, pe *ProxyRecordsListRequestEvent[P, PP]) {
	re.App = pe.App
}

func newProxyListRequestEventFromRecordListRequestEvent[P Proxy, PP ProxyP[P]](re *core.RecordsListRequestEvent) *ProxyRecordsListRequestEvent[P, PP] {
	pe := &ProxyRecordsListRequestEvent[P, PP]{}

	pe.RequestEvent = re.RequestEvent
	pe.Collection = re.Collection
	pe.PRecords = make([]PP, len(re.Records))
	for i, r := range re.Records {
		pe.PRecords[i], _ = WrapRecord[P, PP](r)
	}

	return pe
}

func registerProxyEnrichEventHook[P Proxy, PP ProxyP[P]](
	recordHook *hook.TaggedHook[*core.RecordEnrichEvent],
	proxyHook *hook.Hook[*ProxyRecordEnrichEvent[P, PP]],
) {
	recordHook.Bind(&hook.Handler[*core.RecordEnrichEvent]{
		Func: func(re *core.RecordEnrichEvent) error {
			pe := newProxyEnrichEventFromRecordEnrichEvent[P, PP](re)
			err := proxyHook.Trigger(pe,
				func(pe *ProxyRecordEnrichEvent[P, PP]) error {
					syncRecordEnrichEventWithProxyEnrichEvent(re, pe)
					defer syncProxyEnrichEventWithRecordEnrichEvent(pe, re)
					return re.Next()
				},
			)
			syncRecordEnrichEventWithProxyEnrichEvent(re, pe)
			return err
		},
		Priority: -99,
	})
}

func registerProxyEventHook[P Proxy, PP ProxyP[P]](
	recordHook *hook.TaggedHook[*core.RecordEvent],
	proxyHook *hook.Hook[*ProxyRecordEvent[P, PP]],
) {
	recordHook.Bind(&hook.Handler[*core.RecordEvent]{
		Func: func(re *core.RecordEvent) error {
			pe := newProxyEventFromRecordEvent[P, PP](re)
			err := proxyHook.Trigger(pe,
				func(pe *ProxyRecordEvent[P, PP]) error {
					syncRecordEventWithProxyEvent(re, pe)
					defer syncProxyEventWithRecordEvent(pe, re)
					return re.Next()
				},
			)
			syncRecordEventWithProxyEvent(re, pe)
			return err
		},
		Priority: -99,
	})
}

func registerProxyErrorEventHook[P Proxy, PP ProxyP[P]](
	recordHook *hook.TaggedHook[*core.RecordErrorEvent],
	proxyHook *hook.Hook[*ProxyRecordErrorEvent[P, PP]],
) {
	recordHook.Bind(&hook.Handler[*core.RecordErrorEvent]{
		Func: func(re *core.RecordErrorEvent) error {
			pe := newProxyErrorEventFromRecordErrorEvent[P, PP](re)
			err := proxyHook.Trigger(pe,
				func(pe *ProxyRecordErrorEvent[P, PP]) error {
					syncRecordErrorEventWithProxyErrorEvent(re, pe)
					defer syncProxyErrorEventWithRecordErrorEvent(pe, re)
					return re.Next()
				},
			)
			syncRecordErrorEventWithProxyErrorEvent(re, pe)
			return err
		},
		Priority: -99,
	})
}

func registerProxyListRequestEventHook[P Proxy, PP ProxyP[P]](
	recordHook *hook.TaggedHook[*core.RecordsListRequestEvent],
	proxyHook *hook.Hook[*ProxyRecordsListRequestEvent[P, PP]],
) {
	recordHook.Bind(&hook.Handler[*core.RecordsListRequestEvent]{
		Func: func(re *core.RecordsListRequestEvent) error {
			pe := newProxyListRequestEventFromRecordListRequestEvent[P, PP](re)
			err := proxyHook.Trigger(pe,
				func(pe *ProxyRecordsListRequestEvent[P, PP]) error {
					syncRecordListRequestEventWithProxyListRequestEvent(re, pe)
					defer syncProxyListRequestEventWithRecordListRequestEvent(pe, re)
					return re.Next()
				},
			)
			syncRecordListRequestEventWithProxyListRequestEvent(re, pe)
			return err
		},
		Priority: -99,
	})
}

func registerProxyRequestEventHook[P Proxy, PP ProxyP[P]](
	recordHook *hook.TaggedHook[*core.RecordRequestEvent],
	proxyHook *hook.Hook[*ProxyRecordRequestEvent[P, PP]],
) {
	recordHook.Bind(&hook.Handler[*core.RecordRequestEvent]{
		Func: func(re *core.RecordRequestEvent) error {
			pe := newProxyRequestEventFromRecordRequestEvent[P, PP](re)
			err := proxyHook.Trigger(pe,
				func(pe *ProxyRecordRequestEvent[P, PP]) error {
					syncRecordRequestEventWithProxyRequestEvent(re, pe)
					defer syncProxyRequestEventWithRecordRequestEvent(pe, re)
					return re.Next()
				},
			)
			syncRecordRequestEventWithProxyRequestEvent(re, pe)
			return err
		},
		Priority: -99,
	})
}
`
