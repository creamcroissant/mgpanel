# XBoard User Frontend (Vite + React)

User/Admin SPA for XBoard. Built assets are embedded into the backend binary and can also be shipped via release tarballs.

## Stack

- React 19 + TypeScript + Vite
- Tailwind CSS + Radix UI primitives
- React Router, TanStack Query, i18next
- Playwright (E2E)

## Local development

```bash
cd web/user-vite
npm install
npm run dev
```

- Dev server: `http://127.0.0.1:4173`
- API proxy: `/api` -> `VITE_API_TARGET` (default `http://localhost:8080`)

## Build

```bash
npm run build
```

- Output directory: `web/user-vite/dist`
- In backend build, assets are copied into `./web/user-vite/dist` and embedded into the Go binary.

## Routes

- User auth: `/login`, `/register`, `/forgot-password`
- User app: `/dashboard`, `/servers`, `/plans`, `/traffic`, `/knowledge`, `/settings`
- Admin auth: `/{secure_path}/login` (default `/admin/login`)
- Admin app: `/{secure_path}/agents`, `/{secure_path}/users`, `/{secure_path}/plans`, `/{secure_path}/notices`, `/{secure_path}/knowledge`, `/{secure_path}/forwarding`, `/{secure_path}/access-logs`, `/{secure_path}/config-center`, `/{secure_path}/system`

`secure_path` and `router_base` are read from runtime settings (`window.settings`).

## Test

```bash
npm run lint
npm run test
```

Run a single spec:

```bash
npm run test -- admin-config-center.spec.ts
```

## Deployment notes

- `deploy/panel.sh` installs frontend release asset `frontend-dist.tar.gz` to `${INSTALL_DIR}/web/user-vite/dist`.
- Install UI asset `install-ui.tar.gz` is installed to `${INSTALL_DIR}/web/install`.
- Both assets are checksum-verified against release `SHA256SUMS.txt` before installation.
