#
# Copyright (c) 2020 Intel
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

ARG BASE=golang:1.18-alpine
FROM ${BASE} AS builder

ARG TARGETOS
ARG TARGETARCH

RUN echo "I'm building for $TARGETOS/$TARGETARCH"

WORKDIR /device-gpiod

LABEL license='SPDX-License-Identifier: Apache-2.0' \
  copyright='Copyright (c) 2020: Intel'

RUN sed -e 's/dl-cdn[.]alpinelinux.org/dl-4.alpinelinux.org/g' -i~ /etc/apk/repositories

RUN apk update && apk add --update --no-cache make git gcc libc-dev zeromq-dev libsodium-dev openssh-client

COPY . .

RUN go mod download

RUN make build

# Next image - Copy built Go binary into new workspace
FROM alpine
LABEL license='SPDX-License-Identifier: Apache-2.0' \
  copyright='Copyright (c) 2020: Intel'

RUN sed -e 's/dl-cdn[.]alpinelinux.org/nl.alpinelinux.org/g' -i~ /etc/apk/repositories

RUN apk add --update --no-cache zeromq

WORKDIR /
COPY --from=builder /device-gpiod/cmd/device-gpiod/res /res
COPY --from=builder /device-gpiod/cmd/device-gpiod/device-gpiod /device-gpiod

EXPOSE 60000

ENTRYPOINT ["/device-gpiod"]
CMD ["--confdir=/res"]
