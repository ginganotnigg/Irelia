clean:
	rm api/*.go
generate:
	buf generate --template buf.gen.yaml --path ./api/irelia.proto