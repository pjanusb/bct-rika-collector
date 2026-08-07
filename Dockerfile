FROM fedora:41

ARG GO_VERSION=1.26.5
ARG GO_SHA256=5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053

RUN printf '%s\n' \
    '[fedora-archive]' \
    'name=Fedora 41 archive - $basearch' \
    'baseurl=https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/41/Everything/$basearch/os/' \
    'enabled=1' \
    'gpgcheck=1' \
    'gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-fedora-41-$basearch' \
    '[updates-archive]' \
    'name=Fedora 41 updates archive - $basearch' \
    'baseurl=https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/41/Everything/$basearch/' \
    'enabled=1' \
    'gpgcheck=1' \
    'gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-fedora-41-$basearch' \
    > /etc/yum.repos.d/fedora-archive.repo \
    && dnf -y --disablerepo='*' --enablerepo=fedora-archive --enablerepo=updates-archive install ca-certificates curl git make tar gzip \
    && dnf clean all
RUN curl -fsSLo /tmp/go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm -f /tmp/go.tar.gz

ENV PATH=/usr/local/go/bin:$PATH
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
ENV GOPATH=/go
ENV GOMODCACHE=/go/pkg/mod
ENV GOCACHE=/tmp/go-build

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download && chmod -R a+rwX /go
COPY . .

CMD ["make", "build"]
