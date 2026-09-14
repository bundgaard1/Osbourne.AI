module osbourne.local/assignment-service

go 1.26.5

require (
	github.com/glebarez/sqlite v1.11.0
	github.com/gogo/protobuf v1.3.2
	github.com/google/uuid v1.6.0
	github.com/wagslane/go-rabbitmq v0.16.1
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
	gorm.io/gorm v1.31.2
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/glebarez/go-sqlite v1.22.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rabbitmq/amqp091-go v1.12.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260807164820-c8921c73eeea // indirect
	modernc.org/libc v1.37.6 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.7.2 // indirect
	modernc.org/sqlite v1.28.0 // indirect
)

require osbourne.local/auth-common v0.0.0

replace osbourne.local/auth-common => ../auth-common
