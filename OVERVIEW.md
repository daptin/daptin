## Project Overview

Daptin is a versatile backendasaservice (BaaS) designed to provide a robust foundation for applications requiring
data management, APIs, user handling, and various integrations. It aims to accelerate development by offering a rich set
of features outofthebox, configurable through declarative schemas and a web dashboard.

Key characteristics include:

- **DatabaseDriven**: Persists data in normalized relational database tables (supports PostgreSQL, MySQL, SQLite).

- **API Endpoints**: Automatically generates RESTful JSON APIs (following JSON:API spec where applicable) and GraphQL
  endpoints for CRUD operations and custom actions.

- **User & Access Control**: Features builtin user/group management, social login via OAuth, and a granular permission
  system.

- **Extensibility**: Supports custom business logic via "Actions", integrations with 3rd party services through OpenAPI
  spec imports, and cloud storage synchronization.

- **State Management**: Provides APIs for tracking object states using finite state machines (FSMs).

- **Web Features**: Supports hosting multiple static websites (subsites), optional HTTPS via Let's Encrypt, and includes
  a
  web dashboard for administration.

- **Import/Export**: Offers flexible data import from XLSX, JSON, and CSV, automatically generating schemas if needed.
  Data export is also supported.

Daptin is built with Go, making it performant, horizontally scalable, and deployable across various architectures.

## Repository Structure

The Daptin repository is organized into several key directories:

- **`.circleci/`**: Contains configuration (`config.yml`) for CircleCI continuous integration builds and tests.

- **`.github/`**: Holds GitHubspecific files, including funding information (`FUNDING.yml`) and GitHub Actions
  workflows (`workflows/go.yml`) for CI/CD.

- **`bin/`**: Contains utility scripts and small Go programs used during the build, release, or deployment process (
  e.g., `crosscompile.go`, `getgithubrelease.go`, `uploadgithub`).

- **`crossbuild/`**: Scripts related to crosscompiling the application, specifically using `xgo`.

- **`dockercomposeexamples/`**: Example `dockercompose.yml` files demonstrating how to run Daptin with dependencies
  like PostgreSQL.

- **`docs/`**: Contains the prebuilt HTML documentation site, likely generated from Markdown sources.

- **`docsmarkdown/`**: Holds the source Markdown files (`.md`) and configuration (`mkdocs.yml`) for the project
  documentation, built using MkDocs.

- **`images/`**: Stores project logos, diagrams, and icons used in documentation and potentially the application UI.

- **`integrationtests/`**: Contains files and scripts for running integration tests, including test cases (`cases/`),
  Docker setup (`dockercompose.yml`), and test runners (`runtests.sh`).

- **`kubernetes/`**: Kubernetes deployment manifests (`.yaml`) for deploying Daptin and its dependencies (e.g., MySQL).

- **`loadtest/`**: Scripts and configurations for performance load testing using tools like `vegeta` and `wrk`.

- **`schema/`**: Example schema definitions (e.g., `paybystripe.yaml`) used to configure Daptin entities or
  integrations.

- **`scripts/`**: General build and utility shell scripts (e.g., `build.sh`, `builddocs.sh`, `crosscompile.sh`).

- **`server/`**: Contains the core Go source code for the Daptin server application. This is the heart of the project,
  organized into subpackages like `auth`, `database`, `resource`, `websockets`, etc.

- **Root Directory**: Includes main configuration files (`Makefile`, `Dockerfile`, `.goreleaser.yml`, `.golangci.yml`),
  the main application entry point (`main.go`), license files (`LICENSE`, `COPYING.LESSER`), contribution
  guidelines (`CONTRIBUTING.md`, `CODEOFCONDUCT.md`), and the main project `README.md`.

## Continuous Integration and Deployment (CI/CD)

Daptin utilizes both CircleCI and GitHub Actions for its CI/CD pipelines.

### CircleCI (`.circleci/config.yml`)

- **Primary Goal**: Runs tests and builds the application using Go modules.

Sets up the specified Go version (currently 1.22.2).

Ensures Go modules are enabled (`GO111MODULE=on`).

Downloads Go dependencies (`go get`).

Builds the `main` executable with static linking flags (`ldflags='extldflags "static"'`).

Stores the built executable (`main`) as an artifact named `daptin`.

Runs tests (`go test`) with coverage enabled (`cover`, `coverprofile=coverage.out`). (Note: It seems configured to
test against `github.com/daptin/daptin/...`, which might need adjustment depending on the actual module path).

- **Configuration**: Uses aliases (`&testwithgomodules`, `&defaults`) for reusable steps.

### GitHub Actions (`.github/workflows/go.yml`)

- **Primary Goal**: Performs comprehensive builds, tests, crosscompilation, artifact uploads, and releases across
  multiple operating systems and architectures.

- **Triggers**: Runs on pushes to any branch (), pushes to tags (), and pull requests.

- **Matrix Strategy (`build` job)**: Defines build variations for different operating systems and configurations:

`linux`: Ubuntu latest, Go 1.21.x, builds Linux artifacts, runs quick tests, deploys beta builds.

`mac`: macOS latest, Go 1.21.x, builds Darwin artifacts, runs quick tests, deploys beta builds.

`windowsamd64`: Windows latest, Go 1.21.x, builds Windows amd64 artifacts, deploys beta builds.

`otheros`: Ubuntu latest, Go 1.21.x, compiles for remaining architectures (excluding linux, darwin, windows amd64),
deploys beta builds.

`modulesrace`: Ubuntu latest, Go 1.21.x, runs tests with the race detector (`make racequicktest`).

Installs the specified Go version.

Sets environment variables (GOPATH, GO111MODULE, GOTAGS, GOARCH, CGO\ENABLED).

Installs necessary system libraries (fuse, rpm, pkgconfig, rclone on Linux; winfsp, zip, wget on Windows).

Prints Go environment for debugging.

Runs quick tests (`make quicktest`) or race tests (`make racequicktest`) based on the matrix job.

- **Builds Dashboard**: Downloads `dadadash` release, unzips it into `daptinweb/`, and uses `go.rice` to embed web
  assets.

Installs build tools (`nfpm`, `govvv`).

Compiles all architectures (`make compileall`) if specified.

Builds and deploys beta binaries (`make travisbeta`) for pushes to the main repository (not forks or PR branches).

Uploads individual zipped artifacts for different OS/Arch combinations (linuxamd64, linuxarm64, freebsdamd64, etc.)
using `actions/uploadartifact@v4`.

Runs on Ubuntu latest, Go 1.21.

Builds the dashboard and embeds assets using `go.rice`.

Uses `crazymax/xgo` Docker image for crosscompilation.

Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/`.

Builds Docker image (`daptin/daptin:`) using the crosscompiled Linux amd64 binary.

Pushes Docker image to Docker Hub using secrets (`DOCKERUSERNAME`, `DOCKERPASSWORD`).

Uploads crosscompiled artifacts.

- **Creates GitHub Release**: Uses `softprops/actionghrelease@v1` on tag pushes (`refs/tags/`) to create a draft release
  with attached binaries.

## Build and Release

Daptin employs a sophisticated build and release process leveraging Makefiles, GoReleaser, Docker, and custom scripts.

### Build Orchestration (`Makefile`  Inferred)

Although the `Makefile` content isn't provided, the CI workflows indicate its presence and usage for common tasks:

`make`: Likely builds the default binary for the current platform.

`make quicktest`: Runs a subset of tests quickly.

`make racequicktest`: Runs quick tests with the Go race detector enabled.

`make compileall`: Triggers crosscompilation for various target platforms.

`make travisbeta`: A specific target likely used in the GitHub Actions workflow to build and package beta release
artifacts.

`make vars`: Prints environment variables relevant to the build.

### Release Packaging (`.goreleaser.yml`)

GoReleaser is configured to automate the creation of release artifacts when tags are pushed.

- **Environment**: Enables Go Modules (`GO111MODULE=on`).

- **Hooks**: Runs `go mod download` before building.

- **Builds**: Defines multiple build targets (`id`):

Targets: macOS (`daptindarwin`), Linux (`daptinlinux`), Windows
x64/i386 (`daptinwindowsx64`, `daptinwindowsi386`), ARM (`daptinconfidant`).

- **Versioning**: Embeds version information (Version, GitRev, BuildTime, Mode) using ldflags (`X`).

- **CGO**: Enabled (`CGOENABLED=1`) for most builds, requiring appropriate crosscompilers (
  e.g., `o64clang`, `x8664w64mingw32gcc`).

- **Testnet Builds**: Separate build IDs (`daptinenterprise`) exist, likely enabling a `testnet` build
  tag (`tags=testnet`) and potentially producing a different binary (`daptine`).

Creates `.tar.gz` archives by default, overridden to `.zip` for Windows.

Uses a naming template including version, commit hash, OS, and architecture.

Includes `README.md` and `LICENSE` in archives.

- **Checksums**: Generates a `daptinchecksums.txt` file.

- **Snapshots**: Uses `SNAPSHOT` template for snapshot releases.

- **Changelog**: Filters commit messages, excluding docs, tests, chores, and merge commits.

- **Signing**: Configured to sign artifacts using GPG (key `artpar@gmail.com`).

- **Release**: Targets the `daptin/daptin` GitHub repository, automatically marking nontag builds as prereleases.

### Containerization (`Dockerfile`, `Dockerfilearm`)

Uses a multistage build, starting with `alpine` to get CA certificates.

Final stage based on `ubuntu`.

Copies the precompiled `daptinlinuxamd64` binary into `/opt/daptin/daptin`.

Copies CA certificates.

Sets `WORKDIR` to `/opt/daptin`.

Makes the binary executable.

Sets the `ENTRYPOINT` to run `/opt/daptin/daptin runtime release port :8080`.

Note\*: Contains commentedout steps for installing `glibc` on Alpine, suggesting potential past compatibility issues or
experiments.

- **`Dockerfilearm`**: (Content not provided, but likely similar to the main Dockerfile, using an ARM base image and
  copying the `daptinlinuxarm64` binary).

### Build & Utility Scripts (`bin/`, `scripts/`)

- **`bin/crosscompile.go`**: A Go program (likely run via `go run`) to orchestrate crosscompilation for various OS/Arch
  pairs (`windows/amd64`, `darwin/amd64`, `linux/amd64`, `linux/arm`, etc.). It handles
  setting `GOOS`, `GOARCH`, `CGOENABLED`, embedding version info via ldflags, creating build directories, zipping
  artifacts, and potentially creating `.deb`/`.rpm` packages using `nfpm` (via `bin/nfpm.yaml`).

- **`bin/getgithubrelease.go`**: A Go program to fetch asset download URLs from GitHub releases, either via the API (
  authenticated) or by scraping the releases page (unauthenticated). It can match assets by regex and OS/Arch, download
  them, and optionally install (`dpkg i`) or extract executables.

- **`bin/nfpm.yaml`**: Template for `nfpm` to create `.deb` and `.rpm` packages. It defines metadata like name, arch,
  version, maintainer, description, etc.

- **`bin/uploadgithub`**: (Content not provided) Likely uploads artifacts to GitHub releases.

- **`crossbuild/xgo.sh`**: Script to perform crosscompilation using the `crazymax/xgo` Docker image.

- **`scripts/build.sh`**: Appears to be a primary build script that builds the `daptinweb` frontend, embeds assets
  using `go.rice`, builds the Go binary, appends Rice data, and builds/tags a Docker image (`daptin/daptin`).

- **`scripts/builddocs.sh`**: Builds the MkDocs documentation (`mkdocs build`) and copies the output to the `docs/`
  directory.

- **`scripts/buildosx.sh`**: Similar to `build.sh` but potentially tailored for macOS builds, using Docker for Go
  compilation.

- **`scripts/crosscompile.sh`**: Another script likely related to crosscompilation, possibly using `xgo`.

- **`scripts/go.test.sh`**: Runs `go test` for all packages (excluding vendor) and aggregates coverage profiles
  into `coverage.txt`.

## Testing

Daptin includes setups for integration testing and load testing.

### Integration Tests (`integrationtests/`)

- **Purpose**: To test the interaction between Daptin and its dependencies (like databases) and verify API behavior
  under
  realistic conditions.

`cases/`: Contains specific test scenarios, organized by feature (e.g., `authorization`, `registration`).

Each case has `testcases/.yml` files defining test steps using a framework like `pyresttest` (inferred
from `requirements.txt`). These YAML files likely define HTTP requests (URL, method, headers, body) and expected
outcomes (status codes, body content validation).

May include `dbinit/` subdirectories with SQL files (e.g., `.sql.bz2`) used to set up the database state before running
tests for that case.

`dockercompose.yml`: Defines the services needed for integration tests, typically Daptin itself and a database (MySQL
in this case: `daptinmysqldb`). It uses a custom network (`mynet`) for communication.

`python/`: Contains Python scripts potentially used for more complex test scenarios or interactions (
e.g., `imapservertest.py` suggests testing the IMAP functionality). `requirements.txt` lists Python dependencies (
e.g., `caldav`).

`requirements.txt`: Lists Python dependencies for the testing framework, specifically `pyresttest`.

`runtests.sh`: The main script to execute integration tests.

Accepts a test case name as an argument.

Manages Docker environment (`dockercompose down/up`, `docker network create`).

Copies the appropriate `dbinit` files for the selected test case.

Waits for the Daptin service to become available using `curl`.

Runs `pyresttest` within a Docker container (`thoom/pyresttest`), mounting the test case YAML files and targeting the
Daptin service within the Docker network.

- **Framework**: Primarily uses `pyresttest` for defining and running API tests based on YAML configurations.

### Load Tests (`loadtest/`)

- **Purpose**: To evaluate the performance and stability of Daptin under heavy load.

`localhost/` & `prod/`: Contain test configurations likely targeting local and productionlike environments,
respectively.

Subdirectories define specific API endpoints or actions to test (e.g., `get`, `patch`, `signin`, `getrelation`).

`attack.txt`: Defines the HTTP request details (method, URL, headers) for tools like `vegeta`.

`postbody.json`: Contains the JSON request body for POST/PATCH requests.

`wrk/`: Contains Lua scripts (`wrkpost.lua`, `wrkput.lua`) for use with the `wrk` load testing tool, allowing for
dynamic request generation.

`execute.sh`: A script to run load tests using `vegeta`. It takes target directory, test name, rate, and duration as
arguments. It pipes `vegeta attack` output to `vegeta report`.

- **Tools**: Utilizes `vegeta` and `wrk` for generating load and reporting results.

### Unit/Component Tests

- **Go Tests**: Standard Go test files (`*test.go`) like `formtest.go` and `servertest.go` exist for unit and component
  testing.

- **Coverage**: `codecov.yml` configures Codecov integration, indicating a focus on test coverage. The `coveralls` job
  in
  CircleCI and the `go.test.sh` script further support this.

## Configuration and Environment

Daptin configuration can be managed through commandline flags, environment variables, and potentially configuration
files loaded at runtime. Deployment configurations are provided for Heroku, Docker, and Kubernetes.

### Runtime Configuration (`server/config.go`, `main.go`)

- **Loading**: Configuration files named `schemadaptin.(json|yaml|toml|hcl)` are loaded on startup from the current
  directory and optionally from a path specified by the `DAPTINSCHEMAFOLDER` environment variable. These files define
  tables, relations, actions, state machines, etc. (`resource.CmsConfig`).

- **Commandline Flags**: `main.go` defines flags to override settings:

`dbtype`: Database type (`sqlite3`, `mysql`, `postgres`). Default: `sqlite3`.

`dbconnectionstring`: Connection details for the database. Default: `daptin.db`.

`localstoragepath`: Path for storing blob assets locally. Default: `./storage`.

`dashboard`: Path to the web dashboard static files. Default: `daptinweb`.

`port`: HTTP port to listen on. Default: `:6336`.

`httpsport`: HTTPS port to listen on. Default: `:443`.

`runtime`: Gin runtime mode (`release`, `debug`, `test`, `profile`). Default: `release`.

`loglevel`: Logging level (`info`, `debug`, `trace`, etc.). Default: `info`.

`portvariable`: Env var name for port override. Default: `DAPTINPORT`.

`databaseurlvariable`: Env var name for DB connection string override. Default: `DAPTINDBCONNECTIONSTRING`.

`profiledumppath`, `profiledumpperiod`: For profiling mode.

`olric`: Flags for configuring the embedded Olric distributed cache (peers, ports, env).

`DAPTIN*`: Envy (`github.com/jamiealquiza/envy`) is used to parse environment variables prefixed with `DAPTIN`. Flags
like `port`, `dashboard`, `dbtype`, `runtime`, and `dbconnectionstring` can be set via env vars (
e.g., `DAPTINPORT`, `DAPTINDBCONNECTIONSTRING`).

`DAPTINLOGLOCATION`, `DAPTINLOGMAXSIZE`, etc.: Configure rolling file logging via Lumberjack.

`DAPTINGOMAXPROCS`: Sets `runtime.GOMAXPROCS`.

`TZ`: Sets the application's timezone. Defaults to UTC if not set.

- **Internal Configuration (`config` table)**: Managed via the `server/confighandler.go`
  and `server/resource/cmsconfig.go`. Stores runtime settings like JWT secrets, encryption keys, feature flags (GraphQL,
  IMAP, FTP), rate limits, etc. Accessible via the `/config/backend/` API endpoint (requires admin privileges). Changes
  often require a server restart.

### Deployment

- **Heroku (`app.json`)**: Defines metadata for deploying Daptin on Heroku, specifying the name, description, keywords,
  Go
  buildpack (`heroku/go:1.14`), and repository URL. Likely used with the "Deploy to Heroku" button.

- **Docker (`Dockerfile`, `dockercompose.yml`, `dockercomposeexamples/`)**:

`Dockerfile`: Builds the production amd64 Daptin image (see Build & Release section).

`dockercompose.yml`: (Content empty in provided structure, but examples exist).

`dockercomposeexamples/daptinpostgres.yml`: Example setup running Daptin with a PostgreSQL database, linking them and
passing configuration via environment variables.

- **Kubernetes (`kubernetes/`)**:

`daptindeployment.yaml`: Defines a Kubernetes Service and Deployment for the Daptin application itself. It specifies
the Docker image (`daptin/daptin:latest`) and passes database configuration via commandline arguments.

`mysqldeployment.yaml`: Defines a Service, PersistentVolumeClaim, and Deployment for a MySQL database backend, using a
Kubernetes secret (`mysqlpass`) for the root password.

`ingress.yml`: Defines an Ingress resource to expose the Daptin service, likely via a specific
hostname (`testing.dapt.in` in the example).

## Server Architecture (`server/`, `main.go`)

The `server/` directory contains the core logic for the Daptin application. Based on the file and directory names, the
architecture appears to be modular and resourceoriented.

Initializes logging (logrus, lumberjack).

Parses commandline flags and environment variables (envy).

Handles profiling setup if `runtime=profile`.

Initializes database connection (`server.GetDbConnection`).

Finds/initializes the embedded web dashboard assets (`rice.FindBox`).

- **Initializes Olric Cache**: Sets up an embedded Olric distributed cache/KV store, configuring ports and peer
  discovery.

- **Calls `server.Main`**: This seems to be the core server initialization function, likely setting up resources,
  handlers, and services. It returns key components
  like `hostSwitch`, `mailDaemon`, `taskScheduler`, `configStore`, `certManager`, `ftpServer`, `imapServerInstance`.

- **Sets up Restart Handling**: Uses `gotrigger` to listen for a "restart" event, which gracefully shuts down and
  reinitializes server components (presumably triggered by an action).

- **Starts HTTP/HTTPS Servers**: Configures and starts the main HTTP server (`http.ListenAndServe`) and potentially an
  HTTPS server (`tlsServer.ListenAndServeTLS`) based on configuration, using `hostSwitch` as the handler.

Manages TLS certificates using `certManager`.

- **Core Server (`server/server.go`, `server/config.go`, `server/resource/resource.go`)**:

Likely defines the main server setup logic (`server.Main`).

`server/config.go` loads configuration files (`schema.json`/`yaml`/etc.).

`server/resource/resource.go` defines the `DbResource` struct, likely the central abstraction for handling data
entities (CRUD operations, permissions, middleware).

- **Authentication & Authorization (`server/auth/`, `server/jwt/`)**:

`auth/auth.go`: Defines permission constants (`GuestRead`, `UserCreate`, etc.), permission checking logic, `SessionUser`
struct, and the main `AuthMiddleware`.

`jwt/jwtmiddleware.go`: Implements JWT validation middleware, handling token extraction from headers/parameters/cookies
and verification against issuer/signing method/expiry. Uses Olric for token caching.

- **API
  Handlers (`server/handlers.go`, `server/actionhandlers.go`, `server/graphql.go`, `server/resource/handleaction.go`)**:

Sets up Gin request handlers for various API endpoints (`/api/`, `/action/`, `/stats/`, `/config/`, `/graphql`).

`actionhandlers.go` initializes builtin action performers (`resource.ActionPerformerInterface`).

`handleaction.go` likely contains the logic to parse action requests, validate inputs, execute outcome chains (calling
appropriate CRUD or action performers), and format responses.

`graphql.go` builds the GraphQL schema dynamically based on the configured tables and actions.

Database Interaction (`server/database/`, `server/resource/db``.go`, `server/statementbuilder/`)\*:

Uses `sqlx` for database interaction.

`database/databaseconnectioninterface.go`: Defines the DB interface.

`server/databaseconnection.go`: Provides functions to establish DB connections.

`resource/db``.go` files contain functions for CRUD operations (Create, Get, Update, Delete), schema checks,
index/constraint creation, and specific queries (e.g., getting user groups, checking admin status).

`statementbuilder/`: Uses `goqu` for SQL query building, adapting to different SQL dialects (MySQL, Postgres, SQLite).

- **Resource & Action Abstraction (`server/resource/`)**:

`resource.go`, `actions.go`, `cmsconfig.go`: Define core structs
like `DbResource`, `TableInfo`, `Action`, `Outcome`, `CmsConfig`.

`action.go` files: Implement specific builtin actions (
e.g., `actionbecomeadmin.go`, `actionmailsend.go`, `actiongeneraterandomdata.go`).

`middleware``.go`: Implement middleware logic applied during resource operations (e.g., permission checks, event
generation).

`fsm.go`, `fsmmanager.go`: Handle Finite State Machine logic for state tracking.

`storage.go`, `cloudstore.go`: Manage interactions with storage backends (local, rclone).

\*Web
Services (`server/websockets/`, `server/subsite.go`, `server/ftpserver.go`, `server/smtpserver.go`, `server/imap``.go`, `server/mailadapter.go`)

- **:**

- **

Implements WebSocket support for realtime updates (`websockets/`).

- **

- **

Handles serving static subsites (`subsite.go`).

Provides optional FTP, SMTP, and IMAP server functionality.

- **

- ****

Utilities (`server/columntypes/`, `server/csvmap/`, `server/fakerservice/`, `server/id/`, `server/utils.go`)\*\*:

`columntypes/`: Defines supported data types, their properties, validation rules, and fake data generation.

`csvmap/`: Helper for reading CSV data into maps.

`fakerservice/`: Generates fake data for testing/populating tables.

`id/`: Defines the `DaptinReferenceId` type (likely based on UUID).

`utils.go`: General utility functions.

## ResourceManager

## 1. **Component Name**

ResourceManager (Core Resource Handling)

## 2. **Purpose**

Solves the problem of managing userdefined data structures (tables/entities) within the application.

Represents the core concept of data entities, their fields, relationships, and basic operations (CRUD) on them. It
provides a consistent way to interact with any defined data type.

## 3. **Key Responsibilities**

- **Define Data Structures:** Allows defining tables, columns, and data types, either declaratively (via config files)
  or
  automatically (via data import).

- **Data Persistence:** Handles the Create, Read, Update, and Delete (CRUD) operations for all defined entities,
  interacting with the underlying database (MySQL, PostgreSQL, SQLite).

- **Data Validation:** Enforces data validation rules defined in the schema during create and update operations.

- **Data Conformation:** Applies data cleaning/normalization rules (e.g., trimming whitespace, normalizing email) before
  persisting data.

- **Relationship Management:** Creates and manages relationships (belongs\\\to, has\\\one, has\\\many) between different
  entities, including handling join tables for manytomany relationships.

- **API Exposure:** Exposes defined entities and their data via standardized JSON APIs (`/api/{entityName}`).

- **Data Auditing:** (If enabled) Creates and manages audit trail tables (`{entityName}audit`) to log historical changes
  to data rows.

- **Multilingual Support:** (If enabled) Manages separate translation tables (`{entityName}i18n`) and serves translated
  content based on `AcceptLanguage` header.

- **Schema Management:** Persists and manages the schema definitions themselves within the `world` table.

## 4. **Workflows / Use Cases**

- **Creating a New Data Entry:**

Trigger: `POST /api/{entityName}` request.

Steps: Validate input against schema rules, apply conformations, check permissions, insert data into the database table,
generate audit log (if enabled), generate websocket event (if subscribed).

Outcome: New data row created, JSON API response returned.

Trigger: `GET /api/{entityName}` or `GET /api/{entityName}/{id}` request.

Steps: Check permissions, query database (with filtering, sorting, pagination), fetch related data (if requested
via `includedrelations`), format response according to JSON API spec, apply language filtering (if enabled).

Outcome: List or single data entry returned in JSON API format.

Trigger: `PATCH /api/{entityName}/{id}` request.

Steps: Validate input against schema rules, apply conformations, check permissions, update data in the database table,
generate audit log (if enabled), generate websocket event (if subscribed).

Outcome: Updated data row returned in JSON API response.

Trigger: `DELETE /api/{entityName}/{id}` request.

Steps: Check permissions, delete data from the database table, potentially handle related data based on constraints,
generate audit log (if enabled), generate websocket event (if subscribed).

Outcome: Success (204 No Content) or error response.

Trigger: `GET /api/{entityName}/{id}/{relationName}` request.

Steps: Check permissions, query the appropriate related table or join table based on the defined relation.

Outcome: List of related data entries returned.

## 5. **Inputs and Outputs**

- **Inputs:** HTTP requests (GET, POST, PATCH, DELETE), JSON API formatted data payloads, query parameters (filtering,
  sorting, pagination, included relations), schema definitions (JSON/YAML).

- **Outputs:** JSON API formatted responses, database records created/updated/deleted, audit log entries, websocket
  events.

## 6. **Dependencies**

- **Internal:** Database connection (`server/database`), Auth/Permission
  Engine (`server/auth`, `server/resource/permission.go`), Statement Builder (`server/statementbuilder`), Configuration
  Manager (`server/resource/config.go`), WebSocket Service (`server/websockets/`).

- **External:** Database (MySQL, PostgreSQL, SQLite), `api2go` library (JSON API formatting), `goqu` (SQL query
  building), `conform` (data conformation), `validator.v9` (data validation).

- **Database Models:** Dynamically interacts with all userdefined tables, plus system tables
  like `world`, `config`, `useraccount`, `usergroup`, audit tables (`audit`), translation tables (`i18n`), and
  relationship join tables (`has`).

## 7. **Business Rules & Constraints**

Table and column names must adhere to database naming conventions and avoid reserved words.

Relationships define database constraints (foreign keys, join tables).

Data validation rules (required fields, email format, numeric ranges, etc.) are enforced.

Standard columns (`id`, `version`, `createdat`, `updatedat`, `referenceid`, `permission`, `userid`) are automatically
added to all tables.

Join tables are automatically created for manytomany relationships.

Audit tables mirror the structure of the original table plus an `isauditof` column.

Translation tables mirror the structure plus `languageid` and `translationreferenceid` columns.

## 8. **Design Considerations**

- **Dynamic Schema:** The system is designed to work with schemas defined at runtime, stored in the `world` table,
  allowing flexibility without code changes or restarts for schema modifications (though some changes like column
  deletion
  might require a restart signal).

- **JSON API Standard:** Adheres to the JSON API specification for request/response structure, promoting
  interoperability.

- **ORMlike Abstraction:** Provides a higherlevel interface over raw SQL, simplifying data interaction.

- **Middleware Pipeline:** Utilizes middleware for crosscutting concerns like permissions, event generation, and data
  validation, keeping the core CRUD logic cleaner.

- **Standard Columns:** Enforces a common structure across all tables for consistency in tracking ownership,
  permissions,
  and timestamps.

## ActionExecutor

## 1. **Component Name**

ActionExecutor

## 2. **Purpose**

Solves the need for complex, multistep business operations that go beyond simple CRUD actions on a single entity.

Represents custom business workflows or processes that can be triggered via API
endpoints (`/action/{entityName}/{actionName}`).

## 3. **Key Responsibilities**

- **Define Actions:** Allows defining named actions associated with specific entity types, including input fields,
  validations, conformations, and a sequence of outcomes (steps).

- **Execute Action Sequences:** Processes a defined sequence of outcomes when an action endpoint is triggered.

- **Input Handling:** Gathers input data for an action from the request body, query parameters, and URL parameters.

- **Input Validation/Conformation:** Applies predefined validation and conformation rules to the action's input fields.

- **Outcome Execution:** Executes each outcome step in the defined sequence. Outcomes can include database operations (
  CRUD on any entity), executing integrated 3rd party API calls, sending emails, interacting with cloud storage,
  generating JWT tokens, managing SSL certificates, manipulating clientside state (notifications, redirects), etc.

- **Context Propagation:** Passes data generated from one outcome step (identified by a `Reference` name) to subsequent
  steps, allowing for complex data flows.

- **Conditional Execution:** Allows skipping outcome steps based on conditions evaluated using JavaScript (`Condition`
  field).

- **Error Handling:** Manages errors during outcome execution, potentially continuing or halting the sequence based on
  the `ContinueOnError` flag.

- **Response Aggregation:** Collects responses from individual outcomes (unless `SkipInResponse` is true) and returns
  them
  as a structured list in the final API response.

## 4. **Workflows / Use Cases**

Trigger: `POST /action/useraccount/signup` request with user details.

Steps: Validate inputs (password match, email format), create `useraccount` record, create `usergroup` record, create
relationship record between user and group, return success notification and redirect command.

Outcome: New user and associated group created, client notified and redirected.

Trigger: `POST /action/useraccount/signin` request with email/password.

Steps: Fetch user by email, verify password hash, generate JWT token, return token (via client storage/cookie) and
success notification/redirect.

Outcome: User authenticated, JWT token provided to client.

- **Data Import (e.g., CSV Upload):**

Trigger: `POST /action/world/uploadcsvtosystemschema` with CSV file and target entity name.

Steps: Parse CSV, potentially create/update table schema based on flags (`createifnotexists`, `addmissingcolumns`),
insert data rows into the target table.

Outcome: Data imported into the specified table.

- **ThirdParty Integration (e.g., Payment):**

Trigger: Custom action like `POST /action/package/buypackage` with package ID.

Steps: Create `payment` record (status: initiated), create `sale` record linking user/package/payment, call
Stripe/PayPal API (via Integration outcome) to create payment intent, return payment intent details to client.

Outcome: Payment initiated with the thirdparty provider, relevant records created internally.

- **Cloud Storage Interaction:**

Trigger: e.g., `POST /action/cloudstore/uploadfile` with file data, path, store details.

Steps: Authenticate with cloud provider (using stored OAuth token), upload file via rclone integration.

Outcome: File uploaded to the specified cloud storage path.

## 5. **Inputs and Outputs**

- **Inputs:** HTTP POST requests to `/action/...` endpoints, JSON payloads containing `attributes` map, URL parameters,
  query parameters, Action definitions (from `action` table/config files).

- **Outputs:** Array of `ActionResponse` objects (JSON), side effects like database changes, emails sent, files
  uploaded,
  thirdparty API calls made.

## 6. **Dependencies**

- **Internal:** ResourceManager (for CRUD outcomes), PermissionEngine (for checking if action can be executed),
  IntegrationService (for executing 3rd party API outcomes), EmailService (for mail outcomes), CloudStorageManager (for
  storage outcomes), CertificateService, StateMachineManager, ConfigurationManager, all `ActionPerformerInterface`
  implementations (`server/resource/action.go`).

- **External:** Database, potentially any integrated 3rd party service (Stripe, PayPal, AWS SES, etc.), `goja` (
  JavaScript
  engine for conditions/attributes).

- **Database Models:** `action` table (for storing action definitions), potentially interacts with any table via CRUD
  outcomes.

## 7. **Business Rules & Constraints**

Actions are defined against a specific entity type (`OnType`).

`InstanceOptional` determines if an action requires a specific instance ID (`{entityName}id`) to be passed.

Input fields can have `required` validation and other data type specific validations/conformations.

Outcomes are executed sequentially.

Data from previous outcomes (with a `Reference` name) can be accessed in subsequent outcomes using `$.` syntax.

Input attributes can be accessed using `~` syntax.

JavaScript can be used in `Condition` and attribute values (prefixed with `!`).

Permissions are checked: first, if the user can execute the specific action; second, if the user has permission for the
subject instance (if applicable); third, permissions for each outcome\* are checked individually (e.g., user needs
create permission on `usergroup` to execute a `POST` outcome on `usergroup`).

## 8. **Design Considerations**

- **Workflow Abstraction:** Actions provide a powerful way to encapsulate business logic without writing custom Go code
  for each workflow.

- **Declarative:** Workflows are defined declaratively in JSON/YAML, making them easier to manage and modify.

- **Extensibility:** New capabilities can be added by implementing the `ActionPerformerInterface`.

- **Composability:** Actions can be chained or called from other actions (though not explicitly shown, could be achieved
  via network request outcome to self).

- **JavaScript Integration:** Allows for dynamic conditions and attribute generation within workflows.

- **Security:** Relies heavily on the PermissionEngine to secure both the action trigger and the individual outcome
  operations.

## PermissionEngine

## 1. **Component Name**

PermissionEngine

## 2. **Purpose**

Solves the problem of controlling access to data and actions based on user identity and group membership.

Represents the authorization layer, enforcing rules about who can perform what operations (Read, Create, Update, Delete,
Execute, etc.) on which resources (tables, specific rows, actions).

## 3. **Key Responsibilities**

- **Define Permission Model:** Implements a permission model based on Owner, Group, and Guest (Other) access levels,
  similar to Unix file permissions (Read, Write, Execute extended for CRUD etc.).

- **Permission Storage:** Reads permission settings associated with tables (from `world` table) and individual data
  rows (
  from the `permission` column in each table).

- **User/Group Association:** Determines the owner of a resource (`userid` column) and the groups a user belongs to (
  via `useraccountuseraccountidhasusergroupusergroupid` join table).

- **Access Check Enforcement:** Performs checks before and after database/action operations to determine if the
  requesting
  user (or guest) has the necessary privileges.

- **Hierarchy Check:** Checks permissions in the order: Owner > Group > Guest. If access is granted at a higher level,
  lower levels are not checked.

- **Administrator Bypass:** Allows users in the 'administrators' group to bypass standard permission checks.

- **Guest Handling:** Treats requests without valid authentication tokens as 'Guest' and applies guestspecific
  permissions.

## 4. **Workflows / Use Cases**

- **API Request Authorization:**

Trigger: Any incoming API request (CRUD, Action, Relation).

Steps: Identify the user (from JWT or guest status), identify the target resource (table, row, action), retrieve
relevant permissions (entitylevel, objectlevel), retrieve user's group memberships, evaluate if the required
permission bit (e.g., `UserRead`, `GuestCreate`) is set based on ownership/group membership/guest status.

Outcome: Request allowed to proceed or rejected with a 401/403 error.

Trigger: `GET /api/{entityName}` request.

Steps: After fetching data from the database, iterate through results and filter out rows the current user does not have
at least `Peek` permission for (based on Owner/Group/Guest rules).

Outcome: Filtered list of data entries returned.

Trigger: `POST /action/{entityName}/{actionName}` request.

Steps: Check if the user has `Execute` permission on the `action` definition itself. Then, for each outcome within the
action, perform standard permission checks based on the outcome's target entity/object and method (e.g.,
check `UserCreate` permission for a `POST` outcome).

Outcome: Action execution allowed or denied.

## 5. **Inputs and Outputs**

- **Inputs:** Request context (containing user/group info or guest status), target resource (table name, object ID,
  action
  name), required permission level (e.g., `UserRead`), resource's owner ID, resource's permission bits.

- **Outputs:** Boolean decision (Allow/Deny), potentially filtered data lists.

## 6. **Dependencies**

- **Internal:** ResourceManager (to fetch resource owner/permissions), Auth Middleware (to identify user), Database
  Connection.

- **Database Models:** `world` (for entitylevel permissions), `config` (for administrator group ID,
  potentially), `useraccount`, `usergroup`, `useraccountuseraccountidhasusergroupusergroupid` (for user/group
  lookups), `permission` column in all tables.

## 7. **Business Rules & Constraints**

Permissions are represented as bitmasks (int64).

Permission bits cover Peek, Read, Create, Update, Delete, Execute, Refer for Guest, User (Owner), and Group levels.

The permission check hierarchy is Owner > Group > Guest.

Administrators bypass standard checks.

`DEFAULTPERMISSION` constant defines baseline access for new objects.

`DEFAULTPERMISSIONWHENNOADMIN` defines open access before the first administrator is established.

Certain system tables/actions might have hardcoded or specially managed permissions.

## 8. **Design Considerations**

- **Bitmask Efficiency:** Using bitmasks for permissions allows for efficient storage and checking of multiple access
  rights.

- **Unixlike Model:** Leverages a familiar Owner/Group/Other permission structure.

- **Granularity:** Provides both entitylevel and objectlevel control.

- **Centralized Logic:** Consolidates authorization checks within this component.

- **Extensibility:** The bitmask can potentially be extended with more permission types if needed.

- **Admin Override:** Provides a necessary mechanism for system administration.

## StateMachineManager

## 1. **Component Name**

StateMachineManager

## 2. **Purpose**

Solves the problem of tracking and managing the lifecycle state of data entries (resources).

Represents the concept of Finite State Machines (FSMs) applied to data, ensuring that state transitions happen according
to predefined rules.

## 3. **Key Responsibilities**

- **Define State Machines:** Allows defining state machines with named states, an initial state, and events that trigger
  transitions between states (`StateMachineDescriptions` in config/DB).

- **Track Object State:** Creates and manages state tracking records (`{entityName}state` table) for individual data
  objects when state tracking is initiated.

- **State Transition Logic:** Enforces valid state transitions based on the defined FSM. An event can only transition an
  object from a valid source state (`Src`) to the defined destination state (`Dst`).

- **Event Handling:** Provides an API endpoint (`/track/event/...`) to trigger named events on a specific object's state
  record.

- **State Persistence:** Stores the current state of a tracked object in its corresponding `state` table.

## 4. **Workflows / Use Cases**

- **Initiating State Tracking:**

Trigger: `POST /track/start/{stateMachineId}` request with target object details (`typeName`, `referenceId`).

Steps: Verify the State Machine definition exists, verify the target object exists, check permissions, create a new row
in the `{typeName}state` table, set its `currentstate` to the FSM's `InitialState`, link it to the target object and the
FSM definition.

Outcome: State tracking initiated for the object, response contains the new state record details (including its ID).

Trigger: `POST /track/event/{typeName}/{objectStateId}/{eventName}` request.

Steps: Fetch the current state record (`objectStateId`), fetch the relevant FSM definition, check if the `eventName` is
a valid transition from the `currentstate` according to the FSM definition, check permissions, if valid, update
the `currentstate` in the state record to the event's destination state (`Dst`), create an audit record for the state
change.

Outcome: Object state transitioned, response confirms the new state.

- **Enabling State Tracking for an Entity:**

Trigger: User updates the `world` table entry for an entity, setting `isstatetrackingenabled` to true and associating
SMDs.

Steps: System identifies this change (likely on restart/reconfiguration), creates the `{entityName}state` table if it
doesn't exist, adds necessary foreign keys and relations (`isstateof{entityName}` linking to the
entity, `{entityName}smd` linking to the `smd` table).

Outcome: The entity type is now capable of having its instances tracked by the associated state machines.

## 5. **Inputs and Outputs**

- **Inputs:** State Machine definitions (JSON/YAML), API requests (`/track/start`, `/track/event`), object identifiers,
  event names.

- **Outputs:** State tracking records created/updated in `state` tables, API responses confirming state or errors.

## 6. **Dependencies**

- **Internal:** ResourceManager (to fetch FSM definitions from `smd` table, fetch/update `state` tables, fetch subject
  objects), PermissionEngine.

- **External:** Database, `looplab/fsm` library.

- **Database Models:** `smd` (State Machine Definition) table, dynamically created `*state` tables for each entity with
  tracking enabled.

## 7. **Business Rules & Constraints**

Each state machine must have a unique name and a defined initial state.

Events define valid transitions between states (specific source states to a single destination state).

An event can only be triggered if the object is currently in one of the event's specified source states (`Src`).

State tracking must be explicitly enabled for an entity type in the `world` table configuration.

An entity type must be explicitly associated with one or more State Machine Definitions (SMDs) via the `world` table
configuration.

## 8. **Design Considerations**

- **Decoupling:** State logic is decoupled from the main entity logic, managed in separate tables and via specific APIs.

- **Declarative FSM:** State machines are defined declaratively, making workflows easy to visualize and modify.

- **Reusability:** The same state machine definition can potentially be applied to multiple entity types if configured.

- **Auditability:** State transitions inherently create a history (though explicit audit tables might provide more
  detail
  if enabled on the `state` table itself).

- **Library Reliance:** Leverages the `looplab/fsm` library for the core FSM implementation.

## CloudStorageManager

## 1. **Component Name**

CloudStorageManager

## 2. **Purpose**

Solves the problem of interacting with various external cloud storage providers (S3, Google Drive, Dropbox, local
filesystem, etc.) in a unified way.

Represents the abstraction layer for file and object storage operations used by other features like Asset Columns and
Site Hosting.

## 3. **Key Responsibilities**

- **Configure Storage Backends:** Allows defining connections to different storage providers (`cloudstore` table),
  including type, provider, root path, and authentication details (often referencing `oauthtoken` or `credential`).

- **Authentication Handling:** Manages authentication with providers, potentially using stored OAuth tokens (and
  refreshing them).

- **File/Object Operations:** Provides internal functions (wrapped by `rclone`) for listing directories, reading files,
  writing/uploading files, deleting files/directories, creating directories, and moving/renaming paths on the configured
  storage backends.

- **Local Caching/Syncing:** Manages local filesystem caches (`AssetFolderCache`) for specific cloud storage paths used
  by
  Asset Columns or Sites, handling periodic synchronization.

## 4. **Workflows / Use Cases**

- **Uploading a File (via Action):**

Trigger: `cloudstore.file.upload` action outcome.

Steps: Identify the target `cloudstore` definition, retrieve necessary credentials (e.g., OAuth token), use `rclone`
libraries to perform the upload operation to the specified path within the store's root.

Outcome: File uploaded to the external storage.

- **Creating a Folder (via Action):**

Trigger: `cloudstore.folder.create` action outcome.

Steps: Identify the target `cloudstore`, retrieve credentials, use `rclone` to create the directory at the specified
path.

Outcome: Folder created on the external storage.

- **Deleting a Path (via Action):**

Trigger: `cloudstore.file.delete` action outcome (can delete files or folders).

Steps: Identify the target `cloudstore`, retrieve credentials, use `rclone` to delete the specified path.

Outcome: Path deleted from the external storage.

- **Moving/Renaming a Path (via Action):**

Trigger: `cloudstore.path.move` action outcome.

Steps: Identify the target `cloudstore`, retrieve credentials, use `rclone` to move/rename the source path to the
destination path.

Outcome: Path moved/renamed on the external storage.

- **Synchronizing an Asset Column's Cache:**

Trigger: Scheduled task (`tasksyncstorage.go`) or manual action (`column.storage.sync`).

Steps: Identify the `cloudstore` associated with the asset column, retrieve credentials, use `rclone`'s sync/copy
functionality to update the local cache directory (`AssetFolderCache.LocalSyncPath`) from the remote
path (`CloudStore.RootPath` + `ForeignKeyData.KeyName`).

Outcome: Local cache updated with changes from the remote storage.

- **Serving an Asset Column File:**

Trigger: `GET /asset/{tableName}/{id}/{columnName}` request.

Steps: Identify the associated `cloudstore` and `AssetFolderCache`, construct the local file path (`LocalSyncPath` +
file path stored in the column), serve the file from the local cache.

Outcome: File content served to the client.

## 5. **Inputs and Outputs**

- **Inputs:** `cloudstore` table configuration, `oauthtoken`/`credential` data, file data (for uploads), paths (for all
  operations).

- **Outputs:** Files/folders created/updated/deleted on external storage, synchronized local cache directories, file
  content streamed to clients (via asset handler).

## 6. **Dependencies**

- **Internal:** ResourceManager (to read `cloudstore`, `oauthtoken`, `credential` tables), ConfigurationManager (for
  encryption secret).

- **External:** `rclone` library and its dependencies for various providers (AWS SDK, Google Drive API client, etc.),
  local filesystem (for caching).

- **Database Models:** `cloudstore`, `oauthtoken`, `credential`.

## 7. **Business Rules & Constraints**

Each `cloudstore` entry defines a connection to a specific provider and root path.

Authentication details must be correctly configured (often requires valid OAuth tokens or credentials).

Operations are generally performed relative to the configured `rootpath`.

Asset columns must reference a valid `cloudstore` configuration.

Local sync paths need appropriate filesystem permissions.

## 8. **Design Considerations**

- **Rclone Abstraction:** Leverages the `rclone` library to abstract the complexities of interacting with dozens of
  different storage providers, providing a consistent internal API.

- **OAuth Integration:** Tightly integrated with the OAuth token management for providers that require it.

- **Local Caching:** Uses local filesystem caching for frequently accessed assets (asset columns, sites) to improve
  performance and reduce external API calls. Synchronization ensures eventual consistency.

- **ActionBased Operations:** Exposes cloud store operations primarily through the Action system, allowing them to be
  part of larger workflows.

## CloudStorageManager

## 1. **Component Name**

CloudStorageManager

## 2. **Purpose**

Solves the problem of interacting with various external cloud storage providers (S3, Google Drive, Dropbox, local
filesystem, etc.) in a unified way.

Represents the abstraction layer for file and object storage operations used by other features like Asset Columns and
Site Hosting.

## 3. **Key Responsibilities**

- **Configure Storage Backends:** Allows defining connections to different storage providers (`cloudstore` table),
  including type, provider, root path, and authentication details (often referencing `oauthtoken` or `credential`).

- **Authentication Handling:** Manages authentication with providers, potentially using stored OAuth tokens (and
  refreshing them).

- **File/Object Operations:** Provides internal functions (wrapped by `rclone`) for listing directories, reading files,
  writing/uploading files, deleting files/directories, creating directories, and moving/renaming paths on the configured
  storage backends.

- **Local Caching/Syncing:** Manages local filesystem caches (`AssetFolderCache`) for specific cloud storage paths used
  by
  Asset Columns or Sites, handling periodic synchronization.

## 4. **Workflows / Use Cases**

- **Uploading a File (via Action):**

Trigger: `cloudstore.file.upload` action outcome.

Steps: Identify the target `cloudstore` definition, retrieve necessary credentials (e.g., OAuth token), use `rclone`
libraries to perform the upload operation to the specified path within the store's root.

Outcome: File uploaded to the external storage.

- **Creating a Folder (via Action):**

Trigger: `cloudstore.folder.create` action outcome.

Steps: Identify the target `cloudstore`, retrieve credentials, use `rclone` to create the directory at the specified
path.

Outcome: Folder created on the external storage.

- **Deleting a Path (via Action):**

Trigger: `cloudstore.file.delete` action outcome (can delete files or folders).

Steps: Identify the target `cloudstore`, retrieve credentials, use `rclone` to delete the specified path.

Outcome: Path deleted from the external storage.

- **Moving/Renaming a Path (via Action):**

Trigger: `cloudstore.path.move` action outcome.

Steps: Identify the target `cloudstore`, retrieve credentials, use `rclone` to move/rename the source path to the
destination path.

Outcome: Path moved/renamed on the external storage.

- **Synchronizing an Asset Column's Cache:**

Trigger: Scheduled task (`tasksyncstorage.go`) or manual action (`column.storage.sync`).

Steps: Identify the `cloudstore` associated with the asset column, retrieve credentials, use `rclone`'s sync/copy
functionality to update the local cache directory (`AssetFolderCache.LocalSyncPath`) from the remote
path (`CloudStore.RootPath` + `ForeignKeyData.KeyName`).

Outcome: Local cache updated with changes from the remote storage.

- **Serving an Asset Column File:**

Trigger: `GET /asset/{tableName}/{id}/{columnName}` request.

Steps: Identify the associated `cloudstore` and `AssetFolderCache`, construct the local file path (`LocalSyncPath` +
file path stored in the column), serve the file from the local cache.

Outcome: File content served to the client.

## 5. **Inputs and Outputs**

- **Inputs:** `cloudstore` table configuration, `oauthtoken`/`credential` data, file data (for uploads), paths (for all
  operations).

- **Outputs:** Files/folders created/updated/deleted on external storage, synchronized local cache directories, file
  content streamed to clients (via asset handler).

## 6. **Dependencies**

- **Internal:** ResourceManager (to read `cloudstore`, `oauthtoken`, `credential` tables), ConfigurationManager (for
  encryption secret).

- **External:** `rclone` library and its dependencies for various providers (AWS SDK, Google Drive API client, etc.),
  local filesystem (for caching).

- **Database Models:** `cloudstore`, `oauthtoken`, `credential`.

## 7. **Business Rules & Constraints**

Each `cloudstore` entry defines a connection to a specific provider and root path.

Authentication details must be correctly configured (often requires valid OAuth tokens or credentials).

Operations are generally performed relative to the configured `rootpath`.

Asset columns must reference a valid `cloudstore` configuration.

Local sync paths need appropriate filesystem permissions.

## 8. **Design Considerations**

- **Rclone Abstraction:** Leverages the `rclone` library to abstract the complexities of interacting with dozens of
  different storage providers, providing a consistent internal API.

- **OAuth Integration:** Tightly integrated with the OAuth token management for providers that require it.

- **Local Caching:** Uses local filesystem caching for frequently accessed assets (asset columns, sites) to improve
  performance and reduce external API calls. Synchronization ensures eventual consistency.

- **ActionBased Operations:** Exposes cloud store operations primarily through the Action system, allowing them to be
  part of larger workflows.

## SiteHostingService

## 1. **Component Name**

SiteHostingService

## 2. **Purpose**

Solves the need to host static websites or content directories directly from Daptin, using various cloud storage
backends.

Represents the multitenant static site hosting capability, mapping hostnames/paths to specific cloud storage locations.

## 3. **Key Responsibilities**

- **Define Sites:** Allows configuration of sites (`site` table), linking a hostname and optional subpath to a
  specific `cloudstore` and path within that store.

- **Serve Static Content:** Intercepts HTTP(S) requests matching configured site hostnames/paths and serves the
  corresponding files from the associated cloud store's local cache (`AssetFolderCache`).

- **HTTPS Handling:** Integrates with the CertificateManager to serve sites over HTTPS using managed SSL certificates.

- **Basic Authentication:** (Optional) Enforces Basic HTTP Authentication for accessing specific sites if configured.

- **Site Synchronization:** Manages the local cache synchronization for site content from the backing cloud store (
  similar
  to Asset Columns).

- **Site Creation Workflow:** Supports creating site structures (e.g., basic static site, Hugo site) on the cloud store
  via actions (`cloudstore.site.create`).

- **FTP Access:** (Optional, if FTP enabled globally) Provides FTP server access to the site's underlying storage path.

## 4. **Workflows / Use Cases**

Trigger: Incoming HTTP(S) request matching a configured site's hostname/path.

Steps: Identify the matching `site` configuration, locate the corresponding `AssetFolderCache`, determine the local path
for the requested file within the cache, serve the static file (potentially handling index.html for directories). Apply
Basic Auth if configured. Use appropriate SSL certificate for HTTPS.

Outcome: Static content served to the client's browser.

Trigger: User creates a `site` record via API or dashboard and potentially runs the `cloudstore.site.create` action.

Steps: Define hostname, path, link to `cloudstore`. Action creates initial directory structure on the cloud store.
System (on restart/sync) creates/updates local cache and routing rules.

Outcome: A new site is configured and becomes accessible.

Trigger: Files are updated directly on the backend cloud storage.

Steps: The periodic sync task (`tasksyncstorage.go`) or a manual trigger (`site.storage.sync` action) updates the
local `AssetFolderCache`.

Outcome: Updated content is reflected when the site is next accessed.

Trigger: User configures Basic Auth credentials (`basicauthuser`, `basicauthpasswordhash`) on the `site` record.

Steps: The HTTP request handler checks for these credentials; if present, it issues a 401 challenge if credentials are
missing/invalid, or allows access if valid.

Outcome: Site access restricted to authenticated users.

Trigger: User sets `ftpenabled` to true on the `site` record, and FTP is globally enabled (`config` setting).

Steps: The FTP server component (`server/ftpserver.go`) uses the site configuration to allow authenticated FTP users
access to the site's corresponding storage path.

Outcome: Site content manageable via FTP.

## 5. **Inputs and Outputs**

- **Inputs:** `site` table configuration, `cloudstore` configuration, HTTP(S) requests, FTP connections, SSL
  certificates,
  Basic Auth credentials.

- **Outputs:** Served static HTTP(S) content, FTP directory listings/file transfers, HTTP 401/403 responses.

## 6. **Dependencies**

- **Internal:** CloudStorageManager (for content backend and sync), CertificateManager (for HTTPS), ResourceManager (to
  read `site`, `cloudstore` tables), AuthEngine (for potential FTP/Basic Auth user validation), HTTP
  Router (`HostSwitch`), FTP Server (`server/ftpserver.go`).

- **External:** Cloud storage providers, DNS (for hostname resolution), local filesystem (for cache).

- **Database Models:** `site`, `cloudstore`, potentially `useraccount` (for Basic Auth/FTP).

## 7. **Business Rules & Constraints**

Each site must be linked to a valid `cloudstore`.

Hostnames must be unique or combined with unique paths to avoid routing conflicts.

Domains/subdomains used for hostnames must be configured in DNS to point to the Daptin instance.

For HTTPS, a valid certificate matching the hostname must exist and be managed by the CertificateService.

Basic Auth requires username and bcrypt hashed password stored in the `site` record.

FTP access requires global FTP enablement and the `ftpenabled` flag set on the site.

## 8. **Design Considerations**

- **Cloud Agnostic:** Leverages the CloudStorageManager to support various backends for site content.

- **Performance:** Uses local caching (`AssetFolderCache`) synchronized from the backend store to provide fast static
  file
  serving.

- **MultiTenancy:** Designed to host multiple distinct sites from a single Daptin instance.

- **Integrated Auth/SSL:** Seamlessly integrates with other Daptin features like authentication and certificate
  management.

- **Hugo Integration:** Specific support for creating and potentially building Hugo sites directly on the backend
  storage.

## CertificateService

## 1. **Component Name**

CertificateService

## 2. **Purpose**

Solves the need for managing SSL/TLS certificates to enable HTTPS for hosted sites and potentially other services (like
IMAPS/SMTPS).

Represents the lifecycle management of SSL certificates, including generation, storage, and retrieval.

## 3. **Key Responsibilities**

- **Store Certificates:** Persists certificate data (certificate PEM, private key PEM, root CA) in the `certificate`
  table. Private keys are encrypted before storage.

- **Generate SelfSigned Certificates:** Provides a mechanism (via `self.tls.generate` action) to generate selfsigned
  certificates for specified hostnames, useful for development or internal use.

- **Generate ACME Certificates (Let's Encrypt):** Provides a mechanism (via `acme.tls.generate` action) to obtain
  publicly
  trusted certificates from ACME providers like Let's Encrypt using the HTTP01 challenge.

- **Manage Private Keys:** Handles the generation, storage (encrypted), and decryption (using a
  systemwide `encryption.secret`) of private keys associated with certificates.

- **Provide TLS Configuration:** Constructs `tls.Config` objects on demand for the HTTP/HTTPS server, dynamically
  loading
  the correct certificate based on the requested hostname (Server Name Indication SNI).

## 4. **Workflows / Use Cases**

- **Generating a SelfSigned Certificate:**

Trigger: User executes the `self.tls.generate` action for a `certificate` record.

Steps: Generate a new RSA private key, create a selfsigned x509 certificate template, sign the certificate using the
private key, encrypt the private key, store the certificate PEM, encrypted private key PEM, and public key PEM in the
corresponding `certificate` record.

Outcome: A selfsigned certificate is generated and stored.

- **Obtaining an ACME (Let's Encrypt) Certificate:**

Trigger: User executes the `acme.tls.generate` action for a `certificate` record (hostname must resolve to the Daptin
instance).

Steps: Create an ACME user account key (or load existing one based on email), initialize ACME client (e.g., `lego`),
register the ACME account, initiate certificate order for the hostname, set up the HTTP01 challenge handler (responding
to `/.wellknown/acmechallenge/{token}`), complete the challenge, obtain the certificate bundle from the ACME provider,
encrypt the private key, store the certificate PEM, encrypted private key PEM, and issuer certificate PEM in
the `certificate` record.

Outcome: A publicly trusted certificate is obtained and stored.

- **Serving an HTTPS Request:**

Trigger: Incoming HTTPS connection.

Steps: The Go `tls` package (using the `GetCertificate` callback provided by Daptin) inspects the SNI from the
ClientHello, the CertificateManager looks up the corresponding hostname in the `certificate` table, decrypts the private
key, constructs a `tls.Certificate` object, and returns it to the TLS listener.

Outcome: The correct certificate is presented to the client for the requested hostname.

## 5. **Inputs and Outputs**

- **Inputs:** Hostnames, user email (for ACME), ACME challenge requests, `certificate` table data,
  system `encryption.secret`.

- **Outputs:** Generated certificate PEMs, private key PEMs (encrypted in DB, decrypted in memory), `tls.Config`
  objects,
  ACME HTTP01 challenge responses.

## 6. **Dependencies**

- **Internal:** ResourceManager (to read/write `certificate` table), ConfigurationManager (for `encryption.secret`),
  HTTP
  Router (for adding ACME challenge handler).

- **External:** Database, ACME Provider (e.g., Let's Encrypt), Go crypto
  libraries (`crypto/x509`, `crypto/rsa`, `crypto/tls`), `goacme/lego` library.

- **Database Models:** `certificate`.

## 7. **Business Rules & Constraints**

Private keys are always stored encrypted using the system `encryption.secret`.

ACME certificate generation requires the specified hostname to resolve publicly to the Daptin server handling the
request.

A valid email address is required for ACME registration.

Certificates are stored perhostname.

## 8. **Design Considerations**

- **Multiple Issuers:** Supports both selfsigned and ACME (Let's Encrypt) certificate generation.

- **Dynamic Loading:** Certificates are loaded dynamically based on SNI, allowing hosting of multiple HTTPS sites with
  different certificates on the same IP/port.

- **Security:** Encrypts private keys at rest. Relies on secure handling of the master `encryption.secret`.

- **Automation:** ACME integration automates the process of obtaining and potentially renewing trusted certificates.

- **ACME Challenge Handling:** Implements the HTTP01 challenge provider required by the `lego` library.

## IntegrationService

## 1. **Component Name**

IntegrationService

## 2. **Purpose**

Solves the need to interact with external, thirdparty APIs in a structured way within Daptin workflows.

Represents the capability to import API definitions (OpenAPI specs) and execute operations defined within those specs as
steps in Daptin Actions.

## 3. **Key Responsibilities**

- **Import API Specifications:** Allows importing OpenAPI v2 or v3 specifications (JSON or YAML) into the `integration`
  table.

- **Parse Specifications:** Parses the imported OpenAPI spec to identify available operations (paths, methods,
  parameters,
  request/response bodies, security schemes).

- **Store Integration Definitions:** Persists integration details, including the original spec, parsed operation
  details (
  implicitly), authentication type, and stored authentication credentials (encrypted) in the `integration` table.

- **Action Performer Registration:** Dynamically creates and registers an `ActionPerformerInterface` for each enabled
  integration, making its defined operations available for use within Daptin Actions.

- **Execute Integration Operations:** Handles the execution of a specific operation from an integration as an outcome
  step
  within an Action. This involves:

Identifying the correct operation based on the outcome's `Type` (integration name) and `Method` (operation ID).

Constructing the target URL using the server base URL from the spec and path templating.

Building the request payload (JSON body, form data, query parameters, header parameters) based on the operation's spec
and data provided in the action context.

Applying authentication (API Key, Basic Auth, Bearer Token, OAuth2) based on the integration's configuration and stored
credentials (or dynamically retrieved OAuth tokens).

Making the HTTP request to the external API.

Parsing the response and making it available to subsequent action outcomes.

## 4. **Workflows / Use Cases**

- **Importing a New Integration:**

Trigger: User creates a new `integration` record via API or dashboard, providing the OpenAPI spec content, format,
language, auth type, and credentials.

Steps: System parses the spec, validates it, encrypts credentials, stores the record. On next restart/reconfiguration, a
corresponding Action Performer is registered.

Outcome: The integration's operations become available for use in Actions.

- **Executing an Integration Operation within an Action:**

Trigger: An Action sequence reaches an outcome step with `Type` matching an integration name and `Method` matching an
operation ID.

Steps: The ActionExecutor invokes the corresponding integration's Action Performer. The performer retrieves integration
config and auth details, builds the HTTP request (URL, method, headers, body, query params) using data from the action
context and the OpenAPI spec, sends the request, processes the response.

Outcome: External API called, response data (body, headers, status code) added to the action context under the
outcome's `Reference` name.

- **Installing an Integration (Action):**

Trigger: User executes the `integration.install` action for an `integration` record.

Steps: (Potentially deprecated/alternative flow) This action might parse the integration spec and automatically create
corresponding Daptin `action` records for each operation ID, making them directly triggerable
via `/action/integration/{operationId}`. Note: The primary mechanism seems to be executing via
outcomes `Type: {integrationName}, Method: {operationId}`.

Outcome: Integration operations potentially mapped to directly callable Daptin actions.

## 5. **Inputs and Outputs**

- **Inputs:** OpenAPI v2/v3 specifications (JSON/YAML), `integration` table configuration, authentication credentials (
  API
  keys, user/pass, tokens), `oauthtoken` data (for OAuth2), data from Action context (for building requests).

- **Outputs:** HTTP requests to external APIs, responses from external APIs (passed back into the Action context),
  potentially new `action` records created (if using `integration.install`).

## 6. **Dependencies**

- **Internal:** ResourceManager (to read/write `integration` table, read `oauthtoken`, `credential`), ActionExecutor (to
  invoke the performer), ConfigurationManager (for encryption secret), OAuth handling logic (to refresh/use tokens).

- **External:** Database, any thirdparty API defined by an imported spec, `kinopenapi` library (for spec
  parsing/validation), `resty` or similar HTTP client library.

- **Database Models:** `integration`, `oauthtoken`, `credential`, potentially `action`.

## 7. **Business Rules & Constraints**

Imported specifications must be valid OpenAPI v2 or v3 format (JSON or YAML).

Authentication details must match the types defined in the spec (or the integration record's `authenticationtype`).

Credentials (`authenticationspecification`) are stored encrypted.

Operation IDs (`Method` in outcome) must match those defined in the OpenAPI spec.

Required parameters for an operation must be provided in the action context/attributes.

Integrations must be `enabled` for their performers to be active.

## 8. **Design Considerations**

- **OpenAPI Standard:** Leverages the widely adopted OpenAPI standard for defining external APIs, promoting
  interoperability.

- **Declarative Integration:** Allows integrating with external services by simply providing their spec, without writing
  custom Go code for each integration point.

- **Dynamic Action Performers:** Integrations dynamically extend the available operations within the Action system.

- **Unified Execution Flow:** Treats external API calls as just another type of outcome within the standard Action
  execution flow.

- **Secure Credential Storage:** Encrypts sensitive authentication details.

## EmailService

## 1. **Component Name**

EmailService (SMTP/IMAP Server & Client)

## 2. **Purpose**

Provides capabilities for Daptin to act as both an email server (receiving and storing emails via SMTP, serving them via
IMAP) and an email client (sending emails via SMTP as part of actions).

Represents the email handling subsystem.

## 3. **Key Responsibilities**

- **SMTP Server:** (If enabled) Listens for incoming SMTP connections, authenticates users against `mailaccount`
  credentials, accepts emails addressed to configured domains (`mailserver` hostname), stores received emails in the
  appropriate user's mailbox (`mail` and `mailbox` tables). Handles DKIM signing for outgoing mail if configured.

- **IMAP Server:** (If enabled) Listens for incoming IMAP(S) connections, authenticates users, provides access to stored
  emails and mailboxes according to the IMAP protocol standard.

- **Mail Storage:** Persists email content, metadata, flags (seen, recent, deleted), and mailbox structure in the
  database (`mail`, `mailbox` tables).

- **User Mail Accounts:** Manages email accounts (`mailaccount`) linked to main Daptin users (`useraccount`) and
  specific
  mail server configurations (`mailserver`). Stores hashed passwords.

- **Mail Server Configuration:** Allows defining multiple mail server configurations (`mailserver` table), each
  potentially handling different hostnames and listening interfaces, with TLS settings.

- **Send Mail Action (`mail.send`):** Provides an action outcome to send emails using Daptin's own SMTP capabilities (
  relaying if necessary) or potentially via external providers (see `aws.mail.send`). Handles DKIM signing based on the
  sender domain's certificate.

- **AWS SES Send Mail Action (`aws.mail.send`):** Provides a specific action outcome to send emails via Amazon SES using
  configured AWS credentials.

- **DKIM Signing:** Signs outgoing emails using the private key associated with the sender domain's
  certificate (`certificate` table) if available.

- **SPF/DKIM Verification (Incoming):** Performs basic SPF checks and DKIM verification on incoming emails to calculate
  a
  spam score.

## 4. **Workflows / Use Cases**

Trigger: External mail server connects to Daptin's SMTP port.

Steps: SMTP server accepts connection, potentially performs STARTTLS, authenticates sender (if required by config),
validates recipient domain/address against `mailaccount`, accepts email data, performs SPF/DKIM checks, calculates spam
score, stores email content and metadata in the `mail` table associated with the recipient's `mailbox` (usually INBOX or
Spam).

Outcome: Email stored in the database.

Trigger: Email client connects to Daptin's IMAP(S) port.

Steps: IMAP server accepts connection, handles TLS, authenticates user (`mailaccount`), allows listing
mailboxes (`LIST`), selecting a mailbox (`SELECT`), fetching messages (`FETCH` headers, body, flags), searching
messages (`SEARCH`), managing flags (`STORE`). Operations read data from `mail`/`mailbox` tables.

Outcome: Email client synchronizes and displays emails.

- **Sending an Email (via Action):**

Trigger: Action sequence includes a `mail.send` or `aws.mail.send` outcome.

Steps (`mail.send`): Construct email MIME body, look up certificate for sender domain, sign with DKIM, connect to
recipient's MX server (or configured relay), perform SMTP transaction to deliver the email.

Steps (`aws.mail.send`): Retrieve AWS credentials (`credential` table), construct SES API request, send email via AWS
SES API.

Outcome: Email delivered externally.

- **Configuring a Mail Server:**

Trigger: User creates/updates a `mailserver` record. System restart/sync (`mail.servers.sync` action) is triggered.

Steps: Daptin's main process reads all enabled `mailserver` configs, starts/stops/reconfigures SMTP/IMAP listeners based
on the stored settings and associated certificates.

Outcome: SMTP/IMAP services are active and configured for the specified domains/interfaces.

## 5. **Inputs and Outputs**

- **Inputs:** SMTP connections, IMAP
  connections, `mailserver`, `mailaccount`, `mailbox`, `mail`, `certificate`, `credential` table data, Action outcome
  parameters (`to`, `from`, `subject`, `body`, etc.).

- **Outputs:** Stored emails (`mail` table), SMTP responses, IMAP responses, outgoing emails sent to external servers,
  DKIM signatures.

## 6. **Dependencies**

- **Internal:** ResourceManager (to manage mailrelated tables), CertificateManager (for TLS and DKIM), AuthEngine (for
  SMTP/IMAP login), ConfigurationManager.

- **External:** Database, DNS (for MX lookups), external SMTP servers (for delivery), external email clients, AWS SES (
  if
  used), Go email
  libraries (`emersion/goimap`, `emersion/gosmtp`, `emersion/gomessage`, `emersion/gomsgauth/dkim`), `goguerrilla`
  library (modified).

- **Database Models:** `mailserver`, `mailaccount`, `mailbox`, `mail`, `certificate`, `credential`, `useraccount`.

## 7. **Business Rules & Constraints**

SMTP/IMAP services must be globally enabled via `config`.

`mailserver` records define domains and listening interfaces.

`mailaccount` records link Daptin users to email addresses and store credentials (hashed password).

Incoming emails are stored against the recipient `mailaccount` and `mailbox`.

DKIM signing requires a valid certificate with a private key for the sender's domain.

Authentication can be required for SMTP relaying.

Basic spam scoring is performed based on SPF/DKIM results.

## 8. **Design Considerations**

- **Integrated Mail Server:** Provides fullfledged SMTP/IMAP server capabilities directly within Daptin, eliminating the
  need for separate mail server software for basic use cases.

- **Database Backend:** Uses the main database for storing all email data and configuration, simplifying backups and
  management.

- **MultiDomain:** Can handle multiple domains via distinct `mailserver` configurations.

- **Security:** Supports TLS (STARTTLS/Implicit), SMTPS/IMAPS, user authentication, DKIM signing.

- **Library Reliance:** Builds upon standard Go email libraries and a modified `goguerrilla` for the core server logic.

## DataImportExportService

## 1. **Component Name**

DataImportExportService

## 2. **Purpose**

Solves the need to get data into and out of Daptin in bulk, using common file formats.

Represents the functionality for importing data from files (CSV, XLSX, JSON) and exporting data to files (JSON, CSV).

## 3. **Key Responsibilities**

- **File Parsing:** Parses uploaded files (CSV, XLSX, JSON) to extract structured data.

- **Schema Inference (Import):** (For CSV/XLSX) Analyzes data in uploaded files to automatically detect column types,
  nullability, and potential indexes/uniqueness.

- **Dynamic Table Creation/Alteration (Import):** (Optional) Creates new tables or adds missing columns to existing
  tables
  based on the structure of the imported file and user flags (`createifnotexists`, `addmissingcolumns`).

- **Data Insertion (Import):** Inserts parsed data rows into the target database table, potentially performing type
  conversions and handling errors. Supports bulk insertion.

- **Data Extraction (Export):** Retrieves all data for specified tables (or all tables) from the database.

- **File Formatting (Export):** Formats extracted data into the desired output format (JSON dump, CSV).

- **File Download Trigger:** Generates appropriate clientside responses (`client.file.download`) to trigger file
  downloads in the user's browser for exported data.

## 4. **Workflows / Use Cases**

- **Importing Data from CSV/XLSX:**

Trigger: User executes `uploadcsvfiletoentity` or `uploadxlsxfiletoentity` action with file data and target entity name.

Steps: Parse the file, infer schema (column types, etc.), compare with existing table schema (if any), create/alter
table if flags are set and necessary, iterate through rows and insert data into the database table using the
ResourceManager's create/update capabilities (potentially attempting updates on insert conflicts if unique keys are
involved). Trigger system restart if schema changed.

Outcome: Data imported into the target table, potentially with schema modifications.

- **Importing Data from JSON Dump:**

Trigger: User executes `dataimport` action with a JSON dump file.

Steps: Parse the JSON dump (expected format: `{"tableName": [ {rowMap}, ... ], ...}`), optionally truncate
tables (`truncatebeforeinsert` flag), iterate through tables and rows in the dump, insert data directly into
corresponding database tables.

Outcome: Data restored from the JSON dump.

Trigger: User executes `dataexport` action, optionally specifying a `tablename`.

Steps: Fetch all data for the specified table (or all tables) using ResourceManager, marshal the data into the JSON dump
format, encode the JSON as Base64, create a `client.file.download` action response with the Base64 data, filename, and
content type.

Outcome: Client receives a trigger to download the JSON data dump.

Trigger: User executes `csvdataexport` action, specifying a `tablename`.

Steps: Fetch all data for the specified table, determine headers from the first row (or schema), iterate through rows
formatting them as CSV, encode the CSV data as Base64, create a `client.file.download` action response.

Outcome: Client receives a trigger to download the CSV data file.

## 5. **Inputs and Outputs**

- **Inputs:** Uploaded files (CSV, XLSX, JSON), Action parameters (target entity name, flags
  like `truncatebeforeinsert`, `createifnotexists`).

- **Outputs:** Database records created/updated, clientside file download triggers (`client.file.download` action
  response containing Base64 encoded file content).

## 6. **Dependencies**

- **Internal:** ResourceManager (for database inserts/reads, schema info), ActionExecutor (as these are implemented as
  actions), File system (for temporary storage during upload/processing).

- **External:** Database, CSV parsing library (`encoding/csv`, `gocarina/gocsv`), XLSX parsing library (`tealeg/xlsx`),
  JSON parsing library (`jsoniter`).

- **Database Models:** Interacts with potentially any table for import/export.

## 7. **Business Rules & Constraints**

Import requires specific file formats (CSV, XLSX, JSON dump with specific structure).

Schema inference relies on analyzing a sample of the data (first ~100k distinct rows).

Automatic table creation/alteration is controlled by flags and requires system restart.

Data type conversions during import might fail if data doesn't match the target column type.

Export operations retrieve all data for the specified scope (single table or all tables).

## 8. **Design Considerations**

- **Ease of Use:** Provides simple actionbased mechanisms for common bulk data operations, including automatic schema
  generation for CSV/XLSX.

- **Format Support:** Supports major spreadsheet and data exchange formats.

- **Flexibility:** Flags allow control over whether to create/modify tables during import.

- **Schema Inference Heuristics:** Uses heuristics (sampling data, checking formats like dates, numbers, booleans) to
  guess column types, which might not always be perfect but works well for common cases.

- **ClientSide Download:** Exports trigger downloads via the browser rather than writing files on the server,
  simplifying
  deployment.

## WebSocketService

## 1. **Component Name**

WebSocketService

## 2. **Purpose**

Enables realtime, bidirectional communication between the Daptin server and connected clients.

Allows clients to subscribe to specific data events (e.g., creation, updates, deletion of records in a table) or custom
topics and receive updates instantly.

## 3. **Key Responsibilities**

- **Establish Connections:** Manages WebSocket connections initiated by clients on the `/live` endpoint. Authenticates
  connections using a JWT token provided as a query parameter.

- **Topic Management:** Maintains a registry of available topics. System topics are automatically created for each
  entity/table. Allows users to create and destroy custom topics via WebSocket messages.

- **Subscription Handling:** Processes client requests to subscribe (`subscribe` method) and unsubscribe (`unsubscribe`
  method) from specific topics. Maintains the association between connected clients and their subscriptions.

- **Event Broadcasting:** Receives internal events (e.g., from database middlewares after CRUD operations) and
  broadcasts
  relevant data payloads to all clients subscribed to the corresponding topic.

- **Filtering:** Allows clients to specify filters when subscribing (e.g., only receive `update` events for a specific
  table where `columnname == 'value'`). Filters messages before broadcasting to individual clients.

- **Custom Message Handling:** Allows clients to publish messages (`newmessage` method) to usercreated topics,
  broadcasting them to other subscribers of that topic.

- **Connection Management:** Tracks active client connections and handles disconnections, cleaning up subscriptions.

## 4. **Workflows / Use Cases**

- **Subscribing to Table Updates:**

Trigger: Client connects to `/live` and sends a `subscribe` message for topic `useraccount`.

Steps: WebSocket server registers the client's interest in the `useraccount` topic.

Outcome: Client is subscribed.

- **Receiving a Data Update Event:**

Trigger: A user updates a record in the `useraccount` table via the API. The `eventgenerator` middleware fires an
internal event.

Steps: The WebSocketService receives the internal event, identifies the topic (`useraccount`), finds all clients
subscribed to this topic, checks perclient filters, and sends the relevant data payload (e.g., the updated user account
object) to matching clients over their WebSocket connection.

Outcome: Subscribed clients receive the update in realtime.

- **Creating and Using a Custom Topic (e.g., Chat Room):**

Trigger (1): Client A sends `createtopic` message with `name: "chatroom123"`.

Outcome (1): Topic "chatroom123" is created.

Trigger (2): Client A and Client B send `subscribe` message for topic `chatroom123`.

Outcome (2): Both clients subscribed.

Trigger (3): Client A sends `newmessage` with `topic: "chatroom123"` and `message: {"text": "Hello!"}`.

Steps (3): WebSocketService receives the message, finds subscribers for "chatroom123" (Client A and B), broadcasts the
message payload `{"text": "Hello!"}` to both.

Outcome (3): Client A and Client B receive the chat message.

- **Subscribing with Filters:**

Trigger: Client sends `subscribe` message for topic `orders` with `filters: {"status": "shipped"}`.

Steps: Server registers subscription with the filter. When an `update` event for the `orders` topic occurs, the server
checks if the updated order's `status` is "shipped". If yes, the event is sent to this client; otherwise, it's skipped
for this client.

Outcome: Client only receives updates for orders with status "shipped".

## 5. **Inputs and Outputs**

- **Inputs:** WebSocket connections, JWT tokens (for auth), JSON messages from clients (listtopic, createtopic,
  destroytopic, subscribe, unsubscribe, newmessage), Internal events from other components (e.g., database middlewares).

- **Outputs:** JSON messages sent to clients over WebSocket (responses to requests, broadcasted events/messages).

## 6. **Dependencies**

- **Internal:** Auth Middleware (for initial connection auth), ResourceManager/Database Middlewares (as source of system
  events), potentially ActionExecutor (if actions publish messages). Olric DB (via `dtopicMap` for PubSub).

- **External:** WebSocket library (`gorilla/websocket`), Database (potentially for storing usercreated topic metadata,
  though might be inmemory or via Olric).

- **Database Models:** Implicitly related to all tables (for system event topics).

## 7. **Business Rules & Constraints**

WebSocket connection requires a valid JWT token passed as a query parameter.

System topics follow the naming convention of the entity/table name.

Usercreated topics require unique names.

Clients must subscribe to a topic to receive messages for it.

Filtering is applied serverside before broadcasting.

Permissions might apply to creating/destroying/subscribing/publishing to topics (details not fully specified in code
provided, but likely tied to user/group permissions).

## 8. **Design Considerations**

- **Realtime Updates:** Provides a mechanism for pushing data changes to clients without polling.

- **TopicBased Pub/Sub:** Uses a standard publishsubscribe model for decoupling event producers and consumers.

- **Scalability:** Uses Olric's PubSub for potentially distributed message broadcasting across multiple Daptin
  instances (
  needs confirmation on implementation details).

- **Filtering Efficiency:** Serverside filtering reduces unnecessary data transmission to clients.

- **Unified Endpoint:** Single `/live` endpoint handles all WebSocket communication.

## ConfigurationManager

## 1. **Component Name**

ConfigurationManager

## 2. **Purpose**

Manages systemwide configuration settings for Daptin.

Provides a centralized way to access and modify settings that control various features and behaviors (e.g., feature
flags, secrets, limits).

## 3. **Key Responsibilities**

- **Load Configuration:** Reads configuration settings from environment variables and the `config` database table on
  startup.

- **Store Configuration:** Persists configuration settings in the `config` table.

- **Provide Configuration Access:** Offers functions (`GetConfigValueFor`, `GetConfigIntValueFor`, etc.) for other
  components to retrieve specific configuration values.

- **Update Configuration:** Allows administrators (via the `/config/...` API endpoint) to modify configuration settings
  persisted in the database.

- **Environment Specificity:** Supports different configuration values based on the runtime environment (`ConfigEnv`
  column, e.g., 'release', 'debug', 'test').

- **Type Specificity:** Categorizes settings by type (`ConfigType` column, e.g., 'backend', 'web').

- **Caching:** (Potentially, via Olric) Caches configuration values in memory to reduce database lookups.

## 4. **Workflows / Use Cases**

Trigger: Daptin server starts.

Steps: ConfigurationManager loads default values, overrides with values from environment variables, then overrides with
values stored in the `config` table matching the current runtime environment. Secrets (JWT, Encryption, TOTP) are
initialized if not found.

Outcome: Configuration settings are loaded into memory (and potentially cached).

Trigger: Another component calls `configStore.GetConfigValueFor("jwt.secret", "backend", tx)`.

Steps: Check cache (e.g., Olric) first. If not found, query the `config` table for the key "jwt.secret", type "backend",
and the current environment. Return the stored value. Cache the result.

Outcome: The requested configuration value is returned.

- **Updating a Setting (via API):**

Trigger: Administrator sends `POST /config/backend/graphql.enable` with data `true`.

Steps: ConfigHandler receives the request, verifies admin privileges. Calls `configStore.SetConfigValueFor(...)`.
ConfigurationManager queries the `config` table for the existing value (to store in `previousvalue`), then inserts or
updates the row with the new value (`true`) for the key "graphql.enable", type "backend", and current environment.
Invalidates/updates the cache.

Outcome: Setting updated in the database. Note: A system restart (`restart` action) is usually required for the change
to take effect in the running application.

## 5. **Inputs and Outputs**

- **Inputs:** Environment variables, `config` database table, API requests to `/config/...`.

- **Outputs:** Configuration values provided to other components, rows updated/inserted in `config` table.

## 6. **Dependencies**

- **Internal:** Database connection. Potentially Olric DB for caching.

- **Database Models:** `config`.

## 7. **Business Rules & Constraints**

Configuration keys are identified by a combination of `name`, `configtype`, and `configenv`.

Only administrators can modify configuration via the API.

Certain critical settings (like secrets) are autogenerated on first run if not present.

Changes made via the API typically require a server restart to be fully applied by all components.

## 8. **Design Considerations**

- **Layered Configuration:** Uses a layered approach (defaults > environment > database) allowing flexibility in
  deployment.

- **Database Persistence:** Stores configuration in the database, making it persistent and manageable alongside
  application data.

- **API Management:** Provides an API endpoint for administrative configuration changes.

- **Caching:** (Assumed via Olric mention) Caching improves performance by reducing database queries for frequently
  accessed settings.

- **Requires Restart:** The current design often necessitates a restart for configuration changes to propagate, which
  could be improved with a dynamic reloading mechanism for certain settings. \`\`\`