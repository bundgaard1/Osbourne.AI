# Microservices programming project
INFS605 - course project

# Task
- Osborne.AI
- Student Services Dashboard for university operations
- Microservices architecture - connected with DockerCompose, maybe Kubernetes if time permits

# Requirements
- Minimum 3 new services
- Docker and DockerCompose, git, Yaml, SQL or noSQL,
- Optional languages. (Go)

## Services (Ideas)
- Student Profile Service
- Course Catalogue Service
- Course Content Service
- Assignment/Grading Service
- Notification Service
- Authentication Service

### Standard Services
- Frontend UI,
- API Gateway (Use Nginx or Traefik)
- Monitoring and Logging (Prometheus, Grafana, ELK stack)

## Functional
- Must expose at least 2 RESTful APIs with 2 endpoints
- HTTP, TCP(RPC, maybe gRPC) and simple message queue 
- Small frontend for interaction
- Authentication to one or more services

## Non-Functional
- Each service in its own Docker container
- Must be defined in a DockerCompose file
- Include documentation and usage instructions
- System must support logging, basic error handling


# Design
- Microservices architecture with DockerCompose
- Go-based services
- Frontend in Go with HTML templates
- RabbitMQ for message queue (Asynchronous event-driven task processing)
- gRPC over HTTP/2 for high-speed, type-safe internal synchronous service communication
- Protocol Buffer (.proto) shared schema registry acting as the system's single source of truth
- Database-per-service isolation model ensuring zero binary or storage coupling between domains
- Nginx or Traefik acting as an edge reverse proxy and central JWT authorization boundary

## Extra requirements
- Services can scale to multiple instances 
