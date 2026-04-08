# maintenance-page

`maintenance-page` is a Go application designed to manage and serve maintenance pages for the CyVerse Discovery Environment. It provides a static maintenance page for end-users and a basic-auth protected administrative interface to toggle the Discovery Environment's `HTTPRoute` between the live service and the maintenance page.

## Features

- **Maintenance Page**: Serves a static, visually consistent maintenance page.
- **Admin Interface**: A simple toggle to switch traffic between the Discovery Environment UI and the maintenance page.
- **Kubernetes Integration**: Automatically ensures its own Service definitions are present in the cluster and updates `HTTPRoute` backend refs.
- **Security**: Admin interface is protected via Basic Authentication.

## Configuration

The application is configured using command-line flags or equivalent environment variables.

| Flag | Environment Variable | Default | Description |
|------|----------------------|---------|-------------|
| `--maintenance-page-service` | `MAINTENANCE_PAGE_SERVICE` | `maintenance-page` | Name of the K8s Service for the maintenance page. |
| `--admin-page-service` | `ADMIN_PAGE_SERVICE` | `maintenance-page-admin` | Name of the K8s Service for the admin interface. |
| `--basic-auth-username` | `BASIC_AUTH_USERNAME` | | **Required.** Username for the admin interface. |
| `--basic-auth-password` | `BASIC_AUTH_PASSWORD` | | **Required.** Password for the admin interface. |
| `--port` | | `8080` | Port for the public maintenance page. |
| `--admin-port` | | `8081` | Port for the admin interface. |
| `--kubeconfig` | `KUBECONFIG` | | Path to kubeconfig (optional, defaults to in-cluster). |
| `--namespace` | `NAMESPACE` | `prod` | Kubernetes namespace to operate in. |
| `--sonora-route-name` | `SONORA_ROUTE_NAME` | `discoenv-routes` | Name of the `HTTPRoute` to toggle. |
| `--sonora-service` | `SONORA_SERVICE` | `sonora` | Name of the live DE UI service. |
| `--sonora-port` | `SONORA_PORT` | `80` | Port of the live DE UI service. |

## Development

The project uses `just` for common development tasks.

### Build

```bash
just build
```

### Run Tests

```bash
just test
```

### Build Docker Image

```bash
just build-image
```

## Kubernetes Deployment

Example deployment configurations can be found in the `k8s/` directory. The application requires appropriate RBAC permissions to manage Services and `HTTPRoutes` within its namespace.
