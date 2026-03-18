# Label Station

A Go-based web server and React frontend for printing labels using Brother QL series printers through the `brother_ql` package.

## Features

-   **Backend**: Specified model, backend type, and connection string at startup.
-   **API Endpoint**: `POST /api/v1/print` for all print designs.
-   **Frontend**: Multi-mode labels supporting QR Code (WiFi, Contact, URL), Address format, or arbitrary images properly aligned.

## Setup & Running

To run the full service, you only need the compiled binary and the frontend bundle assets.

### 1. Build

Ensure the Go binary was compiled.

```bash
go build -o label-station main.go
```

The frontend files in `fe/dist` are already built and will be served automatically.

### 2. Run the Server

Start the application by passing your printer model and connection string in the corresponding flags.

**Example: Network Printer (Wi-Fi / Ethernet)**

```bash
./label-station --model QL-800 --backend network --printer 192.168.1.100
```

**Example: Local USB Printer (Linux setup)**

```bash
./label-station --model QL-800 --backend linux_kernel --printer /dev/usb/lp0
```

### 3. Run with Docker Compose

Alternatively, you can run the entire service (backend and frontend) inside a Docker container using Docker Compose.

1.  **Run with Docker Compose**:
    ```bash
    docker compose up --build
    ```

2.  **Custom Configuration**:
    You can customize your printer model, backend, and address by setting environment variables in your shell or creating a `.env` file in the root directory:

    ```bash
    # Example for a network printer
    export BROTHER_QL_MODEL=QL-800
    export BROTHER_QL_BACKEND=network
    export BROTHER_QL_PRINTER=192.168.1.100

    docker compose up --build
    ```

    *Note for Docker on local setups*: Inside the container, `127.0.0.1` will NOT refer to your host machine. Use `host.docker.internal` instead to reach services running on the host machine.

### 4. Usage

Access the tool by navigating to **http://localhost:8080** on your browser.

-   **QR Mode**: Create WiFi profiles to generate scanned prints instantly.
-   **Address Mode**: Compose layout frames before queueing to standard sizes to review placement beforehand structure alignment.
-   **Static Uploads**: Adapt pre-cropped templates natively directly synchronized over streaming frames correctly buffered.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
