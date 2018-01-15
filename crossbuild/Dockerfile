FROM multiarch/crossbuild
RUN curl -O https://dl.google.com/go/go1.8.5.linux-amd64.tar.gz
RUN tar xf go1.8.5.linux-amd64.tar.gz
RUN mv go /usr/local
ENV GOPATH=/go
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
RUN /usr/local/go/bin/go version
RUN /usr/local/go/bin/go env
RUN apt-get update && apt-get install -y libc6-dev-i386 gcc-mingw-w64