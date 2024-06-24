ARG expose_via=local

FROM node:lts-alpine AS FRONTEND
WORKDIR /frontend-build

COPY web/package.json web/yarn.lock ./
RUN yarn install

COPY web ./
RUN yarn build

FROM golang:latest AS BASE

ARG arch=amd64

COPY go.* ./
RUN go mod download
COPY . .
COPY --from=FRONTEND /frontend-build/dist web/dist
RUN go build -v .
RUN go install

RUN (curl -Ls --tlsv1.2 --proto "=https" --retry 3 https://cli.doppler.com/install.sh || wget -t 3 -qO- https://cli.doppler.com/install.sh) | sh

RUN touch run.sh
RUN echo "#!/bin/bash" >> run

FROM base AS expose-cloudflare
RUN curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${arch}.deb
RUN dpkg -i cloudflared.deb
RUN echo "doppler run --command \"cloudflared tunnel run & eth-faucet --faucet.amount=1 --faucet.tokenamount=20 --faucet.minutes=1440\"" >> run

FROM base AS expose-local
EXPOSE 8082
RUN echo "doppler run -- eth-faucet --faucet.amount=1 --faucet.tokenamount=20 --faucet.minutes=1" >> run

FROM expose-$expose_via AS FINAL
RUN chmod +x run

CMD ["/bin/bash", "./run"]
