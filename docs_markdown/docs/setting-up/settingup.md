# Setting up Daptin

Daptin is built in golang and a static artifact is available for most targets

## Deploy and get started

| Deployment preference      | Getting started                                                                                                               |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Heroku                     | [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/daptin/daptin) |
| Docker                     | docker run -p 8080:8080 daptin/daptin                                                                                         |
| Kubernetes                 | [Service & Deployment YAML](#kubernetes)                                                                                      |
| Development                | go get github.com/daptin/daptin                                                                                               |
| Linux (386/amd64/arm5,6,7) | [Download static linux builds](https://github.com/daptin/daptin/releases)                                                     |
| Windows                    | go get github.com/daptin/daptin                                                                                               |
| OS X                       | go get github.com/daptin/daptin                                                                                               |
| Load testing               | [Docker compose](#docker-compose)                                                                                             |
| Raspberry Pi               | [Linux arm 7 static build](https://github.com/daptin/daptin/releases)                                                         |



# Port

Daptin will listen on port 6336 by default. You can change it by using the following argument

```-port=8080```

# Restart

Daptin relies on self ```re-configuration``` to configure new entities and APIs and changes to the other parts of the ststem. As soon as you upload a schema file, daptin will write the file to disk, and ```reconfigure``` itself. When it starts it will read the schema file, make appropriate changes to the database and expose JSON apis for the entities and actions.

You can issue a daptin restart from the dashboard. Daptin takes about 15 seconds approx to start up and configure everything.


# Detailed instructions


