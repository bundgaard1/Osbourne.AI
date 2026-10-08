
## Reminders and tips 

1. Microservices
Your system should have at least three genuine functional application microservices. The database does not count as a microservice.
Each service should have a clear responsibility and perform meaningful work. If you use the starter Student Profile Service, please make sure you have meaningfully completed, extended, and tested it.

2. APIs
Each required microservice should have at least two meaningful REST API endpoints.
Please check that your APIs:

Use appropriate HTTP methods

Accept and return JSON

Validate input

Return appropriate status codes

Provide clear error messages

3. Service communication
Please demonstrate meaningful communication between your services. This should be through mechanisms such as HTTP, TCP, or a message queue, rather than directly importing another service's Python files.
Also think about what happens if another service is unavailable or returns an error.

4. Docker and Docker Compose
Please check that all your services are correctly configured in docker-compose.yml and that each application service has its own Dockerfile.
Remember that communication between containers should use the Docker Compose service name rather than localhost.

It is also a good idea to test your system using:

docker compose config

docker compose up --build

docker compose ps
5. Testing and evidence
Please test both successful and unsuccessful scenarios, such as:
Creating and retrieving records

Invalid or missing data

Records that do not exist

Duplicate data where relevant

Service-to-service communication

Data persistence after restarting the containers

Please keep useful screenshots or recordings as evidence of your work, including your Docker containers, API testing, communication between services, and error handling.

6. README and submission
Your README should clearly explain your project, services, architecture, setup instructions, API endpoints, testing process, and any known limitations.
Before submitting, please check that your ZIP file includes the required code, docker-compose.yml, Dockerfiles, dependency files, .env.example, README, architecture diagram, and testing evidence.

Please do not include your actual .env file, passwords, access tokens, or other sensitive information.

Finally, make sure you can explain your service design, communication, Docker configuration, database design, and testing. The aim is not only to make the system work, but also to demonstrate your understanding of the concepts covered in this course.


## Thinking microservices 

Service boundaries

For each service, think about what specific business capability it provides and whether you can explain its responsibility clearly in one sentence. Consider whether the functions within the service belong together and whether the service is genuinely different from the responsibilities of your other services. You can also ask yourself whether the service would still make sense as a separate component if your system became larger.

For example, separating student profiles, course information, feedback, and notifications can be reasonable because each represents a different business capability. On the other hand, dividing one small operation into several very small or almost empty services may be more difficult to justify.

Independence and coupling

Another important part of microservices is independence. Ideally, one service should be able to be developed, rebuilt, or restarted without requiring every other service to be changed at the same time.

Please think about whether each service has its own application code and container configuration, and whether one service depends directly on another service's internal files or implementation. If changing one service means that several other services must also be changed, you may have created a system that is still quite tightly coupled.

For example, directly importing another service's Python code creates a strong dependency between the services. Communication through a clearly defined API or other interface provides a clearer separation between them.

Data ownership

Please also think carefully about data ownership. For each type of information in your system, consider which service is responsible for creating and updating it. If another service needs that information, does it really need direct access to the data, or could it request the information through the service that owns it?

Several services may use the same PostgreSQL or MongoDB server in a prototype, and this is not necessarily a problem. However, your design should still make it clear which service is responsible for which data. Sharing a database should not mean that the boundaries between your services disappear.

Communication choices

When choosing how your services communicate, try to select a method that makes sense for the particular interaction rather than choosing a technology simply because it seems more advanced.

Think about whether the caller needs an immediate response or whether the operation could happen asynchronously. Consider what information needs to cross the service boundary and what should happen if the receiving service is slow or unavailable. You should also think about how your system handles errors, timeouts, or incomplete operations.

HTTP, TCP, gRPC, and message queues can all be used for communication between services. Your README and architecture diagram should make these interactions clear.

Please also remember that using gRPC or a message queue for internal communication does not automatically replace the assignment's RESTful API requirement. If you choose to use gRPC internally, you should still consider how your REST endpoints will provide the required external interface.

Failure and resilience

Because microservices are distributed systems, it is important to think about what happens when one service is not available. You do not need to build a production-level fault-tolerant system for this assignment, but demonstrating that you have considered these situations will show a stronger understanding of distributed applications.

For example, what happens if one service is unavailable when another service tries to communicate with it? Does the caller receive an appropriate response, or does it wait indefinitely? What happens if the same request is accidentally processed more than once? If a notification fails, should the entire operation fail? What happens when the unavailable service starts working again?

You do not need to implement every possible solution, but please show that you have thought about these issues and, where appropriate, handled them in your design.

Showing your architecture

When you demonstrate your project, try to show more than just the user interface. Your video or screenshots should help the assessor understand how your services actually work together.

For example, you could show the relevant containers running separately, a request entering one service, that service communicating with another service, and the resulting response, log entry, or stored data. You could also demonstrate what happens when part of the interaction fails.

Explaining your design

There is no single perfect combination of services or one architecture that everyone must use. What matters is that your design decisions are appropriate for your application and that you can explain the reasons behind them.

Before submitting, please take a little time to consider whether your documentation clearly explains why you chose your service boundaries, how your services communicate, which service owns each type of data, and where dependencies exist between services. It is also useful to mention any trade-offs you made and what you might change if your prototype were developed into a larger system.

Most importantly, I would like you to use this assignment as an opportunity to demonstrate not only that your system works, but also that you understand why you designed it this way.