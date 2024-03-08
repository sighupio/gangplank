# Copyright 2017-present SIGHUP s.r.l
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.22.1-alpine as dev

ENV CGO_ENABLED=0

WORKDIR /tmp/gangway

COPY . .

RUN go mod download

ENTRYPOINT [ "go", "run", "./cmd/gangway" ]

FROM dev as builder

RUN go build ./cmd/gangway

FROM scratch

COPY --from=builder /tmp/gangway/gangway /urs/local/bin/gangway

ENTRYPOINT ["gangway"]
