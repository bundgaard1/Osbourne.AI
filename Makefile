.PHONY: tools generate dev test routing-test

# grpc-gateway is pinned: v2.31+ requires a google.golang.org/grpc newer than
# the v1.83.0 this workspace runs on, which would force a workspace-wide bump.
GATEWAY_VERSION := v2.30.0
# Checks if buf is installed, otherwise it is installed automatically
tools:
	@which buf > /dev/null || (echo "Installing Buf..." && go install github.com/bufbuild/buf/cmd/buf@latest)
	@which templ > /dev/null || (echo "Installing templ..." && go install github.com/a-h/templ/cmd/templ@latest)
	@which protoc-gen-go > /dev/null || (echo "Installing protoc-gen-go..." && go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0)
	@which protoc-gen-go-grpc > /dev/null || (echo "Installing protoc-gen-go-grpc..." && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0)
	@which protoc-gen-grpc-gateway > /dev/null || (echo "Installing protoc-gen-grpc-gateway..." && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@$(GATEWAY_VERSION))
	@which protoc-gen-openapiv2 > /dev/null || (echo "Installing protoc-gen-openapiv2..." && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@$(GATEWAY_VERSION))

# Runs the tool check and then generates code
generate: tools
	@echo "Resolving protobuf module dependencies..."
	@cd proto && buf dep update
	@echo "Generating Protobuf, gRPC & REST gateway code..."
	@cd proto && buf generate
	@cd proto && buf generate --template buf.gateway.gen.yaml
	@cd proto && buf generate --template buf.openapi.gen.yaml
	@echo "Generating templ components..."
	@cd frontend && templ generate

dev: generate
	@cd frontend && go run cmd/main.go

# Runs go test and go vet in every module of the go.work workspace.
test:
	@./scripts/test.sh

# Verifies the /api/* routing table against stub upstreams. Requires docker.
routing-test:
	@./scripts/routing-test.sh

clean:
	@echo "Cleaning up generated files..."
	@rm -rf **/gen
	@find frontend -name '*_templ.go' -delete
