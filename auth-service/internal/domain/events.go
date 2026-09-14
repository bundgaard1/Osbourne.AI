package domain

// AccountCreatedEvent is the payload published as an account.created RabbitMQ
// event once an account has been persisted. profile-service consumes it to
// create the user's master-data profile row.
type AccountCreatedEvent struct {
	AccountID string
	Email     string
	Role      string
	FullName  string
}