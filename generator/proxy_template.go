package generator

// DO NOT EDIT
// (unless you know what you are doing of course)
//
// If you know what you are doing, note this:
// The function loadTemplateASTs parses this syntactically correct
// code and saves the declaration AST nodes as templates.
// The function adaptFuncTemplate searches and replaces the
// relevant identifiers in the templates to create the actual
// source code.
var proxyTemplateCode =
`package template

// 0: Proxy struct declaration
type StructName struct {
	core.BaseRecordProxy
}

// 1: Primitive getter declaration
func (p *StructName) FuncName() FieldType {
	return p.GetterFuncName("key")
}

// 2: Single relation getter declaration
func (p *StructName) FuncName() *FieldType {
	var proxy *FieldType
	if rel := p.ExpandedOne("key"); rel != nil {
		proxy = &FieldType{}
		proxy.Record = rel
	}
	return proxy
}

// 3: Multi relation getter declaration
func (p *StructName) FuncName() []*FieldType {
	rels := p.ExpandedAll("key")
	proxies := make([]*FieldType, len(rels))
	for i := range len(rels) {
		proxies[i] = &FieldType{}
		proxies[i].Record = rels[i]
	}
	return proxies
}

// 4: Single select getter declaration
func (p *StructName) FuncName() FieldType {
	option := p.GetString("key")
	i, ok := selectNameMap[option]
	if !ok {
		panic("Unknown select value")
	}
	return i
}

// 5: Multi select getter declaration
func (p *StructName) FuncName() []FieldType {
	options := p.GetStringSlice("key")
	is := make([]FieldType, 0, len(options))
	for _, o := range options {
		i, ok := selectNameMap[o]
		if !ok {
			panic("Unknown select value")
		} 
		is = append(is, i)
	}
	return is
}

// 6: Primitive setter declaration
func (p *StructName) FuncName(fieldName FieldType) {
	p.Set("key", fieldName)
}

// 7: Single relation setter declaration
func (p *StructName) FuncName(fieldName *FieldType) {
	p.Record.Set("key", fieldName.Id) 
	e := p.Expand()
	e["key"] = fieldName.Record
	p.SetExpand(e)
}

// 8: Multi relation setter declaration
func (p *StructName) FuncName(fieldName []*FieldType) {
	records := make([]*core.Record, len(fieldName))
	ids := make([]string, len(fieldName))
	for i, r := range fieldName {
		records[i] = r.Record
		ids[i] = r.Record.Id
	}
	p.Record.Set("key", ids)
	e := p.Expand()
	e["key"] = records
	p.SetExpand(e)
}

// 9: Single select setter declaration
func (p *StructName) FuncName(fieldName FieldType) {
	i, ok := selectIotaMap[fieldName]
	if !ok {
		panic("Unknown select value")
	}
	p.Set("key", i)
}

// 10: Multi select setter declaration
func (p *StructName) FuncName(fieldName []FieldType) {
	is := make([]string, 0, len(fieldName))
	for _, s := range fieldName {
		i, ok := selectIotaMap[s]
		if !ok {
			panic("Unknown select value")
		}
		is = append(is, i)
	}
	p.Set("key", is)
}

// 11: Collection name getter
func (p *StructName) FuncName() string {
	return "key"
}
`
