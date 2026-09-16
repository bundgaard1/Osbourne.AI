Design Documentation
=================== 

# Overview

Below is a high-level overview of the design and architecture of the Osborne.AI project.

```m̀ermaid
graph TD;
    subgraph "Frontend"
        A[Frontend UI] -->|HTTP| B[API Gateway]
    end

    subgraph "Backend Services"
        B -->|gRPC| C[Student Profile Service]
        B -->|gRPC| D[Course Catalogue Service]
        B -->|gRPC| E[Course Content Service]
        B -->|gRPC| F[Assignment Service]
        B -->|gRPC| G[Notification Service]
        B -->|gRPC| H[Authentication Service]
    end

    subgraph "Databases"
        C --> I[(Student Profile DB)]
        D --> J[(Course Catalogue DB)]
        E --> K[(Course Content DB)]
        F --> L[(Assignment DB)]
        G --> M[(Notification DB)]
        H --> N[(Authentication DB)]
    end

    subgraph "Message Queue"
        O[RabbitMQ] -.-> C
        O -.-> D
        O -.-> E
        O -.-> F
        O -.-> G
    end


```


# Isolation

## Services

All the services are connected on docker compose networks. The only entrance is through the API Gateway (Nginx), which handles routing and authentication. The services communicate with each other using gRPC for high-speed, type-safe internal synchronous service communication. RabbitMQ is used for asynchronous event-driven task processing.

## Database

The services are made with a database-per-service isolation model, ensuring zero binary or storage coupling between domains. Each service has its own database, and the services communicate with each other through well-defined APIs.

# Example flow





