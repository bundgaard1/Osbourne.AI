.PHONY: tools generate dev

# Checks if buf is installed, otherwise it is installed automatically
tools:
	@which buf > /dev/null || (echo "Installing Buf..." && go install github.com/bufbuild/buf/cmd/buf@latest)
	@which templ > /dev/null || (echo "Installing templ..." && go install github.com/a-h/templ/cmd/templ@latest)
	@which protoc-gen-go > /dev/null || (echo "Installing protoc-gen-go..." && go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0)
	@which protoc-gen-go-grpc > /dev/null || (echo "Installing protoc-gen-go-grpc..." && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0)

# Runs the tool check and then generates code
generate: tools
	@echo "Generating Protobuf & gRPC code..."
	@cd proto && buf generate
	@echo "Generating templ components..."
	@cd frontend && templ generate

dev: generate
	@cd frontend && go run cmd/main.go

clean:
	@echo "Cleaning up generated files..."
	@rm -rf **/gen
	@find frontend -name '*_templ.go' -delete
