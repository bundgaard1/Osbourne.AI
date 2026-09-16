# Osborne.AI - Microservices Project

This project is made for the INFS605 (Microservices) course project. The goal is to create a Student Services Dashboard for university operations using a microservices architecture. 

## Setup steps

Run the following commands to set up the project:
1. Clone the repository
2. Navigate to the project directory
3. Run the following commands:
```bash
make generate # Generates protobufs and frontend
docker compose up # Starts the services in Docker containers
```
4. Navigate to `http://localhost:8080` in your web browser to access the Student Services Dashboard.

## Tech stack

- All the services are written in Go.
- The frontend is a simple web interface using Go's Templ package.
- The API Gateway is built on Nginx.
- The database is a PostgreSQL instance.
- RabbitMQ is used for asynchronous event-driven task processing.

## Architecture

I went a little overboard with the number of services, but the idea was to have a microservices architecture that is as close to a real-world application as possible. The services are:

- **Student Profile Service**: Stores and manages student profiles.
- **Course Catalogue Service**: Manages the course catalogue and course information, aswell as which students are enrolled in which courses.
- **Course Content Service**: Manages course content and materials.
- **Assignment/Grading Service**: Manages assignments and grading for courses.
- **Notification Service**: Sends notifications to students and staff. 
- **Authentication Service**: Handles authentication and authorization for the services.
- **Frontend UI**: A simple web interface for interacting with the services.

For further details on the design and architecture of the project, please refer to the [design document](docs/design.md) and [notes document](docs/notes.md).