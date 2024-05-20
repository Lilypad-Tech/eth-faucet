ARG expose_via=local

FROM node:lts-alpine AS FRONTEND
WORKDIR /frontend-build

COPY web/package.json web/yarn.lock ./
RUN yarn install

COPY web ./
RUN yarn build

FROM golang:latest AS BASE

ARG arch=amd64
ARG doppler_config=dev
ARG cloudflare_token="not-a-token"

COPY go.* ./
RUN go mod download
COPY . .
COPY --from=FRONTEND /frontend-build/dist web/dist
RUN go build -v .
RUN go install

RUN (curl -Ls --tlsv1.2 --proto "=https" --retry 3 https://cli.doppler.com/install.sh || wget -t 3 -qO- https://cli.doppler.com/install.sh) | sh

RUN touch run.sh
RUN echo "#!/bin/bash" >> run.sh

FROM base AS expose-cloudflare
RUN curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${arch}.deb
RUN dpkg -i cloudflared.deb
RUN echo "cloudflared tunnel run --token $cloudflare_token --url http://localhost:8080 &" >> run.sh

FROM base AS expose-local
EXPOSE 8082

FROM expose-$expose_via AS FINAL
RUN echo "doppler run -- eth-faucet --faucet.amount=5 --faucet.tokenamount=20 --faucet.minutes=1" >> run.sh
RUN chmod +x run.sh

CMD ["/bin/bash", "./run.sh"]
