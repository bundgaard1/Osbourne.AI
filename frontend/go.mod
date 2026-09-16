module osbourne.local/frontend

go 1.26.5

require (
	github.com/a-h/templ v0.3.1020
	github.com/go-chi/chi/v5 v5.3.1
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
	osbourne.local/auth-common v0.0.0
)

replace osbourne.local/auth-common => ../auth-common

require (
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260807164820-c8921c73eeea // indirect
	gorm.io/gorm v1.31.2 // indirect
)
