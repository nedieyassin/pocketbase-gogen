template:
	go run main.go template ./pb_data ./.test/schema/template.go

generate:
	go run main.go generate -dju ./pb_data ./.test/proxies/proxies.go