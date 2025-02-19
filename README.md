# PocketBase Type-Safe Go Code Generator

A tool for developers using [PocketBase](https://github.com/pocketbase/pocketbase) as a go backend framework.

Convert your PocketBase schema into type-safe accessor structs ("DAOs", "Proxies") for your code base.

Also supports custom methods that work with the typed data.

## Usage

### Step 0: Install `pocketbase-gogen`
```console
go install github.com/snonky/pocketbase-gogen@latest
```
You should now have access to the CLI with the `pocketbase-gogen` command.

### Step 1: Generate a template
```console
pocketbase-gogen template ./path/to/pb_data ./yourmodule/pbschema/template.go
```

You will now have `template.go` which is your "schema as code" and contains your PB schema as a set of go structs. The file contains a doc comment with a detailed description of what you can do to customize it.

### Step 2: Generate proxies from template

```console
pocketbase-gogen generate ./yourmodule/pbschema/template.go ./yourmodule/generated/proxies.go
```

You will now have `proxies.go` which contains the actual proxy structs
that you can drop in to use instead of raw `core.Record` structs from PocketBase ([PB doc on proxy usage](https://pocketbase.io/docs/go-record-proxy/)).

Optionally append the `--utils` flag. The [example](#generate-utilsgo) shows what the result of that is.

> [!IMPORTANT]
> Do not run the generator against a production data base file.
> As with any code, please always test your generated code before putting it to use.
# Example

For the example we have a simple PB schema of 3 collections `person`, `child` and `bank_account`. They look like this:

<p align="center">
        <img src="https://i.imgur.com/yXYQtAD.png" alt="Example Schema Screenshot" />
</p>

### Running `pocketbase-gogen template` we get this:

```go
type BankAccount struct {
	// collection-name: bank_account
	// system: id
	Id    string
	money int
}

type Child struct {
	// collection-name: child
	// system: id
	Id   string
	name string
	age  int
}

type Person struct {
	// collection-name: person
	// system: id
	Id      string
	name    string
	address string
	// select: HealthSelectType(good, medium, bad)
	health   int
	account  *BankAccount
	children []*Child
}
```

Things to note about the template:

 - The `Id` field is marked as a system field and thus will not have getters/setters generated.
 - The `health` field is a PocketBase select type field which is always represented as an `int` (or `[]int` for multi select types) in the template. The field comment marks this fact.
 - The relation type fields `account` and `children` are already typed with the other template structs.
 - All template structs have a `// collection-name:` comment on their first field that stores the original collection name.

 ### Now running `pocketbase-gogen generate`

 The generated code for the three template structs comes out as this (they are in one file but for the example they are shown separately):

 ### Bank Account
```go
type BankAccount struct {
	core.BaseRecordProxy
}

func (p *BankAccount) CollectionName() string {
	return "bank_account"
}

func (p *BankAccount) Money() int {
	return p.GetInt("money")
}

func (p *BankAccount) SetMoney(money int) {
	p.Set("money", money)
}
```

### Child

```go
type Child struct {
	core.BaseRecordProxy
}

func (p *Child) CollectionName() string {
	return "child"
}

func (p *Child) Name() string {
	return p.GetString("name")
}

func (p *Child) SetName(name string) {
	p.Set("name", name)
}

func (p *Child) Age() int {
	return p.GetInt("age")
}

func (p *Child) SetAge(age int) {
	p.Set("age", age)
}
```

### Person 

```go
type HealthSelectType int

const (
	Good HealthSelectType = iota
	Medium
	Bad
)

var zzHealthSelectTypeSelectNameMap = map[string]HealthSelectType{
	"good":   0,
	"medium": 1,
	"bad":    2,
}
var zzHealthSelectTypeSelectIotaMap = map[HealthSelectType]string{
	0: "good",
	1: "medium",
	2: "bad",
}

type Person struct {
	core.BaseRecordProxy
}

func (p *Person) CollectionName() string {
	return "person"
}

func (p *Person) Name() string {
	return p.GetString("name")
}

func (p *Person) SetName(name string) {
	p.Set("name", name)
}

func (p *Person) Address() string {
	return p.GetString("address")
}

func (p *Person) SetAddress(address string) {
	p.Set("address", address)
}

func (p *Person) Health() HealthSelectType {
	option := p.GetString("health")
	i, ok := zzHealthSelectTypeSelectNameMap[option]
	if !ok {
		panic("Unknown select value")
	}
	return i
}

func (p *Person) SetHealth(health HealthSelectType) {
	i, ok := zzHealthSelectTypeSelectIotaMap[health]
	if !ok {
		panic("Unknown select value")
	}
	p.Set("health", i)
}

func (p *Person) Account() *BankAccount {
	var proxy *BankAccount
	if rel := p.ExpandedOne("account"); rel != nil {
		proxy = &BankAccount{}
		proxy.Record = rel
	}
	return proxy
}

func (p *Person) SetAccount(account *BankAccount) {
	p.Record.Set("account", account.Id)
	e := p.Expand()
	e["account"] = account.Record
	p.SetExpand(e)
}

func (p *Person) Children() []*Child {
	rels := p.ExpandedAll("children")
	proxies := make([]*Child, len(rels))
	for i := range len(rels) {
		proxies[i] = &Child{}
		proxies[i].Record = rels[i]
	}
	return proxies
}

func (p *Person) SetChildren(children []*Child) {
	records := make([]*core.Record, len(children))
	ids := make([]string, len(children))
	for i, r := range children {
		records[i] = r.Record
		ids[i] = r.Record.Id
	}
	p.Record.Set("children", ids)
	e := p.Expand()
	e["children"] = records
	p.SetExpand(e)
}
```

Every proxy has getters and setters generated for its fields plus a `CollectionName()` convenience method.
`CollectionName()` is generated from the `// collection-name:` comment in the template.

Bank Account and Child are straightforward. Things of interest about the Person generation:

 - The `health` select type got its own go type of 
    ```go
    type HealthSelectType int
    ```
    with an accompanying `const` declaration:
    ```go
    const (
        Good HealthSelectType = iota
        Medium
        Bad
    )
    ```
    The constants guarantee correct select options in your application code.

 - For the relation type fields `SetAccount` and `SetChildren` enable you to pass in other proxies just as easily as primitive types.

## Generate `utils.go`
### When running `pocketbase-gogen generate` with the `--utils` flag you get `utils.go`
`utils.go` will be saved in the same package next to your output generated source code file.

It looks like this:
```go
type Proxy interface{ BankAccount | Child | Person }

// This interface constrains a type parameter of
// a Proxy core type into its pointer type.
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
//	proxy := WrapRecord[ProxyType](record)
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
	"person": {
		"bank_account": {
			{"account", false},
		},
		"child": {
			{"children", true},
		},
	},
}
```

- The util functions are documented with usage examples.
- The type contstraint interfaces `Proxy` and `ProxyP` can also be used outside the utils for generic parameters of proxy-handling functions.
- The `Relations` map is a helper for when you need to fetch related records to expand relations. It connects the names of relation type fields with the origin collection of the related records.

## Custom Methods
### `pocketbase-gogen` also converts methods that you manually add to your template

 > [!IMPORTANT]  
> As said before, always test your generated code especially with custom methods.

For example:

```go
func (a *BankAccount) Withdraw(amount int) {
	newAmount := a.money - amount
	if newAmount < 0 {
		fmt.Println("You are out of money!")
		return
	}
	a.money = newAmount
}
```

Will generate this:

```go
func (a *BankAccount) Withdraw(amount int) {
	newAmount := a.Money() - amount
	if newAmount < 0 {
		fmt.Println("You are out of money!")
		return
	}
	a.SetMoney(newAmount)
}
```

As you can see, the generator replaced all template field accessors with the appropriate getters and setters so you use the method for editing your data.

A few more examples:

<table align="center" width="100%">
<tr>
<th> Template</th>
<th> Generated</code></th>
</tr>
<tr>
<td>

```go
func (a *BankAccount) Deposit(amount int) {
	a.money += amount
}
```

</td>
<td>

```go
func (a *BankAccount) Deposit(amount int) {
	a.SetMoney(a.Money() + amount)
}
```

</td>
</tr>
</table>

<table align="center" width="100%">
<tr>
<th> Template</th>
<th> Generated</code></th>
</tr>
<tr>
<td>

```go
// Doctor approved logic
func (p *Person) UpdateHealth() {
	if len(p.children) <= 2 {
		p.health = 0
	} else if len(p.children) <= 4 {
		p.health = 1
	} else {
		p.health = 2
	}
}
```

</td>
<td>

```go
// Doctor approved logic
func (p *Person) UpdateHealth() {
	if len(p.Children()) <= 2 {
		p.SetHealth(HealthSelectType(0))
	} else if len(p.Children()) <= 4 {
		p.SetHealth(HealthSelectType(1))
	} else {
		p.SetHealth(HealthSelectType(2))
	}
}
```

</td>
</tr>
</table>

<table align="center" width="100%">
<tr>
<th> Template</th>
<th> Generated</code></th>
</tr>
<tr>
<td>

```go
func (p *Person) BearChild(child *Child) {
	p.children = append(p.children, child)
}
```

</td>
<td>

```go
func (p *Person) BearChild(child *Child) {
	p.SetChildren(append(p.Children(), child))
}
```

</td>
</tr>
</table>

<table align="center" width="100%">
<tr>
<th> Template</th>
<th> Generated</code></th>
</tr>
<tr>
<td>

```go
func (p *Person) SwitchMyLifeUp() {
	p.name, p.address = p.address, p.name
	p.account.Withdraw(999999)
	p.children = []*Child{}
}
```

</td>
<td>

```go
func (p *Person) SwitchMyLifeUp() {
	name, address := p.Address(), p.Name()
	p.SetAddress(address)
	p.SetName(name)
	p.Account().Withdraw(999999)
	p.SetChildren([]*Child{})
}
```

</td>
</tr>
</table>

## Details

- The generator protects you from accidentially shadowing any names from the `core.Record` struct which could compromise your data in obscure ways.
  For example you can't define `func (p *Person) Id()`. The generator will error.
- There is no support for creating new records from inside of custom template methods.
- When using out-of-bounds indices for select types the getters/setters will panic.
- You can access most system fields in custom methods because they have their getters/setters directly in `core.Record`. Double check what you are doing with those.
- You can rename almost everything in the template. The comment at the top of the template file has instructions for that.
- The `pocketbase-gogen` command needs access to go module imports that you are using (mostly the PocketBase module). Best run it from inside your project directory.
- If you have reserved go keywords (e.g. `func`) as field names in your PB schema, the generator will escape them using a trailing underscore (`func_`).
- When you delete the `// collection-name:` comment from a template struct, the `CollectionName()` method will not be generated and because of that the proxy type will not work with the functions from `utils.go`. The generator will warn of the missing comment.
