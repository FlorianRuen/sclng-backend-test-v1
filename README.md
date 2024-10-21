# Canvas for Backend Technical Test at Scalingo

## Overview

This project is a backend service that interacts with the GitHub API to fetch and filter the last 100 repositories created. 
It is designed to run in a Docker container and uses a configuration file for customizable settings.

## Getting Started

### Execution

To run the application, execute the following command:

```bash
docker-compose up
```

Application will be then running on port `5000`.

### Testing the Endpoint

You can test the service by sending a request to the `/ping` endpoint:

```bash
curl http://localhost:5000/ping
```

You should receive a response like:

```json
{ "status": "pong" }
```

## Configuration

The application uses a `config.toml` file located in `config/config.toml`. Below is a sample configuration:

```toml
[API]
    # Port on which this service runs
    # Default value = "5000"
    # ListenPort = "5000"

[TASKS]
    # Number of tasks allowed to run concurrently when fetching repository languages
    # Default value = 8
    # MaxParallelTasksAllowed = 8

[GITHUB]
    # GitHub token to increase the rate limit for API requests
    # Non-authenticated requests = 60 calls/hour
    # Authenticated requests = 5000 calls/hour
    # Default value = ""
    # Token = ""

[LOGS]
    # Configuration for application logs
    # Available values: error, warn, info, debug
    # Default value = "debug"
    # Level = "debug"

    # Log output format (JSON or plain text), useful for log parsers like Loki
    # Default value = false
    # OutputLogsAsJSON = false
```

## Endpoints

### Fetch Repositories

The main endpoint retrieves the last 100 repositories created on GitHub:

```bash
curl http://localhost:5000/repos
```

Response Example:

```json
[
    {
        "fullName": "jwasham/practice-c",
        "owner": "jwasham",
        "repository": "practice-c",
        "licence": "",
        "languages": {
            "Assembly": 1673,
            "C": 89593,
            "CMake": 2989,
            "Shell": 290
        }
    },
    ...
]
```

### Filtering Options

You can filter the repositories based on various parameters:

- **By Language**: 
  ```bash
  curl http://localhost:5000/repos?language=Go
  ```

- **By License**: 
  ```bash
  curl http://localhost:5000/repos?license=mit
  ```

- **By Owner**: 
  ```bash
  curl http://localhost:5000/repos?owner=FlorianRuen
  ```

- **By Language and License**:
  ```bash
  curl http://localhost:5000/repos?license=mit&language=Go
  ```

## Architecture

- **/controller**: Handles API requests, validates parameters, and manages error responses.
- **/service**: Contains the business logic for GitHub API requests, language processing, and error management.
- **/config**: Manages configuration settings and the configuration file.
- **/logger**: Configures logging based on application settings.

## Makefile

To simplify common tasks, a Makefile is included. Here are some useful commands:

```makefile

# Go linting
lint:
	make tidy
	golangci-lint run

# Run tests and create generates coverage report
test:
	make tidy
	make vendor
	mkdir -p tmp/
	go test -v -timeout 10m ./... -coverprofile=tmp/coverage.out -json > tmp/report.json
```

### Usage

You can run any of the commands above using `make <target>`. For details, you can run:

```bash
make help
```

## Improvement Ideas

- Enhance the rate limiter to include a wait feature that automatically delays requests until available.
- Increase test coverage, especially before production deployment.
- Implement a GitHub workflow for automatic Docker image builds and streamlined production deployments.
- Improve API error management by adding detailed return codes and request IDs for better debugging.
- Enhance logging according to log management tools, optimizing format and fields.
- Revise the Makefile to handle versioning based on GitHub tags or commit hashes.

## Notes

I enjoyed working on this test, thank you! Here are some observations:

- I updated the version of Go used in docker, so that some instructions specific to go 1.21/1.22 (notably toolchain) can work
- The retrieval of the last 100 repositories on GitHub does not always appear sorted correctly, even with the provided parameters. Using the Events API call might yield better results, though care must be taken regarding API consumption and rate limits.
- A Size WaitGroup was used to effectively manage concurrent tasks.
- No database was implemented, as I believe the responsibility of data management should lie with the applications consuming this API. Implementing a database would necessitate handling the frequent updates of information from GitHub.
- Authentication was omitted as the data accessed is public; however, it could be added later using Gin middlewares (OAuth, tokens, etc.).
- I utilized commonly recommended libraries from GitHub and those I have experience with in other Go projects, which may benefit from optimization prior to production use.
- A local rate limiter was chosen to effectively manage authorized request counts while maintaining data consistency.