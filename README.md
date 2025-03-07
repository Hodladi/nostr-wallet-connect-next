# Nostr Wallet Connect (Next)

This is a self-sovereign, self-custodial, single-user rewrite of NWC currently in an experimental phase development. **❗This version is not backwards compatible with NWC - it requires a fresh database and connections to be re-added**

This application allows you to control your Lightning node or wallet from any other application that supports [NWC](https://nwc.dev/).
Connect apps like [Damus](https://damus.io/) or [Amethyst](https://linktr.ee/amethyst.social) to your node. There are many more available on https://nwc.dev/.

**Specification**: [NIP-47](https://github.com/nostr-protocol/nips/blob/master/47.md)

The application can run in two modes:

- Wails (Desktop app): Mac (arm64), Windows (amd64), Linux (amd64)
- HTTP (Web app): Docker, Linux (amd64)

Ideally the app runs 24/7 (on a node, VPS or always-online desktop/laptop machine) so it can be connected to a lightning address and receive online payments.

## Supported Backends

- LND (see: lnd.go)
- Breez (see: breez.go)
- Greenlight (see: greenlight.go)
- LDK (see: ldk.go)
- want more? please open an issue.

## Installation

### Requirements

The application has no runtime dependencies. (simple Go executable).

As data storage SQLite is used.

    $ cp .env.example .env
    # edit the config for your needs
    vim .env

#### Optional Requirements

See [Greenlight](./README_GREENLIGHT.md)

## Development

### Required Software

- Go
- Node
- NPM
- Yarn

### Server (HTTP mode)

1. Create a Lightning Polar setup with two LND nodes and uncomment the Polar LND section in your `.env` file.

2. Compile the frontend or run `touch frontend/dist/tmp` to ensure there are embeddable files available.

3. `go run .`

### React Frontend (HTTP mode)

Go to `/frontend`

1. `yarn install`
2. `yarn dev`

### Wails (Backend + Frontend)

_Make sure to have [wails](https://wails.io/docs/gettingstarted/installation) installed and all platform-specific dependencies installed (see wails doctor)_

    $ wails dev -tags "wails"

_If you get a blank screen, try running in your normal terminal (outside of vscode, and make sure HTTP frontend is not running)_

#### Wails Production build

    $ wails build -tags "wails"

### Build and run locally (HTTP mode)

    $ mkdir tmp
    $ go build -o main
    $ cp main tmp
    $ cp .env tmp
    $ cd tmp
    $ ./main

### Run dockerfile locally (HTTP mode)

    $ docker build . -t nwc-local --progress=plain
    $ docker run --env-file .env -p 8080:8080 nwc-local

### Testing

    $ go test

### Profiling

The application supports both the Go pprof library and the DataDog profiler.

#### Go pprof

To enable Go pprof, set the `GO_PROFILER_ADDR` environment variable to the address you want the profiler to be available on (e.g. `localhost:6060`).

Now, you should be able to access the pprof web interface at `http://localhost:6060/debug/pprof`.

You can use the `go tool pprof` command to collect and inspect the profiling data. For example, to profile the application for 30 seconds and then open the pprof web UI, run:

```sh
go tool pprof -http=localhost:8081 -seconds=30 http://localhost:6060/debug/pprof/profile
```

For more information on the Go pprof library, see the [official documentation](https://pkg.go.dev/net/http/pprof).

#### DataDog profiler

To enable the DataDog profiler, set the `DD_PROFILER_ENABLED` environment variable to `true`.

Make sure to specify the required DataDog configuration environment variables as well.

For more information refer to:

- [DataDog Profiler documentation](https://docs.datadoghq.com/profiler/enabling/go/)
- [DataDog Profiler Go library](https://pkg.go.dev/gopkg.in/DataDog/dd-trace-go.v1/profiler)

### Windows

Breez SDK requires gcc to build the Breez bindings. Run `choco install mingw` and copy the breez SDK bindings file into the root of this directory (from your go installation directory) as per the [Breez SDK instructions](https://github.com/breez/breez-sdk-go?tab=readme-ov-file#windows). ALSO copy the bindings file into the output directory alongside the .exe in order to run it.

## Optional configuration parameters

- `NOSTR_PRIVKEY`: the private key of this service. Should be a securely randomly generated 32 byte hex string.
- `CLIENT_NOSTR_PUBKEY`: if set, this service will only listen to events authored by this public key. You can set this to your own nostr public key.
- `RELAY`: default: "wss://relay.getalby.com/v1"
- `COOKIE_SECRET`: a randomly generated secret string. (only needed in http mode)
- `DATABASE_URI`: a sqlite filename. Default: $XDG_DATA_HOME/nostr-wallet-connect/nwc.db
- `PORT`: the port on which the app should listen on (default: 8080)
- `WORK_DIR`: directory to store NWC data files. Default: $XDG_DATA_HOME/nostr-wallet-connect
- `LOG_LEVEL`: log level for the application. Higher is more verbose. Default: 4 (info)

### LND Backend parameters

Currently only LND can be configured via env. Other node types must be configured via the UI.

_To configure via env, the following parameters must be provided:_

- `LN_BACKEND_TYPE`: LND
- `LND_ADDRESS`: the LND gRPC address, eg. `localhost:10009` (used with the LND backend)
- `LND_CERT_FILE`: the location where LND's `tls.cert` file can be found (used with the LND backend)
- `LND_MACAROON_FILE`: the location where LND's `admin.macaroon` file can be found (used with the LND backend)

### LDK Backend parameters

- `LDK_ESPLORA_SERVER`: If using the mainnet (bitcoin) network, Recommended to use your own LDK esplora server (The public blockstream one is very slow and can cause onchain syncing and issues with opening channels)

#### Testnet

- `MEMPOOL_API=https://mempool.space/testnet/api`
- `LDK_NETWORK=testnet`
- `LDK_ESPLORA_SERVER=https://mempool.space/testnet/api`
- `LDK_GOSSIP_SOURCE=https://rapidsync.lightningdevkit.org/testnet/snapshot`

#### Mutinynet

- `MEMPOOL_API=https://mutinynet.com/api`
- `LDK_NETWORK=signet`
- `LDK_ESPLORA_SERVER=https://mutinynet.com/api`
- `LDK_GOSSIP_SOURCE=https://rgs.mutinynet.com/snapshot`

### Alby OAuth

Create an OAuth client at the [Alby Developer Portal](https://getalby.com/developer) and set your `ALBY_OAUTH_CLIENT_ID` and `ALBY_OAUTH_CLIENT_SECRET` in your .env. If not running locally, you'll also need to change your `BASE_URL`.

> If running the React app locally, OAuth redirects will not work locally if running the react app you will need to manually change the port to 5173. **Login in Wails mode is not yet supported**

## Getting Started with Mutinynet

Follow the steps to integrate Mutinynet with your NWC Next setup:

1. Configure your environment with the [Mutinynet LDK parameters](https://github.com/getAlby/nostr-wallet-connect-next#mutinynet)

2. Proceed as described in the [Development](https://github.com/getAlby/nostr-wallet-connect-next#Development) section to run the frontend and backend

3. During onboarding, after setting your password and authorizing via Alby OAuth, you'll be directed to `/onboarding/lightning/migrate-alby`. Click "Skip For Now" to access your wallet interface

4. Navigate to `channels/onchain/deposit-bitcoin`, copy your On-Chain Address, then visit the [Mutinynet Faucet](https://faucet.mutinynet.com/) to deposit sats. Ensure the transaction confirms on [mempool.space](https://mutinynet.com/)

5. Your On-chain balance will update under `/channels`

### Opening a channel from Mutinynet

1. To create a channel, use the [Mutinynet Faucet](https://faucet.mutinynet.com/) by entering your desired Channel Capacity and Amount to Push

2. Locate your Node ID. In the Wallet click on the status on the top right "online". This shows the node ID or look in the NWC Next logs. Then input this in the Connection String field on the faucet page to request a Lightning Channel

```
{"level":"info","msg":"Connected to LDK node","nodeId":"<your node ID>","time":"<timestamp>"}
```

3. After the transaction confirms, the new channel will appear in the Channels section

### Opening a Channel in NWC Next

1. From the Channels interface (`/channels`), select "Open a Channel" and opt for "Custom Channel."

2. Enter the pubkey of the Faucet Lightning Node (omit host and port details) available on the [Mutinynet Faucet](https://faucet.mutinynet.com/) page.

3. Specify a channel capacity greater than 25,000 sats, confirm the action, and return to the Channels page to view your newly established channel.

## Application deeplink options

### `/apps/new` deeplink options

Clients can use a deeplink to allow the user to add a new connection. Depending on the client this URL has different query options:

#### NWC created secret

The default option is that the NWC app creates a secret and the user uses the nostr wallet connect URL string to enable the client application.

##### Query parameter options

- `name`: the name of the client app

Example:

`/apps/new?name=myapp`

#### Client created secret

If the client creates the secret the client only needs to share the public key of that secret for authorization. The user authorized that pubkey and no sensitivate data needs to be shared.

##### Query parameter options for /new

- `name`: the name of the client app
- `pubkey`: the public key of the client's secret for the user to authorize
- `return_to`: (optional) if a `return_to` URL is provided the user will be redirected to that URL after authorization. The `lud16`, `relay` and `pubkey` query parameters will be added to the URL.
- `expires_at` (optional) connection cannot be used after this date. Unix timestamp in seconds.
- `max_amount` (optional) maximum amount in sats that can be sent per renewal period
- `budget_renewal` (optional) reset the budget at the end of the given budget renewal. Can be `never` (default), `daily`, `weekly`, `monthly`, `yearly`
- `request_methods` (optional) url encoded, space separated list of request types that you need permission for: `pay_invoice` (default), `get_balance` (see NIP47). For example: `..&request_methods=pay_invoice%20get_balance`

Example:

`/apps/new?name=myapp&pubkey=47c5a21...&return_to=https://example.com`

#### Web-flow: client created secret

Web clients can open a new prompt popup to load the authorization page.
Once the user has authorized the app connection a `nwc:success` message is sent to the opening page (using `postMessage`) to indicate that the connection is authorized. See the `initNWC()` function in the [alby-js-sdk](https://github.com/getAlby/alby-js-sdk#nostr-wallet-connect-documentation)

Example:

```js
import { webln } from "alby-js-sdk";
const nwc = new webln.NWC();
// initNWC opens a prompt with /apps/new?c=myapp&pubkey=xxxx
// the promise resolves once the user has authorized the connection (when the `nwc:success` message is received) and the popup is closed automatically
// the promise rejects if the user cancels by closing the prompt popup
await nwc.initNWC({ name: "myapp" });
```

## Help

If you need help contact support@getalby.com or reach out on Nostr: npub1getal6ykt05fsz5nqu4uld09nfj3y3qxmv8crys4aeut53unfvlqr80nfm
You can also visit the chat of our Community on [Telegram](https://t.me/getalby).

## ⚡️Donations

Want to support the work on Alby?

Support the Alby team ⚡️hello@getalby.com
You can also contribute to our [bounty program](https://github.com/getAlby/lightning-browser-extension/wiki/Bounties): ⚡️bounties@getalby.com

## NIP-47 Supported Methods

✅ NIP-47 info event

❌ `expiration` tag in requests

### LND

✅ `get_info`

✅ `get_balance`

✅ `pay_invoice`

- ⚠️ amount not supported (for amountless invoices)
- ⚠️ PAYMENT_FAILED error code not supported

✅ `pay_keysend`

- ⚠️ PAYMENT_FAILED error code not supported

✅ `make_invoice`

✅ `lookup_invoice`

- ⚠️ NOT_FOUND error code not supported

✅ `list_transactions`

- ⚠️ from and until in request not supported
- ⚠️ failed payments will not be returned

✅ `multi_pay_invoice`

- ⚠️ amount not supported (for amountless invoices)
- ⚠️ PAYMENT_FAILED error code not supported

✅ `multi_pay_keysend`

- ⚠️ PAYMENT_FAILED error code not supported

### Breez

(Supported methods coming soon)

## Node Distributions

Run NWC on your own node!

**NOTE: the below links are for the original version of NWC**

- [https://github.com/getAlby/umbrel-community-app-store](Umbrel)
- [https://github.com/horologger/nostr-wallet-connect-startos](Start9)

## Deploy it yourself

### Digital Ocean

[![Deploy to DO](https://www.deploytodo.com/do-btn-blue.svg)](https://cloud.digitalocean.com/apps/new?repo=https://github.com/getAlby/nostr-wallet-connect-next/tree/master)

### Render

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/getAlby/nostr-wallet-connect-next)

### Fly

- [install fly](https://fly.io/docs/hands-on/install-flyctl/)
- update `app = 'nwc'` on **line 6** to a unique name in fly.toml e.g. `app = 'nwc-john-doe-1234'`
- run `fly launch`
  - press 'y' to copy configuration to the new app and then hit enter
  - press 'n' to tweak the settings and then hit enter
  - wait for the deployment to succeed, it should give you a URL like `https://nwc-john-doe-1234.fly.dev`

#### Update Fly App

- run `fly deploy`

#### View logs

Main application logs

- `fly logs`

LDK logs:

- `fly machine exec "tail -100 data/ldk/logs/ldk_node_latest.log"`

### Custom Ubuntu VM

- install go (using snap)
- install build-essential
- install nvm (curl script)
- with nvm, choose node lts
- install yarn (via npm)
- then run yarn build
- go run .

### Docker

#### From Alby's Container Registry

`docker run -p 8080:8080 ghcr.io/getalby/nostr-wallet-connect-next:latest`

#### From Source

`docker run -p 8080:8080 $(docker build -q .)`
