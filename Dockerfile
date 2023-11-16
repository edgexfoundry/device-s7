#
# Copyright (C) 2023 YIQISOFT
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

ARG BASE=golang:1.21-alpine3.18
FROM ${BASE} AS builder

ARG MAKE=make build

WORKDIR /device-s7

LABEL license='SPDX-License-Identifier: Apache-2.0' \
  copyright='Copyright (c) 2023: YIQISOFT'

RUN apk add --update --no-cache make git gcc libc-dev

COPY go.mod vendor* ./
RUN [ ! -d "vendor" ] && go mod download all || echo "skipping..."

COPY . .
RUN ${MAKE}

# Next image - Copy built Go binary into new workspace
FROM alpine:3.18
LABEL license='SPDX-License-Identifier: Apache-2.0' \
  copyright='Copyright (c) 2023: YIQISOFT'

RUN apk add --update --no-cache dumb-init
# Ensure using latest versions of all installed packages to avoid any recent CVEs
RUN apk --no-cache upgrade

WORKDIR /
COPY --from=builder /device-s7/cmd/Attribution.txt /Attribution.txt
COPY --from=builder /device-s7/cmd/device-s7 /device-s7
COPY --from=builder /device-s7/cmd/res /res

EXPOSE 59994

ENTRYPOINT ["/device-s7"]
CMD ["-cp=consul.http://edgex-core-consul:8500", "--registry"]
