# ColormaticStudios/ecommerce: Frontend App

The frontend for the self-hostable ecommerce platform, built with Svelte, TypeScript, and Tailwind CSS. This project provides a modern, responsive interface for interacting with the backend API.

## Overview

This frontend application enables users to:

- Browse and search products
- Authenticate and manage user profiles
- Manage their cart
- Checkout
- View order history

## Getting Started

You can use any Javascript package manager, e.g. NPM, Bun, Yarn, PNPM, etc. For this example, we'll use Bun.

### Setup

1. **Clone the repository**

   ```bash
   git clone https://git.colormatic.org/ColormaticStudios/ecommerce.git
   cd ecommerce/frontend
   ```

2. **Install dependencies**
   ```bash
   bun install
   ```

3. **Start the development server with hot reload**

   ```bash
   bun run dev --open
   ```

The app will be available at http://localhost:3000 by default.

### Building for Production

Build the frontend for production:

```bash
bun run build
```

This will create a production-ready build in the `build/` directory.

## Development Workflow

### Available Scripts

- `bun run dev`: Start development server with hot reload
- `bun run build`: Build production-ready static assets
- `bun run format`: Format code with Prettier
- `bun run lint`: Lint code with ESLint
- `bun run check`: Run type checks with TypeScript